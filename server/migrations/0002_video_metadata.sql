-- Filled by phase 9's extractor (ffprobe); empty until then.
create table video_metadata (
  media_id     bigint primary key references media(id) on delete cascade,
  duration_ms  integer,
  width        integer,
  height       integer,
  codec        text,
  fps          numeric(6, 3),
  extracted_at timestamptz
);
