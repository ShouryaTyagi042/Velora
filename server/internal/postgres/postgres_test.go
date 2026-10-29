package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/ShouryaTyagi042/Velora/server/internal/media/mediatest"
	"github.com/ShouryaTyagi042/Velora/server/migrations"
)

// testDB is a freshly migrated database, or nil when VELORA_TEST_DSN isn't set.
var testDB *sql.DB

// TestMain drops and re-creates the schema once, then migrates it. Point VELORA_TEST_DSN at a
// database you don't mind losing: everything in its public schema is deleted.
func TestMain(m *testing.M) {
	dsn := os.Getenv("VELORA_TEST_DSN")
	if dsn == "" {
		os.Exit(m.Run()) // the tests skip themselves
	}
	ctx := context.Background()
	db, err := Open(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := db.ExecContext(ctx, `drop schema public cascade; create schema public`); err != nil {
		fmt.Fprintln(os.Stderr, "reset schema:", err)
		os.Exit(1)
	}
	if _, err := Migrate(ctx, db, migrations.FS); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	testDB = db
	code := m.Run()
	db.Close()
	os.Exit(code)
}

// emptyDB skips the test without a database, and otherwise empties every table.
func emptyDB(t *testing.T) *sql.DB {
	t.Helper()
	if testDB == nil {
		t.Skip("VELORA_TEST_DSN not set")
	}
	if _, err := testDB.Exec(`truncate media, actors restart identity cascade`); err != nil {
		t.Fatal(err)
	}
	return testDB
}

func TestRepository(t *testing.T) {
	mediatest.Run(t, func(t *testing.T) mediatest.Store { return NewMediaRepo(emptyDB(t)) })
}

func TestMigrateTwiceIsANoOp(t *testing.T) {
	db := emptyDB(t)
	v, err := Migrate(context.Background(), db, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`select count(*) from schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != v {
		t.Errorf("schema_migrations has %d rows, want %d (one per migration)", n, v)
	}
}
