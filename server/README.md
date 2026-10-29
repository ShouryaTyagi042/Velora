# velora-server

The Go backend for Velora, a personal media server. `net/http` from the standard library and PostgreSQL through `database/sql` + `pgx`. It scans a media folder on startup into Postgres.

## Run

Requires Go ≥ 1.25 and PostgreSQL 16. Create a role and two databases once, as a superuser (`psql -U postgres`):

```sql
create role velora login password 'velora-dev';
create database velora owner velora;
create database velora_test owner velora;  -- the integration tests wipe this one
```

Then put `VELORA_DATABASE_URL` and `VELORA_TEST_DSN` in `.env` (see `.env.example`).

```sh
make run      # loads .env, then go run ./cmd/server
make build    # → bin/velora-server
make fmt vet test   # test loads .env too; without VELORA_TEST_DSN the Postgres tests skip
```

On start the server applies any new migrations (`migrations/*.sql`, embedded in the binary, each applied once and recorded in `schema_migrations`) and scans the library.

`Ctrl+C` (or `SIGTERM`) shuts down gracefully: new connections are refused and in-flight requests get up to 10 s to finish. A second `Ctrl+C` exits immediately.

## Environment

| Var | Default | Meaning |
|---|---|---|
| `VELORA_ADDR` | `:8080` | Listen address. `:8080` binds every interface; `127.0.0.1:8080` would make it unreachable from the phone. |
| `VELORA_MEDIA_DIR` | *(required)* | The library folder, holding `movies/` and `comics/` (below). The server won't start without it. |
| `VELORA_CORS_ORIGINS` | *(none)* | Comma-separated browser origins allowed to call the API, e.g. `http://localhost:8081` for Expo web. Only browsers enforce CORS; the phone app doesn't need it. Never `*`. |
| `VELORA_DATABASE_URL` | *(required)* | Postgres DSN, e.g. `postgres://velora:velora-dev@localhost:5432/velora?sslmode=disable`. |
| `VELORA_TEST_DSN` | *(none)* | Tests only: a database the Postgres tests may wipe. Unset, they skip. |

`.env.example` documents the variables. `make run` and `make test` load `.env`; plain `go run` doesn't. `make run` defaults `VELORA_MEDIA_DIR` to `~/velora-media` and `VELORA_CORS_ORIGINS` to `http://localhost:8081`.

## Library layout

The folder structure is data (ADR 013 in the curriculum):

```text
<VELORA_MEDIA_DIR>/
├── movies/
│   ├── Keanu Reeves/              ← the folder name is the video's actor
│   │   ├── The Matrix.mp4         ← one video; title = filename without extension
│   │   └── Extras/Deleted.mkv     ← nested: still Keanu Reeves
│   └── Loose Clip.mp4             ← no actor folder: no actors
└── comics/
    └── Watchmen/                  ← one comic; title = folder name
        ├── page-1.png             ← pages in natural order: page-2 before page-10
        └── page-10.png
```

- Videos: `.mp4 .mkv .webm .mov`. Comic pages: `.jpg .jpeg .png .webp`. Other files are ignored.
- One file per actor folder. Further actors are added in the app (`POST /api/media/{id}/actors`), not by copying files.
- Hidden files (`.DS_Store`, `._*`) are skipped. Anything else at the root, a comic with no images, or a
  subfolder inside a comic produces a scan warning, not a failure.
- Folder names are case-sensitive: `Movies/` is not `movies/`.
- Symlinks are not followed.
- Each item is recognised by a hash of its path, so renaming a file or folder makes it a new item (new id, no manual actors); the old one is marked missing, not deleted.

## API

Every response is JSON (`Content-Type: application/json; charset=utf-8`) and carries an `X-Request-ID` header that matches the server's log line for that request.

| Method & path | Success | Errors |
|---|---|---|
| `GET /health` | `200 {"status":"ok"}` | |
| `GET /api/media` | `200 {"items":[Media, ...]}`, sorted by title | |
| `GET /api/media?kind=video\|comic` | `200 {"items":[...]}` of that kind | `400 validation` for any other `kind` |
| `GET /api/media?actor=<name>` | `200 {"items":[...]}` with that actor (exact match; combines with `kind`); an unknown actor gives `[]` | `400 validation` if empty or over 200 bytes |
| `GET /api/media/{id}` | `200 Media` | `404 not_found` |
| `GET /api/media/{id}/thumbnail` | | `501 not_implemented` (phase 9) |
| `POST /api/media/{id}/actors` | `200 Media` with the actor added; adding one it already has is a no-op `200`. Body `{"name":"Carrie-Anne Moss"}`, 1–100 characters after trimming | `400 validation` (bad body, or a comic), `404 not_found` |
| `DELETE /api/media/{id}/actors/{name}` | `204` | `404 not_found` (no such link), `409 conflict` for a folder actor (rename the folder instead) |
| `GET /api/actors` | `200 {"items":[{"name":"Keanu Reeves","videoCount":2}]}`: available videos per actor, both sources | |
| `GET` or `HEAD /api/media/{id}/stream` | `200` whole video, or `206 Partial Content` for a `Range: bytes=…` request (seeking); `304` for a matching `If-Modified-Since`. `Content-Type` comes from the file's extension; `Accept-Ranges: bytes` on every response | `404 not_found` for an unknown id, a comic, or a file deleted since the last scan; `416` for a range past the end |
| `POST /api/scan` | `200 {"found":6,"warnings":[...]}` after rescanning `VELORA_MEDIA_DIR`; blocks until done | `500 internal` if the folder can't be read |

Any other method on a known path returns `405`. Unknown paths return the mux's plain-text `404`.

`Media`:

```json
{ "id": "7", "title": "The Matrix", "kind": "video", "mimeType": "video/mp4",
  "sizeBytes": 2147483648, "modTime": "2026-10-04T00:00:00Z", "addedAt": "2026-10-04T00:00:00Z",
  "actors": [{ "name": "Keanu Reeves", "source": "folder" }, { "name": "Carrie-Anne Moss", "source": "manual" }] }

{ "id": "12", "title": "Watchmen", "kind": "comic",
  "sizeBytes": 18874368, "modTime": "2026-10-04T00:00:00Z", "addedAt": "2026-10-04T00:00:00Z",
  "comic": { "pageCount": 24 } }
```

`actors` appears only on videos that have one: `folder` actors come from the folder layout and change only by renaming it; `manual` ones are added in the app and survive rescans. `comic` appears only on comics. File paths and page filenames are never sent; clients refer to media by `id` only.

### Errors

Every error has the same shape:

```json
{ "error": { "code": "not_found", "message": "media m9 not found" } }
```

`code` is stable, so clients switch on it. `message` is for humans and may change.

| Status | `code` | When |
|---|---|---|
| 400 | `validation` | bad query parameter or request body |
| 404 | `not_found` | no media with that id |
| 409 | `conflict` | the request is valid but the data refuses it (removing a folder actor) |
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
