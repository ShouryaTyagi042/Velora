# velora-mobile

The Expo / React Native client for Velora: browse the library (Home, Library, Details) and play videos from velora-server.

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
src/app/_layout.tsx       root Stack navigator (expo-router), light/dark theme
src/app/index.tsx         "/": recently added, link to the library
src/app/library.tsx       grid with kind and actor filters (in the URL)
src/app/media/[id].tsx    details; Play opens the player
src/app/player/[id].tsx   expo-video player with native controls
src/api/                  getJSON, ApiError, Media types, streamUrl
src/hooks/                useAsync (fetch state), usePlayerStatus (player events)
src/components/           MediaCard, chips, Loading/Error/Empty views
```

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
