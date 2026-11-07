-- The Shorts feed's clips: a longer montage than the long-press preview (six 6 s cuts, up to 720p,
-- with sound), made by the extractor after the previews and stored as shorts/{id}.mp4 in the cache
-- dir. Same bookkeeping as the preview (0009): current when short_for equals the file's mod_time,
-- retried up to the extractor's limit.
alter table video_metadata
  add column short_for      timestamptz,
  add column short_attempts integer not null default 0,
  add column short_error    text;
