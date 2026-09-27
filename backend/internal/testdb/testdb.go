// Package testdb provides isolated Postgres databases for tests.
//
// Tests need LIFTOFF_TEST_DATABASE_URL pointing at a server they may create
// schemas in (scripts/dev-db.sh url test; `make test` sets it). Each test gets its
// own schema, dropped on cleanup. Without the variable the tests fail rather than
// skip, so nothing passes by accident.
package testdb

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"liftoff/backend/database"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// URL returns the test server's URL, failing the test when it isn't configured.
func URL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("LIFTOFF_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("LIFTOFF_TEST_DATABASE_URL is not set: run `make test`, or `scripts/dev-db.sh start` and export LIFTOFF_TEST_DATABASE_URL=$(scripts/dev-db.sh url test)")
	}
	return url
}

// PostgresEmpty returns a pool whose search_path is a fresh, empty schema.
func PostgresEmpty(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := URL(t)
	ctx := context.Background()

	schema := fmt.Sprintf("t_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), url)
		if err != nil {
			return
		}
		defer c.Close(context.Background())
		c.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Postgres returns a pool on a fresh schema with all migrations applied.
func Postgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := PostgresEmpty(t)
	if err := database.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

// Exec runs each statement on pool, failing the test on error.
func Exec(t *testing.T, pool *pgxpool.Pool, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if _, err := pool.Exec(context.Background(), s); err != nil {
			t.Fatalf("exec %q: %v", strings.SplitN(s, "\n", 2)[0], err)
		}
	}
}
