package httpx

import (
	"fmt"
	"sort"
	"strings"
)

// MaxLimit caps how many rows a single list response may carry.
const (
	MaxLimit     = 500
	DefaultLimit = 20
)

// Page holds the normalised pagination and sorting of a list request.
type Page struct {
	Number int
	Limit  int
	// Sort is the API-facing sort key.
	Sort string
	// SortSQL is the whitelisted SQL expression the repository may interpolate.
	SortSQL string
	Order   string
	Search  *string
}

// Offset is the SQL OFFSET matching the requested page.
func (p Page) Offset() int32 {
	return int32((p.Number - 1) * p.Limit)
}

// RowLimit is the SQL LIMIT matching the requested page.
func (p Page) RowLimit() int32 {
	return int32(p.Limit)
}

// Sortable maps API sort keys to the SQL expressions they are allowed to
// produce. Nothing outside this map ever reaches a query.
type Sortable map[string]string

// Keys lists the accepted sort keys in a stable order, for error messages.
func (s Sortable) Keys() []string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ParsePage reads page, limit, sort, order and search from the query string.
func ParsePage(q *Query, sortable Sortable, defaultSort string) Page {
	page := Page{
		Number: q.Int("page", 1, 1, 1_000_000),
		Limit:  q.Int("limit", DefaultLimit, 1, MaxLimit),
		Sort:   defaultSort,
		Order:  "DESC",
		Search: q.String("search"),
	}

	if raw, ok := q.raw("sort"); ok {
		key := strings.ToLower(raw)
		if _, allowed := sortable[key]; allowed {
			page.Sort = key
		} else {
			q.fail("sort", fmt.Sprintf("Must be one of: %s.", strings.Join(sortable.Keys(), ", ")))
		}
	}

	if raw, ok := q.raw("order"); ok {
		switch strings.ToUpper(raw) {
		case "ASC":
			page.Order = "ASC"
		case "DESC":
			page.Order = "DESC"
		default:
			q.fail("order", "Must be one of: ASC, DESC.")
		}
	}

	page.SortSQL = sortable[page.Sort]
	return page
}

// Meta is the `data.meta` object of a paginated list response.
type Meta struct {
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
	Total      int64  `json:"total"`
	TotalPages int64  `json:"total_pages"`
	HasNext    bool   `json:"has_next"`
	HasPrev    bool   `json:"has_prev"`
	Sort       string `json:"sort"`
	Order      string `json:"order"`
}

// List is the `data` object of a paginated list response.
type List[T any] struct {
	Items []T  `json:"items"`
	Meta  Meta `json:"meta"`
}

// NewList builds a paginated payload, guaranteeing `items` is never null.
func NewList[T any](items []T, page Page, total int64) List[T] {
	if items == nil {
		items = []T{}
	}

	totalPages := int64(0)
	if page.Limit > 0 {
		totalPages = (total + int64(page.Limit) - 1) / int64(page.Limit)
	}

	return List[T]{
		Items: items,
		Meta: Meta{
			Page:       page.Number,
			Limit:      page.Limit,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    int64(page.Number) < totalPages,
			HasPrev:    page.Number > 1,
			Sort:       page.Sort,
			Order:      page.Order,
		},
	}
}

// Collection is the `data` object of a list response that is not paginated.
type Collection[T any] struct {
	Items []T `json:"items"`
}

// NewCollection builds an unpaginated payload, guaranteeing `items` is never null.
func NewCollection[T any](items []T) Collection[T] {
	if items == nil {
		items = []T{}
	}
	return Collection[T]{Items: items}
}

// Deleted is the `data` object returned by a soft delete.
type Deleted struct {
	ID          any  `json:"id"`
	SoftDeleted bool `json:"soft_deleted"`
}

// Removed is the `data` object returned by a hard delete.
type Removed struct {
	ID      any  `json:"id"`
	Deleted bool `json:"deleted"`
}
