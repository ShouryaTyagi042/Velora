-- Views: one row each time someone watches a video in the player (past its first 30 s, or to the
-- end of a shorter one) or reads a comic to its last page. Shared: a video's count is everyone's
-- views. A row per view, not a counter, so a view can be told apart from a retry: the same person
-- viewing the same item again within half an hour is the same view (see RecordView).
create table media_views (
  id        bigserial   primary key,
  media_id  bigint      not null references media (id) on delete cascade,
  user_id   bigint      not null references users (id) on delete cascade,
  viewed_at timestamptz not null default now()
);
-- Counting an item's views (every list row), and the half-hour check.
create index media_views_media_idx on media_views (media_id);
create index media_views_user_media_idx on media_views (user_id, media_id, viewed_at desc);
