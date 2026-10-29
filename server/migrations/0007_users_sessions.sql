-- Identity. No signup endpoint: users are created with velora-admin on the server itself.
create table users (
  id            bigint generated always as identity primary key,
  username      text        not null unique,
  password_hash text        not null,  -- bcrypt; the password itself is never stored
  created_at    timestamptz not null default now()
);

-- One row per signed-in device. The token is 256 random bits sent as a Bearer header; only its
-- sha256 is stored, so a leaked table (a backup, a dump) doesn't hand out live sessions.
create table sessions (
  token_hash bytea       primary key,
  user_id    bigint      not null references users(id) on delete cascade,
  expires_at timestamptz not null,  -- sliding: pushed forward on use, at most once a day
  created_at timestamptz not null default now()
);
create index sessions_user_idx on sessions (user_id);

-- Progress now belongs to a real user: no more default. The foreign keys are "not valid" so the
-- rows saved before auth (all user_id 1) are kept without checking them; new rows are checked.
-- The first user velora-admin creates gets id 1, and with it that history.
alter table watch_progress   alter column user_id drop default;
alter table reading_progress alter column user_id drop default;
alter table watch_progress
  add constraint watch_progress_user_fk foreign key (user_id) references users(id) on delete cascade not valid;
alter table reading_progress
  add constraint reading_progress_user_fk foreign key (user_id) references users(id) on delete cascade not valid;
