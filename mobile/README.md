# velora-mobile

The Expo / React Native client for Velora: browse and search the library, play videos, read comics, and pick up where you left off. The design follows Apple's Liquid Glass: glass on the controls layer (native headers and tab bar, the player's and reader's controls), content edge to edge underneath, and solid fallbacks on Android, web, older iOS and with Reduce Transparency on.

## Run

```sh
npm install
cp .env.example .env     # then put your Mac's LAN IP in it
npx expo start           # scan the QR code with Expo Go
```

Run the server (`../server`, `make run`) first.

## Environment

| Var | Example | Meaning |
|---|---|---|
| `EXPO_PUBLIC_API_URL` | `http://192.168.1.6:8080` | Base URL of velora-server, as seen **from the phone**. Never `localhost`: on the phone, that means the phone. |

`EXPO_PUBLIC_*` values are inlined into the JS bundle at build time and shipped to the device. Never put a secret behind that prefix. After editing `.env`, restart `npx expo start`.

## Layout

```text
src/app/_layout.tsx                 root Stack: the tabs, and above them Details, player, reader
src/app/(tabs)/_layout.tsx          native tabs: Home, Videos, Comics, Search (glass tab bar on iOS 26)
src/app/(tabs)/(home)/index.tsx     "/": Continue watching / reading, recently added
src/app/(tabs)/(videos)/videos.tsx  videos, 2 across, filtered by actor (in the URL); pull to rescan
src/app/(tabs)/(comics)/comics.tsx  comic covers, 3 across; pull to rescan
src/app/(tabs)/(search)/search.tsx  titles and actors, searched on the phone
src/app/media/[id].tsx              details: Play / Resume / Read, actors (add and remove manual ones)
src/app/player/[id].tsx             expo-video with glass controls (native controls in fullscreen); resumes
src/app/reader/[id].tsx             paged comic reader: pinch-zoom, preload, direction setting; resumes
src/theme/                          design tokens (colours light + dark, space, radius, type roles, glass roles)
src/api/                            getJSON/sendJSON, ApiError, Media types, URLs for streams, pages, thumbnails
src/progress/                       local-first progress saves (throttled, one in flight) and launch reconcile
src/hooks/                          useAsync, usePlayerStatus, useGlass, usePreload
src/components/                     Text, Button, Skeleton, Surface (glass or fallback), Icon, MediaCard, Poster,
                                    chips, Loading/Error/Empty views, PlayerControls, ComicPage
```

Screens use components and token names only: no hex colours, no raw font sizes, no `GlassView` directly. `grep -rnE "#[0-9a-fA-F]{3,8}|fontSize|rgba\(" src | grep -v src/theme/` should find nothing.

Files in `src/app/` are routes; anything else lives outside it.

## Plain HTTP in production builds

velora-server speaks `http://` on the LAN. Expo Go and development builds allow that, but a
production build blocks cleartext HTTP by default (App Transport Security on iOS, the cleartext
policy on Android), and the player and every API call fail. Before a release build, either put
the server behind HTTPS or allow cleartext for the server's address in `app.json`
(`ios.infoPlist.NSAppTransportSecurity`, Android `usesCleartextTraffic` via a config plugin).
Never edit `ios/` or `android/` by hand. Phase 16 decides which.

## Reaching the server from the phone

If the screen says "Server unreachable", check these in order:
1. Same Wi-Fi, no client isolation.
2. The server is bound to all interfaces (`*:8080` in `lsof`).
3. `EXPO_PUBLIC_API_URL` matches `ipconfig getifaddr en0` (the IP changes between networks).

## Curriculum

Phase docs, ADRs and the learning log live in the engineering archive under
`learning/projects/velora/`.
