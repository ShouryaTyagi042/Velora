-- An anime episode's series, as a column. Until now it was read off rel_path on every query
-- (split_part(rel_path, '/', 2)), which no index could serve. With the anime library heading for
-- ~30 GB (hundreds to a few thousand episodes), the Anime tab's per-series lists and the series
-- chips should be index work, not a computation per row.
--
-- The rule is still media.SeriesOf (the folder right under anime/): the scanner's upsert writes it,
-- this backfills the rows already there. Null for everything else: movies, comics, and episodes
-- directly in anime/. A series is never an empty string, and only videos have one.
alter table media
  add column series text,
  add constraint media_series_check check (series is null or (kind = 'video' and series <> ''));

update media set series = split_part(rel_path, '/', 2)
where kind = 'video' and rel_path like 'anime/%/%';

-- "Episodes of this series, by title" (GET /api/media?series=…): seek to the series, read rows
-- already in title order (the same natural key as media_kind_title_idx), stop after LIMIT. The
-- leading series column also serves GET /api/series, which groups by it. Partial: only rows that
-- have a series, and only available ones, as every reader filters missing items out.
create index media_series_title_idx on media (series, (velora_title_key(title) collate "C"), id)
  where series is not null and missing_since is null;
