-- DESTRUCTIVE: empties media, actors, users and sessions first. Run it only against
-- velora_test (VELORA_TEST_DSN), never the real database:
--   psql "$VELORA_TEST_DSN" -f bench/seed.sql
-- Then start a server on velora_test with an empty media folder, and after its startup scan run
--   update media set missing_since = null where rel_path like 'fake/%';
-- (the scan marks every row it did not find on disk as missing).
--
-- 100k fake media in velora_test, shaped like a big real library:
-- 90k videos (metadata already extracted, so the extractor leaves them alone), 10k comics,
-- 500 folder actors, progress on a few thousand items for user 1. Added over two years.
begin;
truncate media, actors, users, sessions restart identity cascade;

insert into users (username, password_hash) values ('bench', 'x');  -- id 1; no one can sign in with 'x'

insert into media (fingerprint, kind, title, rel_path, mime_type, size_bytes, mod_time, added_at, last_seen_at)
select md5('m' || g),
       case when g % 10 = 0 then 'comic' else 'video' end,
       initcap(md5(g::text)),                                   -- random-looking titles
       'fake/' || g,
       case when g % 10 = 0 then null else 'video/mp4' end,
       (random() * 2e9)::bigint,
       now() - (random() * interval '730 days'),
       now() - (random() * interval '730 days'),
       now()
from generate_series(1, 100000) g;

insert into video_metadata (media_id, duration_ms, width, height, codec, fps, extracted_at, attempts)
select id, 1000 * (600 + (random() * 7200)::int), 1920, 1080, 'h264', 24, now(), 1
from media where kind = 'video';

insert into comic_metadata (media_id, page_count, pages)
select id, 20, array(select 'page-' || p || '.jpg' from generate_series(1, 20) p)
from media where kind = 'comic';

insert into actors (name) select 'Actor ' || g from generate_series(1, 500) g;
insert into media_actors (media_id, actor_id, source)
select id, 1 + (id % 500), 'folder' from media where kind = 'video';

insert into watch_progress (user_id, media_id, position_ms, duration_ms, completed, updated_at)
select 1, id, 60000, 3600000, id % 3 = 0, now() - (random() * interval '365 days')
from media where kind = 'video' and id % 30 = 1;           -- 3,000 videos started

insert into reading_progress (user_id, media_id, page, page_count, completed, updated_at)
select 1, id, 3, 20, false, now() - (random() * interval '365 days')
from media where kind = 'comic' and id % 20 = 0;           -- 500 comics started
commit;
analyze;
