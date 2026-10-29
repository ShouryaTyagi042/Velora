.PHONY: run build test fmt vet
# run and test load .env (if present) into the environment first.
ENV := set -a; [ -f .env ] && . ./.env; set +a;
run:   ; $(ENV) VELORA_MEDIA_DIR=$${VELORA_MEDIA_DIR:-$$HOME/velora-media} VELORA_CORS_ORIGINS=$${VELORA_CORS_ORIGINS-http://localhost:8081} go run ./cmd/server
build: ; go build -o bin/velora-server ./cmd/server
test:  ; $(ENV) go test ./...
fmt:   ; gofmt -l -w .
vet:   ; go vet ./...
