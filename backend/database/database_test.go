package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

var fastTiming = monitorTiming{minDelay: time.Millisecond, maxDelay: 4 * time.Millisecond, healthEvery: time.Millisecond}

// pingScript returns a ping that fails for the given call numbers (1-based) and
// records which call made the database ready, via the callback.
func pingScript(fail map[int32]bool, calls *atomic.Int32) func(context.Context) error {
	return func(context.Context) error {
		if fail[calls.Add(1)] {
			return errors.New("connection refused")
		}
		return nil
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestMonitor_ConnectsAndMigratesAfterFailures(t *testing.T) {
	db := &Database{}
	var pings, migrations atomic.Int32
	ping := pingScript(map[int32]bool{1: true, 2: true}, &pings)
	migrate := func(context.Context) error { migrations.Add(1); return nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go db.monitor(ctx, ping, migrate, false, fastTiming, t.Errorf)

	waitFor(t, "ready", db.Ready)
	waitFor(t, "a few health pings", func() bool { return pings.Load() > 6 })
	if n := migrations.Load(); n != 1 {
		t.Errorf("migrations ran %d times, want exactly once", n)
	}
}

func TestMonitor_LostConnectionIsNotReadyUntilBack(t *testing.T) {
	db := &Database{}
	db.ready.Store(true)
	var pings atomic.Int32
	// Healthy, one failed ping (a blip), healthy, then down for four pings, then
	// healthy again.
	ping := pingScript(map[int32]bool{2: true, 4: true, 5: true, 6: true, 7: true}, &pings)
	var blipFlipped, sawNotReady atomic.Bool
	checked := func(ctx context.Context) error {
		n := pings.Load() + 1 // this call's number
		if n == 3 && !db.Ready() {
			blipFlipped.Store(true)
		}
		if n == 7 && !db.Ready() {
			sawNotReady.Store(true)
		}
		return ping(ctx)
	}
	migrate := func(context.Context) error { t.Error("already migrated; must not migrate again"); return nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go db.monitor(ctx, checked, migrate, true, fastTiming, t.Errorf)

	waitFor(t, "outage and recovery", func() bool { return pings.Load() > 8 && db.Ready() })
	if blipFlipped.Load() {
		t.Error("a single failed ping marked the database unavailable")
	}
	if !sawNotReady.Load() {
		t.Error("database stayed ready during the outage")
	}
}

func TestMonitor_MigrationFailureIsFatal(t *testing.T) {
	db := &Database{}
	fatal := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go db.monitor(ctx,
		func(context.Context) error { return nil },
		func(context.Context) error { return errors.New("column does not exist") },
		false, fastTiming,
		func(format string, args ...any) { fatal <- fmt.Sprintf(format, args...) })

	select {
	case msg := <-fatal:
		if !strings.Contains(msg, "column does not exist") {
			t.Errorf("fatal message %q should include the migration error", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a failing migration must be fatal, not retried")
	}
	if db.Ready() {
		t.Error("must not be ready after a failed migration")
	}
}

func TestMonitor_StopsOnCancel(t *testing.T) {
	db := &Database{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		db.monitor(ctx, func(context.Context) error { return errors.New("down") }, nil, false, fastTiming, t.Errorf)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("monitor kept running after cancel")
	}
}

// With DATABASE_URL pointing at a dead server the database starts not-ready and
// never falls back to SQLite.
func TestNewDatabase_UnreachablePostgresIsNotReadyAndNoSQLite(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })
	t.Setenv("DATABASE_URL", "postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")

	db, err := NewDatabase()
	if err != nil {
		t.Fatalf("NewDatabase: %v (an unreachable server must not be fatal)", err)
	}
	defer db.Close()
	if db.Ready() || db.IsSQLite() {
		t.Errorf("ready=%v sqlite=%v, want not ready and not SQLite", db.Ready(), db.IsSQLite())
	}
	if _, err := os.Stat("liftoff.db"); !os.IsNotExist(err) {
		t.Error("fell back to SQLite: liftoff.db was created")
	}
}

func TestNewDatabase_InvalidURLIsAnError(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://%zz")
	if _, err := NewDatabase(); err == nil {
		t.Error("want an error for a malformed DATABASE_URL")
	}
}

func TestOpenPostgres_ReachableIsReady(t *testing.T) {
	url := os.Getenv("LIFTOFF_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("LIFTOFF_TEST_DATABASE_URL not set")
	}
	// A private schema keeps the migrations off the shared public schema.
	schema := fmt.Sprintf("t_ready_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	if _, err := admin.Exec(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")

	db, err := OpenPostgres(url + "&search_path=" + schema)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if !db.Ready() {
		t.Fatal("reachable Postgres should be ready immediately")
	}
	var n int
	db.GetPool().QueryRow(context.Background(), "SELECT COUNT(*) FROM schema_migrations").Scan(&n)
	if n == 0 {
		t.Error("migrations were not applied")
	}
}

// A reachable database whose migrations fail must stop startup, not look "down".
func TestOpenPostgres_BrokenMigrationIsAnError(t *testing.T) {
	url := os.Getenv("LIFTOFF_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("LIFTOFF_TEST_DATABASE_URL not set")
	}
	schema := fmt.Sprintf("t_badmig_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	for _, q := range []string{
		"CREATE SCHEMA " + schema,
		// Wrong shape: the runner can't read versions from it.
		"CREATE TABLE " + schema + ".schema_migrations (id INT)",
	} {
		if _, err := admin.Exec(context.Background(), q); err != nil {
			t.Fatal(err)
		}
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")

	db, err := OpenPostgres(url + "&search_path=" + schema)
	if err == nil {
		db.Close()
		t.Fatal("want an error when migrations fail against a reachable database")
	}
}
