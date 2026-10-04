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

export function listMedia(): Promise<Media[]> {
  return getJSON<Media[]>("/api/media");
}
