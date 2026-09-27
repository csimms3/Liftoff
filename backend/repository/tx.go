package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tx is the part of a transaction the repositories use.
type tx interface {
	Exec(ctx context.Context, query string, args ...any) error
	// QueryRow scans one row; it returns ErrNotFound when there is none.
	QueryRow(ctx context.Context, query string, args []any, dest ...any) error
}

// withTx runs fn in a transaction, committing if it returns nil and rolling back
// otherwise.
func withTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx) error) error {
	ptx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(pgTx{ptx}); err != nil {
		ptx.Rollback(ctx)
		return err
	}
	return ptx.Commit(ctx)
}

type pgTx struct{ tx pgx.Tx }

func (t pgTx) Exec(ctx context.Context, query string, args ...any) error {
	_, err := t.tx.Exec(ctx, query, args...)
	return err
}

func (t pgTx) QueryRow(ctx context.Context, query string, args []any, dest ...any) error {
	err := t.tx.QueryRow(ctx, query, args...).Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
