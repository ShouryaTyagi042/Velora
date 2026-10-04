# Velora

A personal media server (videos and comics on a local disk, streamed to a phone), built as a learning project.

| Folder | What |
|---|---|
| [`server/`](./server) | Go backend: standard library only (`net/http`) |
| [`mobile/`](./mobile) | React Native app (Expo managed workflow, expo-router) |

The curriculum assumes two repos (`velora-server`, `velora-mobile`). Here they are two folders in one repo; nothing else changes.

## Quick start

```sh
cd server && make run                       # terminal 1
cd mobile && npm install && npx expo start  # terminal 2, after setting mobile/.env
```

## Status

Phase 0: project setup. The phone calls `GET /health` on the Mac over the LAN.
