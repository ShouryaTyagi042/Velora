# velora-server

The Go backend for Velora, a personal media server. Standard library only (`net/http`); media records live in memory until phase 7.

## Run

Requires Go ≥ 1.22 (method routing in `http.ServeMux`).

```sh
make run      # go run ./cmd/server
make build    # → bin/velora-server
make fmt vet test
```

`Ctrl+C` (or `SIGTERM`) shuts down gracefully: new connections are refused and in-flight requests get up to 10 s to finish. A second `Ctrl+C` exits immediately.

## Environment

| Var | Default | Meaning |
|---|---|---|
| `VELORA_ADDR` | `:8080` | Listen address. `:8080` binds every interface; `127.0.0.1:8080` would make it unreachable from the phone. |

`.env.example` documents the variables. Nothing loads `.env` automatically, so override inline: `VELORA_ADDR=:9000 make run`.

## API

Every response is JSON (`Content-Type: application/json; charset=utf-8`) and carries an `X-Request-ID` header that matches the server's log line for that request.

| Method & path | Success | Errors |
|---|---|---|
| `GET /health` | `200 {"status":"ok"}` | |
| `GET /api/media` | `200 {"items":[Media, ...]}`, sorted by title | |
| `GET /api/media?kind=video\|comic` | `200 {"items":[...]}` of that kind | `400 validation` for any other `kind` |
| `GET /api/media/{id}` | `200 Media` | `404 not_found` |
| `GET /api/media/{id}/thumbnail` | | `501 not_implemented` (phase 9) |

Any other method on a known path returns `405`. Unknown paths return the mux's plain-text `404`.

`Media`:

```json
{ "id": "m1", "title": "The Matrix", "kind": "video", "sizeBytes": 2147483648, "addedAt": "2026-10-04T00:00:00Z" }
```

The file path is never sent; clients refer to media by `id` only.

### Errors

Every error has the same shape:

```json
{ "error": { "code": "not_found", "message": "media m9 not found" } }
```

`code` is stable, so clients switch on it. `message` is for humans and may change.

| Status | `code` | When |
|---|---|---|
| 400 | `validation` | bad query parameter |
| 404 | `not_found` | no media with that id |
| 500 | `internal` | anything unexpected; details go to the server log, never to the client |
| 501 | `not_implemented` | endpoint exists but isn't built yet |

## Reaching it from the phone

1. The phone and Mac must be on the same Wi-Fi, and it must not use AP/client isolation (guest and corporate networks often do). The Mac's hotspot works as a fallback.
2. Find the Mac's LAN IP: `ipconfig getifaddr en0`.
3. From the phone's browser: `http://<mac-ip>:8080/health` → `{"status":"ok"}`.
4. On first run, macOS asks whether to allow incoming connections. Allow it.

Check what it's bound to: `lsof -nP -iTCP:8080 -sTCP:LISTEN` should show `*:8080`.

## Curriculum

Phase docs, ADRs and the learning log live in the engineering archive under
`learning/projects/velora/`.
