import { getMedia, type Media, type Progress } from "@/api/media";
import { loadLocal } from "./store";

// loadWithProgress fetches an item and merges in this device's local progress when it's newer
// than the server's (a save that never made it: offline, or the app was killed mid-send).
export async function loadWithProgress(id: string, signal: AbortSignal): Promise<Media> {
  const [media, local] = await Promise.all([getMedia(id, { signal }), loadLocal(id)]);
  // Compare as instants, not strings: the server may answer in another time zone's notation.
  if (!local || (media.progress && Date.parse(media.progress.updatedAt) >= Date.parse(local.clientTs))) return media;
  const progress: Progress = { ...local.body, completed: false, updatedAt: local.clientTs };
  return { ...media, progress };
}

// FINISHED_AT mirrors the server's rule (progress.FinishedAt), only to decide where to resume
// from a local save the server hasn't seen yet. What counts as finished is still the server's call.
const FINISHED_AT = 0.95;

// RESUME_MIN_MS: a position this close to the start isn't worth resuming from.
const RESUME_MIN_MS = 5_000;

// resumeAt is where a video should start, in seconds: the saved position, or 0 for a finished
// film (resuming at 1:31:00 of a 1:31:00 film is wrong) or one barely started.
export function resumeAt(m: Media): number {
  const p = m.progress;
  if (!p || p.completed || !p.positionMs || p.positionMs < RESUME_MIN_MS) return 0;
  if (p.durationMs && p.positionMs >= FINISHED_AT * p.durationMs) return 0;
  return p.positionMs / 1000;
}

// resumePage is where a comic opens: the saved page, or 0 for a finished one.
export function resumePage(m: Media): number {
  const p = m.progress;
  const count = m.comic?.pageCount ?? 0;
  if (!p || p.completed || !p.page || p.page >= count - 1) return 0;
  return p.page;
}
