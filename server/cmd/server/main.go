package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/api"
	"github.com/ShouryaTyagi042/Velora/server/internal/extractor"
	"github.com/ShouryaTyagi042/Velora/server/internal/ffmpeg"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/postgres"
	"github.com/ShouryaTyagi042/Velora/server/internal/scanner"
	"github.com/ShouryaTyagi042/Velora/server/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
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
	db, err := postgres.Open(context.Background(), dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	// Migrations run on every start: each applies once, so a started server is always current.
	version, err := postgres.Migrate(context.Background(), db, migrations.FS)
	if err != nil {
		return err
	}
	log.Printf("database migrated to version %d", version)

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

	repo := postgres.NewMediaRepo(db)
	lib := &library{fsys: root.FS(), store: repo}
	if _, _, err := lib.Rescan(context.Background()); err != nil {
		return fmt.Errorf("initial scan of %s: %w", mediaDir, err)
	}

	srv := &http.Server{
		Handler: api.CORS(corsOrigins(), api.NewHandler(repo, lib, root.FS(), os.DirFS(thumbDir))),
		// A client that connects and sends nothing would otherwise hold a goroutine forever.
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		// No WriteTimeout: phase 4 streams videos that take far longer than any fixed limit.
	}

	// ctx is cancelled on Ctrl+C (SIGINT) or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The extractor fills in metadata and thumbnails in the background, one video at a time.
	// It stops with ctx, or when run returns for any other reason; main waits for it, so no
	// ffmpeg is left running after exit.
	jobsCtx, stopJobs := context.WithCancel(ctx)
	var jobs sync.WaitGroup
	jobs.Go(func() { extractor.New(repo, mediaDir, thumbDir).Run(jobsCtx) })
	defer func() {
		stopJobs()
		jobs.Wait()
	}()

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

	// Shutdown stops accepting connections and waits for in-flight requests.
	// Serve returns as soon as Shutdown starts, so main must wait here, not there.
	log.Print("shutting down; waiting up to 10s for in-flight requests")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Print("shut down cleanly")
	return nil
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

// library connects the scanner to the store. It satisfies api.Rescanner.
type library struct {
	fsys  fs.FS
	store media.Syncer
	mu    sync.Mutex // one scan at a time; a second POST /api/scan waits for the first
}

// Rescan walks the library and makes the store match what it found, in one transaction.
func (l *library) Rescan(ctx context.Context) (int, []string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	start := time.Now()
	res, err := scanner.Scan(ctx, l.fsys)
	if err != nil {
		return 0, nil, err
	}
	sync, err := l.store.Sync(ctx, res.Items, start)
	if err != nil {
		return 0, nil, fmt.Errorf("save scan: %w", err)
	}

	log.Printf("scan: %d upserted, %d newly missing, %d warnings in %s",
		sync.Upserted, sync.Missing, len(res.Warnings), time.Since(start).Round(time.Millisecond))
	for _, w := range res.Warnings {
		log.Printf("scan warning: %s", w)
	}
	return len(res.Items), res.Warnings, nil
}
