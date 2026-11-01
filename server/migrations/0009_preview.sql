-- Phase 14: long-press previews. A preview is a short silent montage the extractor cuts from the
-- video once its metadata is in (ffmpeg.Preview), stored as previews/{id}.mp4 in the cache dir.
--
-- preview_for is the media.mod_time the preview was made from. The preview is current when it
-- equals the file's mod_time: a replaced file changes mod_time, so its preview goes stale (not
-- served, made again) without anything having to notice and delete it. Null: never made.
-- preview_attempts and preview_error bound the retries like attempts/last_error (0005) do.
alter table video_metadata
  add column preview_for      timestamptz,
  add column preview_attempts integer not null default 0,
  add column preview_error    text;
