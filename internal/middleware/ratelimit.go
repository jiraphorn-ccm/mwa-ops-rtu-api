package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/rtu-api/internal/httpx"
)

// rateLimitBody is intentionally outside the standard envelope, matching the
// express-rate-limit responses of the other MWA services.
type rateLimitBody struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter throttles each client IP to `requests` per `window` using a token
// bucket, and evicts idle buckets in the background.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    rate.Limit
	burst    int
	requests int
	window   time.Duration
	stop     chan struct{}
}

// NewRateLimiter starts a limiter along with its janitor goroutine.
func NewRateLimiter(requests int, window time.Duration) *RateLimiter {
	if requests < 1 {
		requests = 1
	}
	if window <= 0 {
		window = time.Minute
	}

	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    rate.Limit(float64(requests) / window.Seconds()),
		burst:    requests,
		requests: requests,
		window:   window,
		stop:     make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// Close stops the janitor goroutine.
func (rl *RateLimiter) Close() {
	close(rl.stop)
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-3 * rl.window)
			rl.mu.Lock()
			for key, v := range rl.visitors {
				if v.lastSeen.Before(cutoff) {
					delete(rl.visitors, key)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func (rl *RateLimiter) limiterFor(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, ok := rl.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.visitors[key] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// Handler is the http middleware form of the limiter.
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limiter := rl.limiterFor(clientIP(r))

		w.Header().Set("RateLimit-Limit", strconv.Itoa(rl.requests))
		w.Header().Set("RateLimit-Remaining", strconv.Itoa(int(limiter.Tokens())))

		if !limiter.Allow() {
			w.Header().Set("Retry-After", strconv.Itoa(int(rl.window.Seconds())))
			httpx.Raw(w, r, http.StatusTooManyRequests, rateLimitBody{
				Status:  "error",
				Message: "Too many requests.",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	// chi's RealIP middleware normalises X-Forwarded-For into RemoteAddr.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
