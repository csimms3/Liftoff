package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Package database connects to PostgreSQL (DATABASE_URL) and keeps the schema
// current. If the database is unreachable the server still starts, reports
// not-ready (the API answers 503) and keeps retrying in the background.
// For local development, scripts/dev-db.sh runs a project-local Postgres.

// ErrNoDatabaseURL is returned when DATABASE_URL is not set.
var ErrNoDatabaseURL = errors.New("DATABASE_URL is not set (for local development: scripts/dev-db.sh start, then DATABASE_URL=$(scripts/dev-db.sh url); scripts/boot.sh does both)")

// Retry backoff while the configured Postgres is unreachable, and how often a
// connected Postgres is pinged so an outage after startup is noticed too.
const (
	retryMinDelay  = 2 * time.Second
	retryMaxDelay  = 30 * time.Second
	healthInterval = 5 * time.Second
	pingTimeout    = 3 * time.Second
)

// Database is the PostgreSQL connection pool plus its availability.
type Database struct {
	pool  *pgxpool.Pool
	ready atomic.Bool        // reachable and migrated
	stop  context.CancelFunc // stops the background monitor
}

// NewDatabase opens the database at DATABASE_URL (see OpenPostgres).
func NewDatabase() (*Database, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, ErrNoDatabaseURL
	}
	return OpenPostgres(url)
}

// OpenPostgres opens the Postgres at url. If it can't be reached now, the returned
// Database is not Ready and connects in the background. A migration that fails
// against a reachable database is an error: retrying won't fix it, and a server
// that looks alive but never serves would hide a bad deploy.
func OpenPostgres(url string) (*Database, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}
	// Connections are opened lazily, so this doesn't need the server to be up.
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}

	ctx, stop := context.WithCancel(context.Background())
	db := &Database{pool: pool, stop: stop}
	ping := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, pingTimeout)
		defer cancel()
		return pool.Ping(ctx)
	}
	migrate := func(ctx context.Context) error { return Migrate(ctx, pool) }

	migrated := false
	if err := ping(ctx); err != nil {
		log.Printf("PostgreSQL unavailable, serving 503 and retrying in the background: %v", err)
	} else {
		if err := migrate(ctx); err != nil {
			stop()
			pool.Close()
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		migrated = true
		db.ready.Store(true)
		log.Println("Database connected successfully (PostgreSQL)")
	}
	go db.monitor(ctx, ping, migrate, migrated, monitorTiming{retryMinDelay, retryMaxDelay, healthInterval}, fatalf)
	return db, nil
}

type monitorTiming struct{ minDelay, maxDelay, healthEvery time.Duration }

// fatalf ends the process; a variable so tests can observe it.
var fatalf = log.Fatalf

// monitor keeps ready accurate for the life of the server. While the database is
// unreachable it retries with exponential backoff; once reachable it migrates
// (first time only) and marks ready; after that it pings every healthEvery and
// drops back to not-ready after two failed pings in a row (one slow ping, e.g.
// every pooled connection busy, isn't an outage). A migration failure is fatal.
func (db *Database) monitor(ctx context.Context, ping, migrate func(context.Context) error, migrated bool, t monitorTiming, fatal func(string, ...any)) {
	delay := t.minDelay
	failures := 0
	for {
		wait := delay
		if db.ready.Load() && failures == 0 {
			wait = t.healthEvery // after one failed ping, recheck sooner
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		if err := ping(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			failures++
			if db.ready.Load() && failures < 2 {
				continue
			}
			if db.ready.Swap(false) {
				log.Printf("PostgreSQL connection lost, serving 503 until it is back: %v", err)
				delay = t.minDelay
			} else {
				delay = min(delay*2, t.maxDelay)
				log.Printf("PostgreSQL still unavailable (next try in %s): %v", delay, err)
			}
			continue
		}
		if !migrated {
			if err := migrate(ctx); err != nil {
				fatal("failed to run migrations: %v", err)
				return
			}
			migrated = true
		}
		if !db.ready.Swap(true) {
			log.Println("Database connected successfully (PostgreSQL)")
		}
		failures = 0
		delay = t.minDelay
	}
}

// Ready reports whether the database is connected and migrated. Until it is,
// the API answers 503.
func (db *Database) Ready() bool {
	return db.ready.Load()
}

func (db *Database) Close() {
	if db.stop != nil {
		db.stop()
	}
	db.pool.Close()
}

func (db *Database) GetPool() *pgxpool.Pool {
	return db.pool
}
