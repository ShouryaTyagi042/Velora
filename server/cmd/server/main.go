package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/api"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
)

func main() {
	addr := os.Getenv("VELORA_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	repo := memstore.New(seed())
	handler := api.NewHandler(repo)

	log.Printf("velora-server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
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
