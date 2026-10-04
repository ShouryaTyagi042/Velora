# velora-mobile

The Expo / React Native client for Velora. In Phase 0 it has one screen that calls the server's `GET /health` and shows the result.

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
src/app/_layout.tsx   root Stack navigator (expo-router)
src/app/index.tsx     "/" (the health-check screen)
src/api/client.ts     getJSON(): base URL + fetch + status check
```

Files in `src/app/` are routes; anything else lives outside it.

## Reaching the server from the phone

If the screen says "Server unreachable", check these in order:
1. Same Wi-Fi, no client isolation.
2. The server is bound to all interfaces (`*:8080` in `lsof`).
3. `EXPO_PUBLIC_API_URL` matches `ipconfig getifaddr en0` (the IP changes between networks).

## Curriculum

Phase docs, ADRs and the learning log live in the engineering archive under
`learning/projects/velora/`.
