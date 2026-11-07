-- Three per-item features.
--
-- The bin: moving an item to the bin only marks it (in_bin, and when). Binned items disappear from
-- every list, row, count and search, but keep their files, progress, actors and tags, so restoring
-- one brings it back exactly as it was. Deleting from the bin is the only thing that removes files.
alter table media
  add column in_bin    boolean not null default false,
  add column binned_at timestamptz,
  add constraint media_bin_check check (in_bin = (binned_at is not null));

-- "What's in the bin, most recently binned first": small, so a partial index.
create index media_bin_idx on media (binned_at desc, id desc) where in_bin;

-- Favourites are each user's own, like progress.
create table favourites (
  user_id    bigint      not null references users (id) on delete cascade,
  media_id   bigint      not null references media (id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (user_id, media_id)
);
-- "My favourites, newest first".
create index favourites_user_recent_idx on favourites (user_id, created_at desc, media_id desc);

-- Tags are shared, like actors: a word or two that says what a video is ("vintage", "pov"), added
-- in the app. Stored lowercased and trimmed, so "POV" and "pov " are one tag.
create table tags (
  id   bigserial primary key,
  name text not null unique check (name <> '' and name = lower(btrim(name)))
);

create table media_tags (
  media_id bigint not null references media (id) on delete cascade,
  tag_id   bigint not null references tags (id) on delete cascade,
  primary key (media_id, tag_id)
);
-- "Videos with this tag": from the tag to its videos.
create index media_tags_tag_idx on media_tags (tag_id, media_id);
