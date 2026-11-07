import { baseUrl, getJSON, sendJSON } from "./client";

// Mirrors the Go media.Media JSON (server/internal/media/media.go), kept in sync by hand.
// The server never sends filesystem paths or comic page filenames.
// Enum values must match the Go constants exactly.
export enum MediaKind {
  Video = "video",
  Comic = "comic",
}

// Section is the shelf a video sits on: the top-level folder it is in on the server (movies/ or
// anime/). Anime is still a video; only the tab differs. Comics have none.
export enum Section {
  Movies = "movies",
  Anime = "anime",
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
  preview?: boolean; // a long-press preview of the file as it is now exists (phase 14)
  short?: boolean; // the Shorts feed's clip (six 6 s cuts, up to 720p, with sound) exists
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
  section?: Section; // videos only
  series?: string; // anime in a series folder only (anime/<series>/…); a show, not an actor
  mimeType?: string; // videos only; a comic is a folder
  sizeBytes: number;
  modTime: string; // RFC 3339 timestamp
  addedAt: string; // RFC 3339 timestamp
  actors?: ActorTag[]; // videos only; absent when there are none
  comic?: { pageCount: number }; // comics only
  video?: VideoMeta; // videos only, once extracted
  progress?: Progress; // once started
  tags?: string[]; // videos only, lowercase; absent when there are none
  favourite?: boolean; // the signed-in user's favourite
  views: number; // times watched (videos) or read to the last page (comics), by anyone
  inBin?: boolean; // moved to the bin: hidden everywhere but the bin until restored or deleted
  binnedAt?: string; // RFC 3339, while in the bin
  sourceUrl?: string; // the web page it was downloaded from, when known
};

export type ListOptions = {
  kind?: MediaKind;
  section?: Section; // videos in this section only
  series?: string; // episodes of this anime series only (exact name)
  actor?: string;
  clip?: "preview" | "short"; // only videos whose clip of this kind is ready
  tag?: string; // only videos with this tag
  favourites?: boolean; // only the signed-in user's favourites
  bin?: boolean; // only what's in the bin (everything else leaves it out)
  q?: string; // search titles and actors, ignoring case and accents
  // title (A to Z, natural: 2 before 10) by default. random is a new shuffle every call, with no
  // next page: ask again for more (Surprise me, Shorts).
  sort?: "title" | "title_desc" | "recent" | "random" | "binned"; // binned: the bin's order, with bin
  limit?: number; // the server's default is 50, its maximum 200
  after?: string; // the previous page's nextCursor
  signal?: AbortSignal;
};

// Page is one page of a list. nextCursor is null on the last page; pass it as `after` for the next.
export type Page<T> = { items: T[]; nextCursor: string | null };

