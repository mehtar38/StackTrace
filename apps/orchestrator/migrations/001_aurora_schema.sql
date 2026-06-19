-- ============================================================
-- StackTrace — Aurora PostgreSQL Schema
-- Differences from the old Supabase schema:
--   - No RLS policies. Supabase's RLS model assumes Supabase's own
--     auth.uid() function, which doesn't exist on plain Aurora.
--     The orchestrator is now the sole gatekeeper — it already checks
--     state.UserID != claims.Sub on every request, so authorization
--     happens in Go, not in the database. This is the standard pattern
--     for any app using a direct Postgres connection instead of Supabase's
--     auto-generated REST+RLS layer.
--   - Connects via stacktrace_app, an IAM-auth user with no password.
-- ============================================================

create extension if not exists "pgcrypto";

-- ── users ─────────────────────────────────────────────────────────────────────
create table public.users (
  id          text        primary key,
  email       text        not null unique,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now()
);

-- ── sessions ──────────────────────────────────────────────────────────────────
create type session_status as enum (
  'prewarming',
  'active',
  'exited',
  'completed',
  'expired',
  'error'
);

create table public.sessions (
  id                  uuid            primary key default gen_random_uuid(),
  user_id             text            references public.users(id) on delete cascade,
  challenge_id        text            not null,
  status              session_status  not null default 'prewarming',
  anon_token          text,
  container_id        text,           -- ECS task ARN (was Docker container ID)
  container_host      text,           -- ECS task private IP / connection info
  started_at          timestamptz,
  last_active_at      timestamptz,
  ended_at            timestamptz,
  duration_seconds    integer,
  created_at          timestamptz     not null default now()
);

create unique index sessions_anon_token_active_idx
  on public.sessions(anon_token)
  where status in ('prewarming', 'active') and anon_token is not null;

create index sessions_user_id_idx      on public.sessions(user_id);
create index sessions_challenge_id_idx on public.sessions(challenge_id);
create index sessions_status_idx       on public.sessions(status);

-- ── session_file_diffs ────────────────────────────────────────────────────────
create table public.session_file_diffs (
  id          uuid        primary key default gen_random_uuid(),
  session_id  uuid        not null references public.sessions(id) on delete cascade,
  file_path   text        not null,
  content     text        not null,
  saved_at    timestamptz not null default now(),

  unique (session_id, file_path)
);

create index session_file_diffs_session_id_idx on public.session_file_diffs(session_id);

-- ── submissions ───────────────────────────────────────────────────────────────
create type submission_status as enum (
  'pending',
  'running',
  'passed',
  'failed',
  'error'
);

create table public.submissions (
  id              uuid              primary key default gen_random_uuid(),
  session_id      uuid              not null references public.sessions(id) on delete cascade,
  user_id         text              not null references public.users(id) on delete cascade,
  challenge_id    text              not null,
  status          submission_status not null default 'pending',
  result_json     jsonb,
  submitted_at    timestamptz       not null default now(),
  evaluated_at    timestamptz
);

create index submissions_session_id_idx   on public.submissions(session_id);
create index submissions_user_id_idx      on public.submissions(user_id);
create index submissions_challenge_id_idx on public.submissions(challenge_id);

-- ── updated_at trigger ────────────────────────────────────────────────────────
create or replace function public.set_updated_at()
returns trigger language plpgsql as $$
begin
  new.updated_at = now();
  return new;
end;
$$;

create trigger users_set_updated_at
  before update on public.users
  for each row execute function public.set_updated_at();

-- ── Grants for the app user ──────────────────────────────────────────────────
-- stacktrace_app already has broad grants from aurora_setup.md step 5,
-- but explicit per-table grants are safer if you ever tighten that.
grant select, insert, update, delete on public.users               to stacktrace_app;
grant select, insert, update, delete on public.sessions            to stacktrace_app;
grant select, insert, update, delete on public.session_file_diffs  to stacktrace_app;
grant select, insert, update, delete on public.submissions         to stacktrace_app;