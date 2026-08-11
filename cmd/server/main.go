// Command server runs the RTU calibration HTTP API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/shopspring/decimal"

	"github.com/rtu-api/internal/config"
	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/handler"
	"github.com/rtu-api/internal/middleware"
	"github.com/rtu-api/internal/repository"
	"github.com/rtu-api/internal/router"
	"github.com/rtu-api/internal/service"
	"github.com/rtu-api/internal/storage"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Emit numeric columns as JSON numbers rather than quoted strings.
	decimal.MarshalJSONWithoutQuotes = true

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := cfg.Logger().With("version", buildVersion())
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("connected to postgres", "max_conns", cfg.DBMaxConns)

	schema, err := db.CheckSchema(ctx, pool)
	switch {
	case err != nil:
		logger.Warn("could not verify database schema", "error", err)
	case !schema.UpToDate():
		if cfg.SchemaGuard {
			return errors.New("database schema is outdated (" + schema.Describe() + "); run `make migrate-up`")
		}
		logger.Warn("database schema is outdated", "detail", schema.Describe())
	default:
		logger.Info("database schema is up to date", "detail", schema.Describe())
	}

	store := repository.New(pool)

	s3Client, err := storage.NewS3(ctx, storage.Options{
		Region:          cfg.AWSRegion,
		Bucket:          cfg.S3Bucket,
		AppPrefix:       cfg.S3AppPrefix,
		AccessKeyID:     cfg.AWSAccessKeyID,
		SecretAccessKey: cfg.AWSSecretAccessKey,
		SignedURLTTL:    cfg.S3SignedURLTTL,
	})
	if err != nil {
		return err
	}
	if s3Client != nil {
		logger.Info("s3 storage configured", "bucket", cfg.S3Bucket, "prefix", cfg.S3AppPrefix)
	} else {
		logger.Warn("s3 storage not configured; panel image uploads are disabled")
	}

	services := service.New(store, s3Client, cfg.S3AppPrefix)
	health := handler.NewHealthHandler(cfg, pool, buildVersion())
	handlers := handler.New(cfg, services, health)

	var limiter *middleware.RateLimiter
	if cfg.RateLimitEnabled {
		limiter = middleware.NewRateLimiter(cfg.RateLimitRequests, cfg.RateLimitWindow)
		defer limiter.Close()
	}

	srv := &http.Server{
		Addr: cfg.Addr(),
		Handler: router.New(router.Deps{
			Config:      cfg,
			Logger:      logger,
			Handlers:    handlers,
			RateLimiter: limiter,
		}),
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.Addr(), "api_prefix", cfg.APIPrefix)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		return srv.Close()
	}

	logger.Info("shutdown complete")
	return nil
}

// buildVersion prefers the linker-provided version and falls back to the VCS
// revision embedded by the Go toolchain.
func buildVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" && len(setting.Value) >= 7 {
			return setting.Value[:7]
		}
	}
	return version
}
