-- Shorts clips grew from six 6 s cuts to eight 8 s cuts. A clip is current when short_for matches
-- the file's mod_time, which a new clip length doesn't change, so mark every clip stale: the
-- extractor makes them again (the old file keeps nothing alive: until the new one is in, the
-- database says there is no clip, and the app plays the long-press preview instead).
update video_metadata set short_for = null, short_attempts = 0, short_error = null;
