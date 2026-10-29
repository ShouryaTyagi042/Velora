-- Where you are in a video or a comic: state, not events, so one row per (user, media),
-- upserted. user_id is here before users exist (phase 11): adding a column to a primary key
-- later is a table rewrite.
create table watch_progress (
  user_id     bigint      not null default 1,
  media_id    bigint      not null references media(id) on delete cascade,
  position_ms integer     not null check (position_ms >= 0),
  duration_ms integer     not null check (duration_ms > 0),
  completed   boolean     not null default false,  -- computed by the server, never sent by the client
  updated_at  timestamptz not null,                -- the client's clock: the newest save wins
  primary key (user_id, media_id)
);

create table reading_progress (
  user_id    bigint      not null default 1,
  media_id   bigint      not null references media(id) on delete cascade,
  page       integer     not null check (page >= 0),
  page_count integer     not null check (page_count > 0),
  completed  boolean     not null default false,
  updated_at timestamptz not null,
  primary key (user_id, media_id),
  check (page < page_count)
);

-- Continue Watching / Reading: a user's rows, newest first.
create index watch_progress_recent_idx   on watch_progress (user_id, updated_at desc);
create index reading_progress_recent_idx on reading_progress (user_id, updated_at desc);
