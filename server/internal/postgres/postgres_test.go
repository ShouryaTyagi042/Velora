package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
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
	if _, err := Migrate(context.Background(), db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	// One row per file, not per number up to the latest: numbers may skip (0009 is phase 14's).
	files, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`select count(*) from schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != len(files) {
		t.Errorf("schema_migrations has %d rows, want %d (one per migration)", n, len(files))
	}
}

func TestExtractorQueue(t *testing.T) {
	repo := NewMediaRepo(emptyDB(t))
	ctx := context.Background()
	if _, err := repo.Sync(ctx, mediatest.Library(), time.Now()); err != nil {
		t.Fatal(err)
	}

	// Four videos pending; the comic never is. A limit is respected.
	if two, err := repo.Pending(ctx, 2); err != nil || len(two) != 2 {
		t.Fatalf("Pending(2) = %d videos, %v", len(two), err)
	}
	pending, err := repo.Pending(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 4 {
		t.Fatalf("Pending = %d videos, want 4", len(pending))
	}
	for _, m := range pending {
		if m.Kind != media.KindVideo || m.RelPath == "" {
			t.Fatalf("Pending returned %+v", m)
		}
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
	// Extracted ones and ones that failed MaxAttempts times are no longer pending.
	if left, err := repo.Pending(ctx, 100); err != nil || len(left) != 0 {
		t.Errorf("Pending after extraction = %d videos, %v; want none", len(left), err)
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

// TestTitleKeyMatchesGo checks velora_title_key against its Go twin, media.TitleKey: memstore
// sorts by one and Postgres by the other, and the shared suite assumes they agree.
func TestTitleKeyMatchesGo(t *testing.T) {
	db := emptyDB(t)
	for _, s := range []string{"", "Episode 2", "Episode 01 – Night Train", "007", "ABC", "Éclair 9",
		"x 123456789012345678901234 y", "0", "000", "a1b22c333", "Watchmen", "它 12"} {
		var got string
		if err := db.QueryRow(`select velora_title_key($1)`, s).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if want := media.TitleKey(s); got != want {
			t.Errorf("velora_title_key(%q) = %q, Go TitleKey = %q", s, got, want)
		}
	}
}

// TestPreviewQueue: a video is pending a preview once it has metadata; making it clears it and
// shows preview: true; a changed file (new mod_time) makes it stale again; failures are bounded.
func TestPreviewQueue(t *testing.T) {
	repo := NewMediaRepo(emptyDB(t))
	ctx := context.Background()
	lib := mediatest.Library()
	if _, err := repo.Sync(ctx, lib, time.Now()); err != nil {
		t.Fatal(err)
	}
	if p, _ := repo.PendingPreviews(ctx, 100); len(p) != 0 {
		t.Fatalf("PendingPreviews before metadata = %d, want 0", len(p))
	}
	pending, _ := repo.Pending(ctx, 100)
	for _, m := range pending {
		if err := repo.SetVideoMeta(ctx, m.ID, media.VideoMeta{DurationMs: 60_000, Width: 640, Height: 360, Codec: "h264", FPS: 24}); err != nil {
			t.Fatal(err)
		}
	}
	todo, err := repo.PendingPreviews(ctx, 100)
	if err != nil || len(todo) != 4 {
		t.Fatalf("PendingPreviews = %d, %v; want 4", len(todo), err)
	}
	first := todo[0]
	if first.Video == nil || first.Video.DurationMs != 60_000 || first.ModTime.IsZero() || first.RelPath == "" {
		t.Fatalf("pending preview = %+v", first)
	}
	if err := repo.SetPreview(ctx, first.ID, first.ModTime); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx, 0, first.ID); got.Video == nil || !got.Video.Preview {
		t.Errorf("after SetPreview: video = %+v, want preview true", got.Video)
	}
	if left, _ := repo.PendingPreviews(ctx, 100); len(left) != 3 {
		t.Errorf("PendingPreviews after one = %d, want 3", len(left))
	}

	// The file changes: the next scan stores a new mod_time and the preview is stale.
	for i := range lib {
		if lib[i].Title == first.Title {
			lib[i].ModTime = lib[i].ModTime.Add(time.Hour)
		}
	}
	if _, err := repo.Sync(ctx, lib, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx, 0, first.ID); got.Video.Preview {
		t.Error("preview of a changed file still reported current")
	}
	if left, _ := repo.PendingPreviews(ctx, 100); len(left) != 4 {
		t.Errorf("PendingPreviews after a change = %d, want 4", len(left))
	}

	// Failures: after MaxAttempts, no longer pending.
	for range extractor.MaxAttempts {
		if err := repo.RecordPreviewFailure(ctx, first.ID, "invalid data"); err != nil {
			t.Fatal(err)
		}
	}
	if left, _ := repo.PendingPreviews(ctx, 100); len(left) != 3 {
		t.Errorf("PendingPreviews after %d failures = %d, want 3", extractor.MaxAttempts, len(left))
	}
}

// TestSeriesColumn: the scan stores each item's series in media.series by media.SeriesOf (whatever
// the item says), null for everything but anime in a series folder, and the check refuses the rest.
func TestSeriesColumn(t *testing.T) {
	db := emptyDB(t)
	repo := NewMediaRepo(db)
	ctx := context.Background()
	ep := mediatest.Library()[0]
	ep.Fingerprint, ep.ID, ep.RelPath, ep.Title, ep.Actors = "fp:ep", "fp:ep", "anime/Cowboy Bebop/Session 1.mkv", "Session 1", nil
	ep.Series = "" // the scanner sets it, but the store mustn't depend on that
	lib := append(mediatest.Library(), ep)
	if _, err := repo.Sync(ctx, lib, time.Now()); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`select title, coalesce(series, '<null>') from media order by title`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for rows.Next() {
		var title, series string
		rows.Scan(&title, &series)
		got[title] = series
	}
	rows.Close()
	if got["Session 1"] != "Cowboy Bebop" || got["The Matrix"] != "<null>" || got["Watchmen"] != "<null>" {
		t.Errorf("series column = %v", got)
	}
	if _, err := db.Exec(`update media set series = '' where title = 'Session 1'`); err == nil {
		t.Error("an empty series was accepted")
	}
	if _, err := db.Exec(`update media set series = 'X' where title = 'Watchmen'`); err == nil {
		t.Error("a comic with a series was accepted")
	}

	// A folder renamed under the same fingerprint (memstore can't, but the column must follow
	// rel_path): the upsert rewrites it.
	lib[len(lib)-1].RelPath = "anime/Bebop/Session 1.mkv"
	if _, err := repo.Sync(ctx, lib, time.Now()); err != nil {
		t.Fatal(err)
	}
	var series string
	db.QueryRow(`select series from media where title = 'Session 1'`).Scan(&series)
	if series != "Bebop" {
		t.Errorf("series after the path changed = %q, want Bebop", series)
	}
}
