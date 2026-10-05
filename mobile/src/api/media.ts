import { getJSON } from "./client";

// Mirrors the Go media.Media JSON. Path is never sent by the server.
// Values must match media.Kind constants in server/internal/media/media.go.
export enum MediaKind {
  Video = "video",
  Comic = "comic",
}

export type Media = {
  id: string;
  title: string;
  kind: MediaKind;
  sizeBytes: number;
  addedAt: string; // RFC 3339 timestamp
};

export async function listMedia(kind?: MediaKind): Promise<Media[]> {
  const query = kind ? `?kind=${kind}` : "";
  const body = await getJSON<{ items: Media[] }>(`/api/media${query}`);
  return body.items;
}
