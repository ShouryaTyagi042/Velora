-- Shared identity for every item in the library. Kind-specific shape lives in child tables.
create table media (
  id            bigint generated always as identity primary key,
  fingerprint   text        not null unique,  -- sha256(rel_path): the scanner's stable key
  kind          text        not null check (kind in ('video', 'comic')),
  title         text        not null,
  rel_path      text        not null unique,  -- a file for a video, a folder for a comic
  mime_type     text,                         -- null for comics: a folder has no MIME type
  size_bytes    bigint      not null,         -- comics: sum of page sizes
  mod_time      timestamptz not null,
  added_at      timestamptz not null default now(),
  last_seen_at  timestamptz not null default now(),
  missing_since timestamptz                   -- set when a scan no longer finds the file; never deleted
);
