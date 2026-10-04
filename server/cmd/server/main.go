package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	addr := os.Getenv("VELORA_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("velora-server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