// listMedia is one page of the library. The server never sends the whole thing: at 100k items that
// was 33 MB for a screen showing a dozen cards (phase 12).
export function listMedia(options: ListOptions = {}): Promise<Page<Media>> {
  const { kind, section, series, actor, clip, tag, favourites, bin, q, sort, limit, after, signal } = options;
  const params = new URLSearchParams();
  if (kind) params.set("kind", kind);
  if (section) params.set("section", section);
  if (series) params.set("series", series);
  if (actor) params.set("actor", actor);
  if (clip) params.set("clip", clip);
  if (tag) params.set("tag", tag);
  if (favourites) params.set("favourites", "true");
  if (bin) params.set("bin", "true");
  if (q) params.set("q", q);
  if (sort) params.set("sort", sort);
  if (limit) params.set("limit", String(limit));
  if (after) params.set("after", after);
  const qs = params.toString(); // encodes spaces as "+", which Go's Query() decodes back
  return getJSON<Page<Media>>(`/api/media${qs ? `?${qs}` : ""}`, { signal });
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

// Cover is the video whose thumbnail stands for an actor or a series: an actor's newest video, a
// series' first episode. Absent while none of them has a thumbnail yet.
export type Cover = { id: string; modTime: string };

// The bin. Moving to the bin only marks an item: it leaves every list but the bin, keeping its
// files, progress, tags and actors. Restoring brings it back as it was. Deleting (only from the
// bin) removes its files from the server for good.
export function moveToBin(id: string): Promise<Media> {
  return sendJSON<Media>("POST", `/api/media/${encodeURIComponent(id)}/bin`);
}

export function restoreFromBin(id: string): Promise<Media> {
  return sendJSON<Media>("DELETE", `/api/media/${encodeURIComponent(id)}/bin`);
}

export function deleteForever(id: string): Promise<void> {
  return sendJSON<void>("DELETE", `/api/media/${encodeURIComponent(id)}`);
}

// emptyBin deletes everything in the bin. failed are items whose files the server couldn't remove:
// they stay in the bin.
// With kind or section, only what's of that kind or section is deleted.
export function emptyBin({ kind, section }: { kind?: MediaKind; section?: Section } = {}): Promise<{ deleted: number; failed: number }> {
  const params = new URLSearchParams();
  if (kind) params.set("kind", kind);
  if (section) params.set("section", section);
  const qs = params.toString();
  return sendJSON("DELETE", `/api/bin${qs ? `?${qs}` : ""}`);
}

// setFavourite adds an item to the signed-in user's favourites, or removes it; the answer is the item.
export function setFavourite(id: string, on: boolean): Promise<Media> {
  return sendJSON<Media>(on ? "PUT" : "DELETE", `/api/media/${encodeURIComponent(id)}/favourite`);
}

// addTag tags a video (the server lowercases and trims); removeTag untags it. Both answer with the video.
export function addTag(id: string, name: string): Promise<Media> {
  return sendJSON<Media>("POST", `/api/media/${encodeURIComponent(id)}/tags`, { name });
}

export function removeTag(id: string, name: string): Promise<Media> {
  return sendJSON<Media>("DELETE", `/api/media/${encodeURIComponent(id)}/tags/${encodeURIComponent(name)}`);
}

// recordView counts a view: the player calls it past 30 s of watching (or at the end of a shorter
// video), the reader on reaching the last page. The server treats the same person's repeat within
// half an hour as the same view (counted: false).
export function recordView(id: string): Promise<{ views: number; counted: boolean }> {
  return sendJSON("POST", `/api/media/${encodeURIComponent(id)}/views`);
}

export type TagCount = { name: string; videoCount: number };

// listTags is every tag in use; with section, counting only that shelf's videos.
export async function listTags({ section, signal }: { section?: Section; signal?: AbortSignal } = {}): Promise<TagCount[]> {
  return (await getJSON<{ items: TagCount[] }>(`/api/tags${section ? `?section=${section}` : ""}`, { signal })).items;
}

export type ActorCount = { name: string; videoCount: number; cover?: Cover };

// listActors is every actor with at least one available video (in section, if given), by name.
export async function listActors({ section, signal }: { section?: Section; signal?: AbortSignal } = {}): Promise<
  ActorCount[]
> {
  const body = await getJSON<{ items: ActorCount[] }>(`/api/actors${section ? `?section=${section}` : ""}`, { signal });
  return body.items;
}

export type SeriesCount = { name: string; episodeCount: number; cover?: Cover };

// listSeries is every anime series with at least one available episode, in natural order.
export async function listSeries({ signal }: { signal?: AbortSignal } = {}): Promise<SeriesCount[]> {
  const body = await getJSON<{ items: SeriesCount[] }>("/api/series", { signal });
  return body.items;
}

// Job is background work on the server (server/internal/jobs): a scan, or a round of extraction.
export type Job = {
  id: string;
  name: string;
  status: "running" | "done" | "failed" | "canceled";
  progress?: { done: number; total: number };
  result?: ScanResult; // a finished scan's
  error?: string;
  finishedAt?: string;
};

// ScanResult is what a finished scan found: every item on disk, and how many it newly marked
// missing (their files are gone).
export type ScanResult = { found: number; missing: number; warnings: string[] };

// scanLibrary asks the server to rescan its folder (new, renamed and removed files). It answers
// at once with the scan's job, before the scan is done: getJob says when it is. If a scan is
// already running, that is the job you get.
export function scanLibrary(): Promise<Job> {
  return sendJSON<Job>("POST", "/api/scan");
}

// getJob is a job's current status. A 404 means the server has restarted since and forgotten it.
export function getJob(id: string, { signal }: { signal?: AbortSignal } = {}): Promise<Job> {
  return getJSON<Job>(`/api/jobs/${encodeURIComponent(id)}`, { signal });
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

// coverUrl is the thumbnail of an actor's or a series' cover video.
export function coverUrl(c: Cover): string {
  return `${baseUrl}/api/media/${encodeURIComponent(c.id)}/thumbnail?v=${encodeURIComponent(c.modTime)}`;
}

// randomMedia is up to n items picked at random (of kind, if given): a fresh shuffle every call.
export async function randomMedia(
  n: number,
  { kind, section, clip, signal }: { kind?: MediaKind; section?: Section; clip?: "preview" | "short"; signal?: AbortSignal } = {},
): Promise<Media[]> {
  return (await listMedia({ kind, section, clip, sort: "random", limit: n, signal })).items;
}

// SHORT_FORMAT changes whenever the server's Shorts clip changes shape (2: eight 8 s cuts). The
// phone caches clips forever by URL, so a new format must be a new URL, even for the same file.
const SHORT_FORMAT = 2;

// shortUrl is a video's Shorts clip: only for a video with video.short. Versioned like previews,
// and by the clip format.
export function shortUrl(m: Media): string {
  return `${baseUrl}/api/media/${encodeURIComponent(m.id)}/short?v=${encodeURIComponent(m.modTime)}&f=${SHORT_FORMAT}`;
}

// previewUrl is a video's long-press preview: a short silent montage the server cut from it. Only
// for a video with video.preview; anything else is a 404. Versioned by modTime like thumbnails, so
// the phone keeps it and a replaced file gets a new URL.
export function previewUrl(m: Media): string {
  return `${baseUrl}/api/media/${encodeURIComponent(m.id)}/preview?v=${encodeURIComponent(m.modTime)}`;
}

// streamUrl is what the native player fetches: GET /api/media/{id}/stream with Range headers.
// The player opens its own connections, so this is a URL, not a getJSON call.
export function streamUrl(id: string): string {
  return `${baseUrl}/api/media/${encodeURIComponent(id)}/stream`;
}
