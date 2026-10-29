import { getJSON, sendJSON } from "./client";
import type { Media, Progress } from "./media";

// The whole current state, never a change: sending it twice is harmless (PUT).
export type ProgressBody =
  | { positionMs: number; durationMs: number } // video
  | { page: number; pageCount: number }; // comic

// putProgress saves where you are. clientTs is when it was true on this device: the server keeps
// the newest save, so a slow older request can't move you backwards.
export function putProgress(id: string, body: ProgressBody, clientTs: string): Promise<Progress> {
  return sendJSON<Progress>("PUT", `/api/media/${encodeURIComponent(id)}/progress`, { ...body, clientTs });
}

export type ContinueRows = { watching: Media[]; reading: Media[] };

export function getContinue({ signal }: { signal?: AbortSignal } = {}): Promise<ContinueRows> {
  return getJSON<ContinueRows>("/api/continue", { signal });
}
