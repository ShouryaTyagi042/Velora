package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/api"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
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

	repo := memstore.New(seed())

	srv := &http.Server{
		Handler: api.NewHandler(repo),
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

// seed returns hardcoded records until phase 3's scanner reads them from disk.
func seed() []media.Media {
	added := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	return []media.Media{
		{ID: "m1", Title: "The Matrix", Kind: media.KindVideo, Path: "/media/videos/the-matrix.mp4", SizeBytes: 2_147_483_648, AddedAt: added},
		{ID: "m2", Title: "Interstellar", Kind: media.KindVideo, Path: "/media/videos/interstellar.mkv", SizeBytes: 4_831_838_208, AddedAt: added},
		{ID: "m3", Title: "Saga Vol. 1", Kind: media.KindComic, Path: "/media/comics/saga-v1.cbz", SizeBytes: 157_286_400, AddedAt: added},
		{ID: "m4", Title: "Watchmen", Kind: media.KindComic, Path: "/media/comics/watchmen.cbz", SizeBytes: 314_572_800, AddedAt: added},
	}
}
