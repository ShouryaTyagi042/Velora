-- Known at scan time: the scanner counts and orders the pages while it walks.
create table comic_metadata (
  media_id   bigint primary key references media(id) on delete cascade,
  page_count integer not null check (page_count > 0),
  pages      text[]  not null,  -- page filenames in reading order; phase 8 serves pages[n]
  check (cardinality(pages) = page_count)
);
