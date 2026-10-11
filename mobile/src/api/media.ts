import { getJSON } from "./client";

// Mirrors the Go media.Media JSON (server/internal/media/media.go), kept in sync by hand.
// The server never sends filesystem paths or comic page filenames.
// Enum values must match the Go constants exactly.
export enum MediaKind {
  Video = "video",
  Comic = "comic",
}

export enum ActorSource {
  Folder = "folder", // from movies/<actor>/; changes only by renaming the folder
  Manual = "manual", // added in the app (phase 7)
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

// parseKind turns a route param back into a MediaKind; anything else means "all".
export function parseKind(value: string | undefined): MediaKind | undefined {
  return Object.values(MediaKind).find((k) => k === value);
}
