create table actors (
  id   bigint generated always as identity primary key,
  name text not null unique
);

-- One row per (video, actor). source says which writer owns the row: the scanner re-syncs
-- only 'folder' rows, the API adds and removes only 'manual' ones.
create table media_actors (
  media_id bigint not null references media(id) on delete cascade,
  actor_id bigint not null references actors(id),
  source   text   not null check (source in ('folder', 'manual')),
  primary key (media_id, actor_id)
);

-- "Videos by this actor": the primary key leads with media_id, so it can't answer that.
create index media_actors_actor_id_idx on media_actors (actor_id);
