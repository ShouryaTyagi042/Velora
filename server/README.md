# velora-server

The Go backend for Velora, a personal media server. `net/http` from the standard library, PostgreSQL through `database/sql` + `pgx`, and `ffprobe`/`ffmpeg` for video metadata. It scans a media folder on startup into Postgres, and a background job fills in each video's duration, resolution and thumbnail. Everything but the health check needs a signed-in user.

## Run

Requires Go ≥ 1.25, PostgreSQL 16 and FFmpeg (`brew install ffmpeg`). Create a role and two databases once, as a superuser (`psql -U postgres`):

```sql
create role velora login password 'velora-dev';
create database velora owner velora;
create database velora_test owner velora;  -- the integration tests wipe this one
```

Then put `VELORA_DATABASE_URL` and `VELORA_TEST_DSN` in `.env` (see `.env.example`), and create a user to sign in with. There is no signup endpoint: users are made on the server itself.

```sh
make user NAME=shourya   # asks for the password twice (at least 8 characters)
make run      # loads .env, then go run ./cmd/server
make build    # → bin/velora-server, bin/velora-admin
make fmt vet test   # test loads .env too; without VELORA_TEST_DSN the Postgres tests skip
```

`velora-admin create-user <name>` reads the password from the terminal without echo, or from the first line of stdin when it isn't a terminal. The first user created gets id 1 and with it any progress saved before phase 11.

After a video's metadata, the extractor also cuts its long-press preview (phase 14, `ffmpeg.Preview`), in small batches between metadata rounds so new videos get thumbnails first. A replaced file (new `modTime`) gets a new preview; a file that fails 3 times is left alone.

On start the server applies any new migrations (`migrations/*.sql`, embedded in the binary, each applied once and recorded in `schema_migrations`), scans the library, and starts the metadata extractor.

`Ctrl+C` (or `SIGTERM`) shuts down gracefully: new connections are refused and in-flight requests get up to 10 s to finish. A second `Ctrl+C` exits immediately.

## Environment

| Var | Default | Meaning |
|---|---|---|
| `VELORA_ADDR` | `:8080` | Listen address. `:8080` binds every interface; `127.0.0.1:8080` would make it unreachable from the phone. |
| `VELORA_MEDIA_DIR` | *(required)* | The library folder, holding `movies/`, `anime/` and `comics/` (below). The server won't start without it. |
| `VELORA_CORS_ORIGINS` | *(none)* | Comma-separated browser origins allowed to call the API, e.g. `http://localhost:8081` for Expo web. Only browsers enforce CORS; the phone app doesn't need it. Never `*`. |
| `VELORA_DATABASE_URL` | *(required)* | Postgres DSN, e.g. `postgres://velora:velora-dev@localhost:5432/velora?sslmode=disable`. It holds a password: it is logged only redacted (`postgres://velora:xxxxx@…`) and never sent to a client. |
| `VELORA_CACHE_DIR` | the OS cache dir + `/velora` | Velora's own derived files: `thumbs/{id}.jpg` and `previews/{id}.mp4`. If you delete it, also `delete from video_metadata`, or the extractor won't redo videos it already finished. |
| `VELORA_WORKERS` | half the CPUs | How many videos have metadata and thumbnails extracted at once (1–64). |
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
├── anime/                         ← shown in the app's Anime tab
│   ├── Cowboy Bebop/Session 1.mkv ← the folder name is the episode's series (not an actor)
│   └── OVA.mkv                    ← no series folder: no series
└── comics/
    └── Watchmen/                  ← one comic; title = folder name
        ├── page-1.png             ← pages in natural order: page-2 before page-10
        └── page-10.png
