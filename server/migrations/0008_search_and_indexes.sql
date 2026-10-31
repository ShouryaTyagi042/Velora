-- Phase 12: search on the server, and the indexes the paged list needs. Every index here was added
-- because EXPLAIN ANALYZE on a 100k-row library showed a full scan and sort without it (the
-- before/after numbers are in ADR 009, "No Redis (yet)").

-- Search ignores case and accents: "amelie" finds "Amélie". unaccent is a trusted extension, so
-- the database owner can create it without a superuser.
create extension if not exists unaccent;

-- unaccent(text) is only "stable" (its dictionary could change), so it can't be used in an index.
-- Naming the dictionary explicitly makes the result fixed for a given input, which lets us
-- declare velora_fold immutable. internal/media/Fold is the Go twin used by the test double.
create function velora_fold(text) returns text
  language sql immutable parallel safe strict
  return lower(public.unaccent('public.unaccent'::regdictionary, $1));

-- The Videos and Comics screens page by title: where kind = $1 order by lower(title), id. The
-- filter column comes first, then the sort columns in sort order, so Postgres seeks to the kind
-- (and to the cursor) and reads rows already in order: no sort, and it stops after LIMIT.
-- Partial (missing_since is null) because no reader ever asks for missing items.
create index media_kind_title_idx on media (kind, lower(title), id) where missing_since is null;

-- Home's "New videos" / "New comics": where kind = $1 order by added_at desc, id desc limit 10.
create index media_kind_added_idx on media (kind, added_at desc, id desc) where missing_since is null;
