package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/auth"
	"github.com/ShouryaTyagi042/Velora/server/internal/extractor"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
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
	if _, err := testDB.Exec(`truncate media, actors, users restart identity cascade`); err != nil {
		t.Fatal(err)
	}
	return testDB
}

func TestRepository(t *testing.T) {
	mediatest.Run(t, func(t *testing.T) mediatest.Store {
		db := emptyDB(t)
		// The suite saves progress as users 1 and 2; progress rows reference users.
		users := NewAuthRepo(db)
		for _, name := range []string{"alice", "bob"} {
			if _, err := users.CreateUser(context.Background(), name, "not-a-real-hash"); err != nil {
				t.Fatal(err)
			}
		}
		return NewMediaRepo(db)
	})
}

func TestAuthRepo(t *testing.T) {
	ctx := context.Background()
	r := NewAuthRepo(emptyDB(t))

	u, err := r.CreateUser(ctx, "shourya", "$2a$hash")
	if err != nil || u.ID == 0 {
		t.Fatalf("CreateUser = %+v, %v", u, err)
	}
	if _, err := r.CreateUser(ctx, "shourya", "$2a$other"); !errors.Is(err, auth.ErrUserExists) {
		t.Errorf("duplicate CreateUser = %v, want ErrUserExists", err)
	}
	if got, hash, err := r.UserByName(ctx, "shourya"); err != nil || got != u || hash != "$2a$hash" {
		t.Errorf("UserByName = %+v, %q, %v", got, hash, err)
	}
	if _, _, err := r.UserByName(ctx, "nobody"); !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("UserByName(unknown) = %v, want ErrNotFound", err)
	}

	th := []byte("0123456789abcdef0123456789abcdef")
	exp := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if err := r.CreateSession(ctx, auth.Session{TokenHash: th, UserID: u.ID, ExpiresAt: exp}); err != nil {
		t.Fatal(err)
	}
	sess, su, err := r.Session(ctx, th)
	if err != nil || su != u || !sess.ExpiresAt.Equal(exp) {
		t.Errorf("Session = %+v, %+v, %v", sess, su, err)
	}
	later := exp.Add(24 * time.Hour)
	if err := r.ExtendSession(ctx, th, later); err != nil {
		t.Fatal(err)
	}
	if sess, _, _ := r.Session(ctx, th); !sess.ExpiresAt.Equal(later) {
		t.Errorf("after ExtendSession: expires %v, want %v", sess.ExpiresAt, later)
	}
	if err := r.DeleteSession(ctx, th); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Session(ctx, th); !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("after DeleteSession: %v, want ErrNotFound", err)
	}
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

func TestExtractorQueue(t *testing.T) {
	repo := NewMediaRepo(emptyDB(t))
	ctx := context.Background()
	if _, err := repo.Sync(ctx, mediatest.Library(), time.Now()); err != nil {
		t.Fatal(err)
	}

	// Four videos pending; the comic never is.
	seen := map[string]bool{}
	for range 10 {
		m, ok, err := repo.NextPending(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		if m.Kind != media.KindVideo || seen[m.ID] {
			t.Fatalf("NextPending = %s %s (seen: %v)", m.Kind, m.Title, seen[m.ID])
		}
		seen[m.ID] = true
		if m.Title == "Loose Clip" { // fails every time
			for range extractor.MaxAttempts {
				if err := repo.RecordFailure(ctx, m.ID, "moov atom not found"); err != nil {
					t.Fatal(err)
				}
			}
			continue
		}
		if err := repo.SetVideoMeta(ctx, m.ID, media.VideoMeta{DurationMs: 1000, Width: 1920, Height: 1080, Codec: "h264", FPS: 23.976}); err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 4 {
		t.Errorf("extracted %d videos, want 4", len(seen))
	}

	got, err := repo.List(ctx, 0, media.ListFilter{Kind: media.KindVideo})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range got {
		switch {
		case m.Title == "Loose Clip" && m.Video != nil:
			t.Errorf("failed video has metadata: %+v", m.Video)
		case m.Title != "Loose Clip" && (m.Video == nil || m.Video.FPS != 23.976):
			t.Errorf("%s: Video = %+v", m.Title, m.Video)
		}
	}
}
