# velora-mobile

The Expo / React Native client for Velora: sign in, browse and search the library, play videos, read comics, and pick up where you left off. The design is a streaming service's, in Velora's own colours: one dark theme (a blue-black ink, white Play buttons, Velora blue `#1f6bff` for progress, selection and the brand), artwork edge to edge, a billboard and rails on Home, and one full-width card per row wherever videos or anime are listed. It is tuned for iOS: Liquid Glass on the controls layer (native headers and tab bar, the player's and reader's controls), with solid fallbacks on Android, web, older iOS and with Reduce Transparency on.

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
| `EXPO_PUBLIC_API_URL` | `http://192.168.1.10:8080` | Base URL of velora-server, as seen **from the phone**. Never `localhost`: on the phone, that means the phone. |

`EXPO_PUBLIC_*` values are inlined into the JS bundle at build time and shipped to the device. Never put a secret behind that prefix. After editing `.env`, restart `npx expo start`.

## Layout

```text
src/app/_layout.tsx                 root Stack, guarded by the session: the tabs, Details, player, reader when
                                    signed in; login when not (Stack.Protected)
src/app/login.tsx                   username and password; no signup (users are made with velora-admin)
src/app/(tabs)/_layout.tsx          native tabs: Home, Shorts, Actors, Anime, Search (five: a sixth would
                                    hide behind "More" on iPhone, so Comics is reached from Home)
src/app/(tabs)/(home)/index.tsx     "/": billboard (a random video; its preview plays after a moment), pills
                                    (Actors, Anime, Comics, and Surprise me in the brand gradient), rails:
                                    Continue watching, new films, actors, new anime, series, Continue reading,
                                    comics. Pull down to refresh: every rail, and a new billboard
src/app/(tabs)/(home)/comics.tsx    "/comics": comic covers, 3 across, A–Z / Z–A; pull to rescan
src/app/(tabs)/(shorts)/shorts.tsx  "/shorts": vertical feed of server-cut clips (the Shorts clip, else the
                                    long-press preview) and comics as stories (10 pages from past 60%), Mix /
                                    Videos / Anime / Comics; the page on screen and its
                                    neighbours each own a preloaded player, shown only once it has drawn its
                                    own first frame; tap to pause, sound toggle, Watch / Details
src/app/(tabs)/(actors)/            "/actors": one banner per actor (their newest still); "/actor/[name]":
                                    their videos, one per row, Shuffle play; "/videos": every film
src/app/(tabs)/(anime)/             "/anime": one banner per series; "/series/[name]": Play / Resume and the
                                    episode list; "/episodes": all anime
src/app/(tabs)/(search)/search.tsx  server-side search, one result per row; recent searches when the field is
                                    empty (src/search/history: on this device, cleared on sign-out)
src/app/surprise.tsx                "/surprise?kind=any|film|anime|comic": a sheet with one random pick
src/app/media/[id].tsx              video details: still edge to edge, Play / Resume, nearby episodes of its
                                    series, cast (add and remove manual ones), more with its first actor;
                                    a comic's card opens the reader directly (navigation/openMedia)
src/app/player/[id].tsx             expo-video with glass controls (native controls in fullscreen); resumes
src/app/reader/[id].tsx             paged comic reader: pinch-zoom, preload, direction setting; resumes
src/theme/                          design tokens (one dark palette, gradients, space, radius, type roles,
                                    wordmark, glass roles)
src/api/                            getJSON/sendJSON (with the Bearer token), ApiError, Media types, URLs for
                                    streams, pages, thumbnails; authHeaders for the player and images
src/auth/                           AuthProvider (session state, 401 → login), token in SecureStore
src/preview/                        long-press previews: one shared muted player (PreviewProvider, around the
                                    tabs) for videos, a page flip for comics (ComicPreview: pages from past
                                    60% of it, lib/pages), which card is previewing (a tiny external store)
src/progress/                       local-first progress saves (throttled, one in flight) and launch reconcile
src/hooks/                          useAsync, usePlayerStatus, useGlass, usePreload
src/components/                     Text, Button (play / primary / secondary / quiet), Skeleton, Surface, Icon,
                                    Artwork, Poster, MediaCard (rail or wide), CoverCard, EpisodeRow, Rail,
                                    GroupList, VideoList, chips, Loading/Error/Empty views,
                                    PlayerControls, ComicPage
```

Screens use components and token names only: no hex colours, no raw font sizes, no `GlassView` directly. `grep -rnE "#[0-9a-fA-F]{3,8}|fontSize|rgba\(" src | grep -v src/theme/` should find nothing.

Files in `src/app/` are routes; anything else lives outside it.

## Signing in

Create a user on the server first (`make user NAME=…` in `../server`). The app keeps the session token in the Keychain / Keystore (`expo-secure-store`), never in AsyncStorage, and sends it on every request: API calls, the video stream and every image. A `401` anywhere (the session expired after 30 idle days, or was logged out) clears it and shows the login screen. On Expo web there's no secure store, so the token is kept in memory and a reload signs you out.

Signing out pushes any unsynced progress, ends the session on the server, and forgets this phone's local progress, so the next person to sign in doesn't inherit it.

## Plain HTTP in production builds

velora-server speaks `http://` on the LAN. Expo Go and development builds allow that, but a
production build blocks cleartext HTTP by default (App Transport Security on iOS, the cleartext
policy on Android), and the player and every API call fail. Before a release build, either put
the server behind HTTPS or allow cleartext for the server's address in `app.json`
(`ios.infoPlist.NSAppTransportSecurity`, Android `usesCleartextTraffic` via a config plugin).
Never edit `ios/` or `android/` by hand. Phase 16 decides which.

Over plain HTTP the password (at sign-in) and the session token (on every request) cross the Wi-Fi
unencrypted: anyone on the same network can read them. Acceptable on your own network for now;
HTTPS in phase 16 is what fixes it.

## Reaching the server from the phone

If the screen says "Server unreachable", check these in order:
1. Same Wi-Fi, no client isolation.
2. The server is bound to all interfaces (`*:8080` in `lsof`).
3. `EXPO_PUBLIC_API_URL` matches `ipconfig getifaddr en0` (the IP changes between networks).

## Curriculum

Phase docs, ADRs and the learning log live in the engineering archive under
`learning/projects/velora/`.
