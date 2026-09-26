package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tx is a transaction on either backend. Queries use ? placeholders; they are
// rewritten to $1, $2, ... on Postgres, so one query string serves both.
type tx interface {
	Exec(ctx context.Context, query string, args ...any) error
	// QueryRow scans one row; it returns ErrNotFound when there is none.
	QueryRow(ctx context.Context, query string, args []any, dest ...any) error
}

// withTx runs fn in a transaction, committing if it returns nil and rolling back
// otherwise.
func withTx(ctx context.Context, pool *pgxpool.Pool, sqlite *sql.DB, useSQLite bool, fn func(tx) error) error {
	if useSQLite {
		stx, err := sqlite.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin: %w", err)
		}
		if err := fn(sqliteTx{stx}); err != nil {
			stx.Rollback()
			return err
		}
		return stx.Commit()
	}
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

type sqliteTx struct{ tx *sql.Tx }

func (t sqliteTx) Exec(ctx context.Context, query string, args ...any) error {
	_, err := t.tx.ExecContext(ctx, query, args...)
	return err
}

func (t sqliteTx) QueryRow(ctx context.Context, query string, args []any, dest ...any) error {
	err := t.tx.QueryRowContext(ctx, query, args...).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

type pgTx struct{ tx pgx.Tx }

func (t pgTx) Exec(ctx context.Context, query string, args ...any) error {
	_, err := t.tx.Exec(ctx, rebind(query), args...)
	return err
}

func (t pgTx) QueryRow(ctx context.Context, query string, args []any, dest ...any) error {
	err := t.tx.QueryRow(ctx, rebind(query), args...).Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// rebind turns ? placeholders into Postgres's $1, $2, ...
func rebind(query string) string {
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
