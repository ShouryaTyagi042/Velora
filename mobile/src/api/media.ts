import { baseUrl, getJSON, sendJSON } from "./client";

// Mirrors the Go media.Media JSON (server/internal/media/media.go), kept in sync by hand.
// The server never sends filesystem paths or comic page filenames.
// Enum values must match the Go constants exactly.
export enum MediaKind {
  Video = "video",
  Comic = "comic",
}

export enum ActorSource {
  Folder = "folder", // from movies/<actor>/; changes only by renaming the folder
  Manual = "manual", // added in the app; removable there, and kept by rescans
}

export type ActorTag = {
  name: string;
  source: ActorSource;
};

export type Media = {
  id: string;
  title: string;
  kind: MediaKind;
  mimeType?: string; // videos only; a comic is a folder
  sizeBytes: number;
  modTime: string; // RFC 3339 timestamp
  addedAt: string; // RFC 3339 timestamp
  actors?: ActorTag[]; // videos only; absent when there are none
  comic?: { pageCount: number }; // comics only
};

export type ListOptions = { kind?: MediaKind; actor?: string; signal?: AbortSignal };

export async function listMedia({ kind, actor, signal }: ListOptions = {}): Promise<Media[]> {
  const params = new URLSearchParams();
  if (kind) params.set("kind", kind);
  if (actor) params.set("actor", actor);
  const qs = params.toString(); // encodes spaces as "+", which Go's Query() decodes back
  const body = await getJSON<{ items: Media[] }>(`/api/media${qs ? `?${qs}` : ""}`, { signal });
  return body.items;
}

export function getMedia(id: string, { signal }: { signal?: AbortSignal } = {}): Promise<Media> {
  return getJSON<Media>(`/api/media/${encodeURIComponent(id)}`, { signal });
}

// addActor links a manual actor to a video. The answer is the updated video, so the screen can
// replace its copy without refetching. Adding an actor the video already has is a no-op.
export function addActor(id: string, name: string): Promise<Media> {
  return sendJSON<Media>("POST", `/api/media/${encodeURIComponent(id)}/actors`, { name });
}

// removeActor unlinks a manual actor. Folder actors answer 409 conflict: rename the folder instead.
// encodeURIComponent keeps "AC/DC" one path segment.
export function removeActor(id: string, name: string): Promise<void> {
  return sendJSON<void>("DELETE", `/api/media/${encodeURIComponent(id)}/actors/${encodeURIComponent(name)}`);
}

// parseKind turns a route param back into a MediaKind; anything else means "all".
export function parseKind(value: string | undefined): MediaKind | undefined {
  return Object.values(MediaKind).find((k) => k === value);
}

// streamUrl is what the native player fetches: GET /api/media/{id}/stream with Range headers.
// The player opens its own connections, so this is a URL, not a getJSON call.
export function streamUrl(id: string): string {
  return `${baseUrl}/api/media/${encodeURIComponent(id)}/stream`;
}
