package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db/sqlc"
)

// InTx runs fn inside a transaction, committing on success and rolling back on
// any error or panic.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *sqlc.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			// The context may already be cancelled, so roll back detached
			// from it to make sure the connection is released cleanly.
			rollbackCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
			defer cancel()
			if rbErr := tx.Rollback(rollbackCtx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				_ = rbErr
			}
		}
	}()

	if err := fn(sqlc.New(tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	committed = true
	return nil
}
