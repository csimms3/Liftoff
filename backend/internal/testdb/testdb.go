// Package testdb provides isolated databases for tests.
//
// SQLite tests always run. Postgres tests run only when LIFTOFF_TEST_DATABASE_URL
// points at a server the tests may create schemas in; each test gets its own
// schema, dropped on cleanup.
package testdb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"liftoff/backend/database"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLite returns a migrated SQLite database in a temp directory.
func SQLite(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

// PostgresEmpty returns a pool whose search_path is a fresh, empty schema.
// It skips the test when LIFTOFF_TEST_DATABASE_URL is unset.
func PostgresEmpty(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("LIFTOFF_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("LIFTOFF_TEST_DATABASE_URL not set")
	}
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
	if err := database.MigratePostgres(context.Background(), pool); err != nil {
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
