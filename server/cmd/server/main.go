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
	"sync"
	"syscall"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/api"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
	"github.com/ShouryaTyagi042/Velora/server/internal/scanner"
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

	repo := memstore.New(nil)
	lib := &library{fsys: os.DirFS(mediaDir), store: repo}
	if _, _, err := lib.Rescan(context.Background()); err != nil {
		return fmt.Errorf("initial scan of %s: %w", mediaDir, err)
	}

	srv := &http.Server{
		Handler: api.NewHandler(repo, lib),
		// A client that connects and sends nothing would otherwise hold a goroutine forever.
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		// No WriteTimeout: phase 4 streams videos that take far longer than any fixed limit.
	}

	// ctx is cancelled on Ctrl+C (SIGINT) or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

// library connects the scanner to the store. It satisfies api.Rescanner.
type library struct {
	fsys  fs.FS
	store *memstore.Store
	mu    sync.Mutex // one scan at a time; a second POST /api/scan waits for the first
}

// Rescan walks the library and swaps the store's contents for what it found.
func (l *library) Rescan(ctx context.Context) (int, []string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	start := time.Now()
	res, err := scanner.Scan(ctx, l.fsys)
	if err != nil {
		return 0, nil, err
	}
	l.store.Replace(ctx, res.Items)

	log.Printf("scan: %d items, %d warnings in %s", len(res.Items), len(res.Warnings), time.Since(start))
	for _, w := range res.Warnings {
		log.Printf("scan warning: %s", w)
	}
	return len(res.Items), res.Warnings, nil
}
