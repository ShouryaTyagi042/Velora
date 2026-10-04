# velora-server

The Go backend for Velora, a personal media server. In Phase 0 it answers one route, `GET /health`.

## Run

Requires Go ≥ 1.22 (method routing in `http.ServeMux`).

```sh
make run      # go run ./cmd/server
make build    # → bin/velora-server
make fmt vet test
```

## Environment

| Var | Default | Meaning |
|---|---|---|
| `VELORA_ADDR` | `:8080` | Listen address. `:8080` binds every interface; `127.0.0.1:8080` would make it unreachable from the phone. |

`.env.example` documents the variables. Nothing loads `.env` automatically, so override inline: `VELORA_ADDR=:9000 make run`.

## Reaching it from the phone

1. The phone and Mac must be on the same Wi-Fi, and it must not use AP/client isolation (guest and corporate networks often do). The Mac's hotspot works as a fallback.
2. Find the Mac's LAN IP: `ipconfig getifaddr en0`.
3. From the phone's browser: `http://<mac-ip>:8080/health` → `{"status":"ok"}`.
4. On first run, macOS asks whether to allow incoming connections. Allow it.

Check what it's bound to: `lsof -nP -iTCP:8080 -sTCP:LISTEN` should show `*:8080`.

## Curriculum

Phase docs, ADRs and the learning log live in the engineering archive under
`learning/projects/velora/`.
