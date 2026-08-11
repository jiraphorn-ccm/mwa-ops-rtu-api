package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/migrations"
)

// SchemaStatus compares the migrations compiled into the binary with the ones
// actually applied to the database.
type SchemaStatus struct {
	Expected int64
	Applied  int64
	Dirty    bool
}

// UpToDate reports whether the database can safely serve traffic.
func (s SchemaStatus) UpToDate() bool {
	return !s.Dirty && s.Applied >= s.Expected
}

// Describe renders the status for logs and the health endpoint.
func (s SchemaStatus) Describe() string {
	if s.Dirty {
		return fmt.Sprintf("migration %d is dirty", s.Applied)
	}
	if s.Applied < s.Expected {
		return fmt.Sprintf("applied %d, expected %d", s.Applied, s.Expected)
	}
	return fmt.Sprintf("version %d", s.Applied)
}

// ExpectedSchemaVersion is the highest migration version embedded in the binary.
func ExpectedSchemaVersion() (int64, error) {
	entries, err := fs.Glob(migrations.FS, "*.up.sql")
	if err != nil {
		return 0, fmt.Errorf("read embedded migrations: %w", err)
	}

	var latest int64
	for _, name := range entries {
		prefix, _, found := strings.Cut(name, "_")
		if !found {
			continue
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("migration %q has a non-numeric version prefix", name)
		}
		if version > latest {
			latest = version
		}
	}

	if latest == 0 {
		return 0, errors.New("no embedded migrations found")
	}
	return latest, nil
}

// CheckSchema reads golang-migrate's bookkeeping table and compares it with the
// embedded migrations.
func CheckSchema(ctx context.Context, pool *pgxpool.Pool) (SchemaStatus, error) {
	expected, err := ExpectedSchemaVersion()
	if err != nil {
		return SchemaStatus{}, err
	}
	status := SchemaStatus{Expected: expected}

	row := pool.QueryRow(ctx, `SELECT version, dirty FROM public.schema_migrations LIMIT 1`)
	if err := row.Scan(&status.Applied, &status.Dirty); err != nil {
		// No bookkeeping table means nothing has been migrated yet.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == sqlStateUndefinedTable {
			return status, nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return status, nil
		}
		return status, fmt.Errorf("read schema_migrations: %w", err)
	}

	return status, nil
}