```

- Anime are videos (same streaming, thumbnails, metadata and watch progress), with `"section":"anime"` instead of `"movies"`. The folder under `anime/` is the `series` (stored in `media.series`, indexed), never a folder actor: a series is a show, an actor a person, and keeping them apart keeps the actor list and actor filter about people. Anime can still get manual actors (voice actors) in the app. The section is the top-level folder, so moving a file between `movies/` and `anime/` makes it a new item, like any rename.
- Videos: `.mp4 .mkv .webm .mov`. Comic pages: `.jpg .jpeg .png .webp`. Other files are ignored.
- One file per actor folder. Further actors are added in the app (`POST /api/media/{id}/actors`), not by copying files.
- Hidden files (`.DS_Store`, `._*`) are skipped. Anything else at the root, a comic with no images, or a
  subfolder inside a comic produces a scan warning, not a failure.
- Folder names are case-sensitive: `Movies/` is not `movies/`.
- Symlinks are not followed.
- Each item is recognised by a hash of its path, so renaming a file or folder makes it a new item (new id, no progress, no manual actors); the old one is marked missing, not deleted.

## API

Every response is JSON (`Content-Type: application/json; charset=utf-8`) and carries an `X-Request-ID` header that matches the server's log line for that request.

### Signing in

`POST /api/auth/login` returns a session token. Every other `/api/*` request sends it as `Authorization: Bearer <token>`; without a valid one the answer is `401 unauthorized` (with `WWW-Authenticate: Bearer`), before the route is even looked up. `/health` is the only other open route.

- The token is 32 random bytes, base64url. The server stores only its SHA-256, so a leaked `sessions` table holds no live sessions. Passwords are bcrypt, cost 12.
- Sessions last 30 days from last use: each use pushes the expiry forward (at most one write a day). Logout deletes the session at once.
- Login allows 5 attempts a minute per client address, then `429 rate_limited` with `Retry-After`. A wrong password and an unknown username give the same answer.
- Progress is per user; media, actors and scans are shared by everyone who can sign in.

| Method & path | Success | Errors |
|---|---|---|
| `POST /api/auth/login` | `200 {"token":"…","user":{"username":"shourya"}}` (`Cache-Control: no-store`). Body `{"username","password"}` | `400 validation`, `401 unauthorized` (wrong username or password), `429 rate_limited` |
| `POST /api/auth/logout` | `204`: this token stops working | `401` |
| `GET /api/auth/me` | `200 {"username":"shourya"}` | `401` |

### Library

| Method & path | Success | Errors |
|---|---|---|
| `GET /health` | `200 {"status":"ok"}` | |
| `GET /api/media` | One page: `200 {"items":[Media, ...], "nextCursor":"…"\|null}`, sorted by title in natural order (`Episode 2` before `Episode 10`), ignoring ASCII case. `ETag` + `Cache-Control: private, no-cache`; `If-None-Match` with the same tag gives `304` and no body | |
| `GET /api/media?kind=video\|comic` | that kind only | `400 validation` for any other `kind` |
| `GET /api/media?section=movies\|anime` | videos from `movies/` or `anime/` only (no comics) | `400 validation` for any other `section` |
| `GET /api/media?series=<name>` | anime episodes of that series (exact match) | `400 validation` if empty or over 200 bytes |
| `GET /api/media?actor=<name>` | with that actor (exact match); an unknown actor gives `[]` | `400 validation` if empty or over 200 bytes |
| `GET /api/media?q=<text>` | titles, actor names or anime series containing `text`, ignoring case and accents (`amelie` finds `Amélie`); blank is no search | `400 validation` if over 200 bytes |
| `GET /api/media?sort=title\|title_desc\|recent\|random` | `recent`: newest first (by when the scan first saw it). `random`: a fresh shuffle on every call (`Cache-Control: no-store`), never a `nextCursor`: ask again for more | `400 validation` for any other `sort`, or `after` with `random` |
| `GET /api/media?limit=<n>&after=<cursor>` | `limit` 1–200 (default 50); `after` is the previous page's `nextCursor`. All the filters combine | `400 validation` for a bad `limit`, or a cursor that isn't from this list's `sort` |
| `GET /api/media/{id}` | `200 Media` | `404 not_found` |
| `GET /api/media/{id}/thumbnail` | `200 image/jpeg` (a frame from 10% in, up to 960 px wide (never enlarged; thumbnails made before this change stay 480 until the file changes); `private, max-age=86400`, or `private, immutable` with `?v=<the video's modTime>`); a comic's is its first page; `202 {"status":"pending"}` while a video's is still being made | `404 not_found` |
| `GET /api/media?clip=preview\|short` | only videos whose long-press preview, or Shorts clip, is ready for the file as it is now | `400 validation` for any other `clip` |
| `GET /api/media/{id}/short` | `200 video/mp4`: the Shorts feed's clip, eight 8 s cuts from 8% to 92% of the video, up to 720 px high, with sound. Served like `/preview` (ranges, `immutable` with `?v=<modTime>`); the extractor makes these after every preview is done | `404 not_found` until it exists (`video.short` says when) |
| `GET /api/media/{id}/pages` | `200 {"pageCount":24}` | `404 not_found` if not a comic |
| `GET /api/media/{id}/pages/{n}` | `200` the image of page `n` (0-based), with `Last-Modified`/`304` and ranges; with `?v=<modTime>` also `Cache-Control: private, immutable` | `400 validation` if `n` isn't a number; `404 not_found` if out of range, not a comic, or deleted since the scan |
| `POST /api/media/{id}/actors` | `200 Media` with the actor added; adding one it already has is a no-op `200`. Body `{"name":"Carrie-Anne Moss"}`, 1–100 characters after trimming | `400 validation` (bad body, or a comic), `404 not_found` |
| `DELETE /api/media/{id}/actors/{name}` | `204` | `404 not_found` (no such link), `409 conflict` for a folder actor (rename the folder instead) |
| `GET /api/actors` | `200 {"items":[{"name":"Keanu Reeves","videoCount":2,"cover":{"id":"7","modTime":"…"}}]}`: available videos per actor, both sources. `cover` is their newest video with a thumbnail (absent while none has one); its thumbnail URL is `/api/media/{cover.id}/thumbnail?v={cover.modTime}`. `?section=movies\|anime` counts only that section's videos | `400 validation` for any other `section` |
| `GET /api/series` | `200 {"items":[{"name":"Cowboy Bebop","episodeCount":26,"cover":{"id":"12","modTime":"…"}}]}`: available episodes per anime series, in natural order. `cover` is the first episode (natural order) with a thumbnail | |
| `GET /api/media?bin=true&sort=binned` | what's in the bin, most recently binned first. Without `bin=true`, every list leaves binned items out (so do Continue, actors, series, tags and search) | `400 validation` for `sort=binned` without `bin=true` |
| `GET /api/media?favourites=true` | the signed-in user's favourites | |
| `GET /api/media?tag=<name>` | videos with that tag (matched lowercased, spaces collapsed); `q` matches tags too | `400 validation` if empty or over 200 bytes |
| `POST /api/media/{id}/bin` | `200 Media` with `inBin` and `binnedAt`: hidden everywhere but the bin; files, progress, actors and tags kept | `404 not_found` |
| `DELETE /api/media/{id}/bin` | `200 Media`: restored as it was | `404 not_found` |
| `DELETE /api/media/{id}` | `204`: deletes an item **in the bin** for good: its file (or comic folder) inside the library, its thumbnail, preview and Shorts clip, then its row (progress, favourites, actor and tag links with it) | `409 conflict` if it isn't in the bin; `404 not_found` |
| `DELETE /api/bin` | `200 {"deleted":n,"failed":m}`: everything in the bin (only `?kind=video\|comic` and/or `?section=movies\|anime` when given), deleted as above; an item whose files can't be removed stays in the bin and is counted in `failed` | |
| `PUT` / `DELETE /api/media/{id}/favourite` | `200 Media` with `favourite` for the signed-in user; both idempotent | `404 not_found` |
| `POST /api/media/{id}/tags` | `200 Media` with the tag added (videos only). Body `{"name":"Sci Fi"}`, stored as `sci fi`; 1–40 characters, no `/ # ?` | `400 validation` (bad name, or a comic), `404 not_found` |
| `DELETE /api/media/{id}/tags/{name}` | `200 Media` without it | `404 not_found` |
| `GET /api/tags` | `200 {"items":[{"name":"sci fi","videoCount":3}]}`: tags on available, unbinned videos, by name; `?section=movies\|anime` counts only that section's videos | `400 validation` for any other `section` |
| `POST /api/media/{id}/views` | `200 {"views":12,"counted":true}`: one view (the app sends it past 30 s of playback, or at the end of a shorter video, and when a comic reaches its last page). The same person again within 30 minutes is the same view: `counted:false`. Every `Media` carries `views`, everyone's total | `404 not_found` |
| `PUT /api/media/{id}/progress` | `200 Progress` as stored. Body: `{"positionMs","durationMs","clientTs"}` for a video, `{"page","pageCount","clientTs"}` for a comic. A save older (by `clientTs`) than the stored one is accepted and ignored | `400 validation`, `404 not_found` |
| `GET /api/media/{id}/progress` | `200 Progress` | `404 not_found` if never started |
| `GET /api/continue` | `200 {"watching":[Media],"reading":[Media]}`: the signed-in user's unfinished items, newest first, 10 each. `ETag`/`304` like the list | |
| `GET /api/media/{id}/preview?v=<modTime>` | `200 video/mp4`: the long-press preview, a silent ~12 s montage of four 3 s cuts (at 15/35/55/75%), ≤360 px high, 24 fps, faststart. Ranges work; `private, immutable` with `?v=`. Present exactly when the video's `"video":{"preview":true}` | `404 not_found`: not a video, or no preview of the file as it is now (not made yet, failed, or the file changed since) |
| `GET` or `HEAD /api/media/{id}/stream` | `200` whole video, or `206 Partial Content` for a `Range: bytes=…` request (seeking); `304` for a matching `If-Modified-Since`. `Content-Type` comes from the file's extension; `Accept-Ranges: bytes` on every response | `404 not_found` for an unknown id, a comic, or a file deleted since the last scan; `416` for a range past the end |
| `POST /api/scan` | `202 Job` at once, with `Location: /api/jobs/{id}`; the scan of `VELORA_MEDIA_DIR` runs in the background. While one runs, every request gets that same job. When done, the job's `result` is `{"found":6,"missing":0,"warnings":[...]}` | |
| `GET /api/jobs/{id}` | `200 {"id":"j_…","name":"scan","status":"running","startedAt":…}`. `status` is `running`, `done`, `failed` (the server log has why) or `canceled` (shutdown); `progress: {"done":120,"total":200}` while known. `Cache-Control: no-store` | `404 not_found`: unknown, or forgotten (the last 50 jobs are kept, in memory) |
| `GET /api/jobs` | `200 {"items":[Job, ...]}`, newest first: scans, and extraction rounds (`"name":"extract"`) | |

Any other method on a known path returns `405`. Unknown paths return the mux's plain-text `404` (under `/api/`, only once signed in). Images are `Cache-Control: private`: the phone may cache them, a shared proxy must not, since they were fetched with a token.

`Media`:

```json
{ "id": "7", "title": "The Matrix", "kind": "video", "section": "movies", "mimeType": "video/mp4",
  "sizeBytes": 2147483648, "modTime": "2026-10-04T00:00:00Z", "addedAt": "2026-10-04T00:00:00Z",
  "actors": [{ "name": "Keanu Reeves", "source": "folder" }, { "name": "Carrie-Anne Moss", "source": "manual" }],
  "video": { "durationMs": 8160000, "width": 1920, "height": 800, "codec": "h264", "fps": 23.976 },
  "progress": { "positionMs": 2530000, "durationMs": 8160000, "completed": false, "updatedAt": "2026-10-28T21:14:03Z" } }

{ "id": "12", "title": "Watchmen", "kind": "comic",
  "sizeBytes": 18874368, "modTime": "2026-10-04T00:00:00Z", "addedAt": "2026-10-04T00:00:00Z",
  "comic": { "pageCount": 24 },
  "progress": { "page": 11, "pageCount": 24, "completed": false, "updatedAt": "2026-10-28T21:20:00Z" } }
```

`section` (`movies` or `anime`) appears on videos only, `series` on anime in a series folder only. `actors` appears only on videos that have one: `folder` actors come from the folder layout and change only by renaming it; `manual` ones are added in the app and survive rescans. `video` appears once the extractor has run, `progress` once the signed-in user started the item (`completed` is decided by the server: 95% of a video, or a comic's last page). `comic` appears only on comics. File paths and page filenames are never sent; clients refer to media by `id` only.

### Errors

Every error has the same shape:

```json
{ "error": { "code": "not_found", "message": "media m9 not found" } }
```

`code` is stable, so clients switch on it. `message` is for humans and may change.

| Status | `code` | When |
|---|---|---|
| 400 | `validation` | bad query parameter or request body |
| 401 | `unauthorized` | no session, an unknown or expired token, or a failed login |
| 404 | `not_found` | no media with that id |
| 409 | `conflict` | the request is valid but the data refuses it (removing a folder actor) |
| 429 | `rate_limited` | too many login attempts from one address; see `Retry-After` |
| 500 | `internal` | anything unexpected; details go to the server log, never to the client |
| 501 | `not_implemented` | endpoint exists but isn't built yet |

## Reaching it from the phone

1. The phone and Mac must be on the same Wi-Fi, and it must not use AP/client isolation (guest and corporate networks often do). The Mac's hotspot works as a fallback.
2. Find the Mac's LAN IP: `ipconfig getifaddr en0`.
3. From the phone's browser: `http://<mac-ip>:8080/health` → `{"status":"ok"}` (the one route that needs no sign-in).
4. On first run, macOS asks whether to allow incoming connections. Allow it.

Check what it's bound to: `lsof -nP -iTCP:8080 -sTCP:LISTEN` should show `*:8080`.

## Curriculum

Phase docs, ADRs and the learning log live in the engineering archive under
`learning/projects/velora/`.
