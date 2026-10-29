-- The extractor's job state. A video is pending while it has no row, or a row that was never
-- extracted and hasn't failed too often: a corrupt file is retried a few times, not forever.
alter table video_metadata
  add column attempts   integer not null default 0,
  add column last_error text;
