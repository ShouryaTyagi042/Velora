package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/api"
	"github.com/ShouryaTyagi042/Velora/server/internal/auth"
	"github.com/ShouryaTyagi042/Velora/server/internal/extractor"
	"github.com/ShouryaTyagi042/Velora/server/internal/ffmpeg"
	"github.com/ShouryaTyagi042/Velora/server/internal/jobs"
	"github.com/ShouryaTyagi042/Velora/server/internal/library"
	"github.com/ShouryaTyagi042/Velora/server/internal/postgres"
	"github.com/ShouryaTyagi042/Velora/server/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// ctx is cancelled on Ctrl+C (SIGINT) or SIGTERM. Set up first, so a stop during the startup
	// scan (seconds, on a big library) is a clean stop too.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := os.Getenv("VELORA_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	mediaDir := os.Getenv("VELORA_MEDIA_DIR")
	if mediaDir == "" {
		return errors.New("VELORA_MEDIA_DIR is not set: point it at the folder holding movies/ and comics/")
	}
	if info, err := os.Stat(mediaDir); err != nil {
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("VELORA_MEDIA_DIR %s is not a directory", mediaDir)
	}

	// An os.Root refuses to open anything outside mediaDir: no "..", no absolute paths,
	// no symlinks pointing out. Both the scanner and the stream handler go through it.
	root, err := os.OpenRoot(mediaDir)
	if err != nil {
		return err
	}
	defer root.Close()

	dsn := os.Getenv("VELORA_DATABASE_URL")
	if dsn == "" {
		return errors.New("VELORA_DATABASE_URL is not set: e.g. postgres://velora:pw@localhost:5432/velora?sslmode=disable")
	}
	// The DSN holds the database password: it is logged only redacted, and never sent to a client.
	db, err := postgres.Open(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("database %s: %w", postgres.Redact(dsn), err)
	}
	defer db.Close()
	log.Printf("database: %s", postgres.Redact(dsn))
	// Migrations run on every start: each applies once, so a started server is always current.
	version, err := postgres.Migrate(context.Background(), db, migrations.FS)
	if err != nil {
		return err
	}
	log.Printf("database migrated to version %d", version)

	users := postgres.NewAuthRepo(db)
	if ok, err := users.HasUsers(context.Background()); err != nil {
		return err
	} else if !ok {
		log.Print("no users yet: nobody can sign in. Create one with: go run ./cmd/velora-admin create-user <name>")
	}
	authn := auth.New(users, auth.Options{})

	// Thumbnails are Velora's own files, derived and disposable: a cache, not the library.
	if err := ffmpeg.Check(); err != nil {
		return err
	}
	cacheDir, err := cacheDir()
	if err != nil {
		return err
	}
	thumbDir := filepath.Join(cacheDir, "thumbs")
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		return err
	}
	log.Printf("cache: %s", cacheDir)

	workers, err := workerCount()
	if err != nil {
		return err
	}

	// Background work: scans (one at a time) and metadata extraction (workers at a time). Both
	// run under jobsCtx, which shutdown cancels; runner and extracting are what it waits for.
	jobsCtx, stopJobs := context.WithCancel(ctx)
	defer stopJobs()
	registry := jobs.NewRegistry()
	runner := jobs.NewRunner(jobsCtx, registry)
	repo := postgres.NewMediaRepo(db)
	ext := extractor.New(repo, mediaDir, thumbDir, workers, registry)
	lib := &library.Library{FS: root.FS(), Store: repo, Jobs: runner, Scanned: ext.Wake}

	if _, err := lib.Rescan(ctx); err != nil {
		return fmt.Errorf("initial scan of %s: %w", mediaDir, err)
	}
	var extracting sync.WaitGroup
	extracting.Go(func() { ext.Run(jobsCtx) })
	// However run returns, background work is stopped and waited for before the deferred
	// db.Close: no job is left writing to a closed database, no ffmpeg outlives the server.
	defer func() {
		stopJobs()
		runner.Wait()
		extracting.Wait()
	}()

	srv := &http.Server{
		Handler: api.CORS(corsOrigins(), api.NewHandler(repo, serverJobs{lib, registry}, root.FS(), os.DirFS(thumbDir), authn)),
		// A client that connects and sends nothing would otherwise hold a goroutine forever.
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		// No WriteTimeout: phase 4 streams videos that take far longer than any fixed limit.
	}

	// Bind first, so "address already in use" is reported before we claim to be listening.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Printf("velora-server listening on %s", ln.Addr())

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}
	stop() // a second Ctrl+C now kills the process immediately

	// The order matters, and each step waits for the one before:
	//  1. ctx is already cancelled, so jobsCtx is too: extraction and any scan are stopping now,
	//     in parallel with step 2 (ffmpeg is killed through exec.CommandContext; a scan's
	//     transaction rolls back; neither records a failure for a cancel).
	//  2. Shutdown stops accepting connections and waits for in-flight requests. Serve returns
	//     as soon as Shutdown starts, so main must wait here, not there.
	//  3. Wait for the background work to return (below, then the deferred wait is a no-op).
	//  4. The deferred db.Close runs last: nothing is using the pool any more.
	log.Print("shutting down; waiting up to 10s for in-flight requests")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Print("requests drained; waiting for background jobs")
	stopJobs()
	runner.Wait()
	extracting.Wait()
	log.Print("jobs drained; shut down cleanly")
	return nil
}

// serverJobs is the API's view of background work: scans from the library, status from the
// registry. It satisfies api.Jobs.
type serverJobs struct {
	*library.Library
	*jobs.Registry
}

// workerCount is VELORA_WORKERS, or half the CPUs: how many videos are extracted at once.
// Half, from measurement (ADR 010): 40 videos on an 8-core M1 took 28.8 s with 1 worker, 21.1 s
// with 2, 18.4 s with 4, and 20.4 s with 8. One ffmpeg thumbnail already decodes on ~5 cores, so
// beyond 4 the workers only fight over the same cores, and the API and Postgres lose theirs.
func workerCount() (int, error) {
	s := os.Getenv("VELORA_WORKERS")
	if s == "" {
		return max(1, runtime.NumCPU()/2), nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 64 {
		return 0, fmt.Errorf("VELORA_WORKERS=%q: want a whole number from 1 to 64", s)
	}
	return n, nil
}

// cacheDir is VELORA_CACHE_DIR, or the OS's cache folder (~/Library/Caches/velora on a Mac).
func cacheDir() (string, error) {
	if d := os.Getenv("VELORA_CACHE_DIR"); d != "" {
		return d, nil
	}
	d, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("VELORA_CACHE_DIR is not set and there is no user cache dir: %w", err)
	}
	return filepath.Join(d, "velora"), nil
}

// corsOrigins reads VELORA_CORS_ORIGINS, a comma-separated list of browser origins
// allowed to call the API (e.g. http://localhost:8081 for Expo web). Empty: none.
func corsOrigins() []string {
	var origins []string
	for _, o := range strings.Split(os.Getenv("VELORA_CORS_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	if len(origins) > 0 {
		log.Printf("CORS: browser pages at %s may call the API", strings.Join(origins, ", "))
	}
	return origins
}
