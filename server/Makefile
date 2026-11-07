.PHONY: run build test fmt vet user sources
# run, test and user load .env (if present) into the environment first.
ENV := set -a; [ -f .env ] && . ./.env; set +a;
run:   ; $(ENV) VELORA_MEDIA_DIR=$${VELORA_MEDIA_DIR:-$$HOME/velora-media} VELORA_CORS_ORIGINS=$${VELORA_CORS_ORIGINS-http://localhost:8081} go run ./cmd/server
build: ; go build -o bin/velora-server ./cmd/server && go build -o bin/velora-admin ./cmd/velora-admin
test:  ; $(ENV) go test ./...
fmt:   ; gofmt -l -w .
vet:   ; go vet ./...
# make user NAME=shourya: create a user (asks for the password twice)
user:  ; @test -n "$(NAME)" || { echo "usage: make user NAME=<name>"; exit 2; }; $(ENV) go run ./cmd/velora-admin create-user "$(NAME)"
# make sources: store each item's download page (media.source_url) from the nomedia downloader's
# records. DOWNLOADER= points elsewhere if it's installed somewhere else.
DOWNLOADER ?= $(HOME)/Library/Application Support/nomedia-series-downloader
sources: ; $(ENV) go run ./cmd/velora-admin import-sources "$(DOWNLOADER)/history.json" "$(DOWNLOADER)/pending.json"
