-- Titles sort naturally: "Episode 2" before "Episode 10" (plain text order puts "10" first,
-- because "1" < "2"). 0010, not 0009: 0009 is phase 14's preview migration.
--
-- velora_title_key is the sort key: the title in lower case with every run of digits zero-padded
-- to 20 places ("episode 00000000000000000002"), so plain byte order on the key is natural order.
-- Leading zeros are dropped first, so "Episode 01" and "Episode 1" tie (the id breaks it). A run
-- longer than 20 digits keeps its length and still sorts after every shorter number.
--
-- Not an ICU numeric collation: that would also reorder accents and punctuation, and the Go twin
-- (media.TitleKey, used by the in-memory test double) couldn't match it. This one it matches
-- exactly: lower() under "C" changes ASCII letters only, and so does TitleKey. Everything that
-- sorts by the key says collate "C", so the order is byte order whatever the database's default.
create function velora_title_key(text) returns text
  language sql immutable parallel safe strict
  return (
    select coalesce(string_agg(coalesce(lpad(n, greatest(20, length(n)), '0'), m[2]), '' order by i), '')
    from regexp_matches(lower($1 collate "C"), '([0-9]+)|([^0-9]+)', 'g') with ordinality as r(m, i),
         ltrim(m[1], '0') as n
  );

-- The Videos, Anime and Comics screens page by title: the index follows the new order.
drop index media_kind_title_idx;
create index media_kind_title_idx on media (kind, (velora_title_key(title) collate "C"), id)
  where missing_since is null;
