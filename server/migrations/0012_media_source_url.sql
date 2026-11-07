-- Where an item came from: the web page it was downloaded from (a video's watch page, a comic's
-- chapter page). Not the file's own address: those are signed, short-lived links.
--
-- Filled by velora-admin import-sources from the downloader's records; the scanner never writes it,
-- so a rescan (which upserts only the columns it knows) keeps it. Null when unknown.
alter table media
  add column source_url text,
  add constraint media_source_url_check check (source_url is null or source_url ~ '^https?://');
