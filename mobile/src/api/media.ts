import { getJSON } from "./client";

// Mirrors the Go media.Media JSON (server/internal/media/media.go).
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

export async function listMedia(filter: { kind?: MediaKind; actor?: string } = {}): Promise<Media[]> {
  const params = new URLSearchParams();
  if (filter.kind) params.set("kind", filter.kind);
  if (filter.actor) params.set("actor", filter.actor);
  const qs = params.toString(); // encodes spaces as "+", which Go's Query() decodes back
  const query = qs ? `?${qs}` : "";
  const body = await getJSON<{ items: Media[] }>(`/api/media${query}`);
  return body.items;
}
