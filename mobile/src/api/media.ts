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

// What ffprobe found in a video. Absent until the server's extractor has run (seconds after a scan).
export type VideoMeta = {
  durationMs: number;
  width: number;
  height: number;
  codec: string;
  fps: number;
};

// Where you are in an item, as the server stored it. Absent fields are 0.
export type Progress = {
  positionMs?: number; // videos
  durationMs?: number;
  page?: number; // comics, 0-based
  pageCount?: number;
  completed: boolean; // decided by the server
  updatedAt: string; // the saving device's clock, RFC 3339
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
  video?: VideoMeta; // videos only, once extracted
  progress?: Progress; // once started
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

// pageUrl is page n (0-based) of a comic. v is the comic's modTime, which a rescan changes when
// any page does: the server marks versioned URLs immutable, so the phone never asks twice, and a
// changed page simply gets a new URL.
export function pageUrl(comic: Media, n: number): string {
  return `${baseUrl}/api/media/${encodeURIComponent(comic.id)}/pages/${n}?v=${encodeURIComponent(comic.modTime)}`;
}

// thumbnailUrl is the poster image, or null while a video's thumbnail is still being made (its
// metadata and thumbnail arrive together). A comic's is its first page, available at once.
// v changes when the file does, so a replaced file never shows a stale poster.
export function thumbnailUrl(m: Media): string | null {
  if (m.kind === MediaKind.Video && !m.video) return null;
  return `${baseUrl}/api/media/${encodeURIComponent(m.id)}/thumbnail?v=${encodeURIComponent(m.modTime)}`;
}

// streamUrl is what the native player fetches: GET /api/media/{id}/stream with Range headers.
// The player opens its own connections, so this is a URL, not a getJSON call.
export function streamUrl(id: string): string {
  return `${baseUrl}/api/media/${encodeURIComponent(id)}/stream`;
}
