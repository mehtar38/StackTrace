-- -- ============================================================
-- -- StackTrace — Supabase Schema
-- -- Migration: 001_initial_schema
-- -- Run via Supabase dashboard or CLI: supabase db push
-- -- ============================================================

-- -- Enable UUID generation
-- create extension if not exists "pgcrypto";

-- -- ============================================================
-- -- users
-- -- Mirrors Clerk users. `id` is the Clerk `sub` claim (string).
-- -- Created on first authenticated request to the orchestrator.
-- -- ============================================================
-- create table public.users (
--   id          text        primary key,         -- Clerk sub claim, e.g. "user_2abc..."
--   email       text        not null unique,
--   created_at  timestamptz not null default now(),
--   updated_at  timestamptz not null default now()
-- );

-- comment on table public.users is 'Mirrors Clerk user records. Created on first orchestrator contact.';

-- -- ============================================================
-- -- sessions
-- -- One row per user×challenge attempt. A user can have multiple
-- -- sessions for the same challenge (retries after expiry).
-- -- ============================================================
-- create type session_status as enum (
--   'prewarming',   -- container spinning up, user has not started yet
--   'active',       -- user clicked Start Challenge, challenge clock running
--   'exited',       -- user clicked Save & Exit cleanly
--   'completed',    -- user submitted and evaluation finished
--   'expired',      -- hard 60-min limit hit, or inactivity timeout
--   'error'         -- container failed to start or died unexpectedly
-- );

-- create table public.sessions (
--   id                  uuid          primary key default gen_random_uuid(),
--   user_id             text          references public.users(id) on delete cascade,
--   challenge_id        text          not null,                   -- e.g. "01-silent-write"
--   status              session_status not null default 'prewarming',
--   anon_token          text          unique,                     -- UUID from localStorage, used during prewarm→promote
--   container_id        text,                                     -- Docker / ACA container ID
--   container_host      text,                                     -- host:port or ACA FQDN for WebSocket
--   started_at          timestamptz,                              -- set when status → active
--   last_active_at      timestamptz,                              -- updated on file write or terminal activity
--   ended_at            timestamptz,                              -- set on exit / complete / expire
--   duration_seconds    integer,                                  -- computed on session close
--   created_at          timestamptz   not null default now()
-- );

-- comment on table public.sessions is 'One row per challenge attempt. Tracks lifecycle from prewarm to completion.';

-- create index sessions_user_id_idx       on public.sessions(user_id);
-- create index sessions_challenge_id_idx  on public.sessions(challenge_id);
-- create index sessions_anon_token_idx    on public.sessions(anon_token);
-- create index sessions_status_idx        on public.sessions(status);

-- -- ============================================================
-- -- session_file_diffs
-- -- Stores the last known state of every modified file as full
-- -- file content (not actual diffs — files are small, content is
-- -- simpler to replay). One row per file per session; upserted on
-- -- each Save & Exit so only the latest version is stored.
-- -- ============================================================
-- create table public.session_file_diffs (
--   id          uuid        primary key default gen_random_uuid(),
--   session_id  uuid        not null references public.sessions(id) on delete cascade,
--   file_path   text        not null,    -- relative path, e.g. "src/db/write.js"
--   content     text        not null,    -- full file content at time of save
--   saved_at    timestamptz not null default now(),

--   -- Only one row per file per session; upsert on conflict
--   unique (session_id, file_path)
-- );

-- comment on table public.session_file_diffs is 'Full file content of modified files, saved on exit. Replayed on session resume.';

-- create index session_file_diffs_session_id_idx on public.session_file_diffs(session_id);

-- -- ============================================================
-- -- submissions
-- -- One row per submission attempt. A session can have multiple
-- -- submissions (user can submit, fail, keep working, resubmit).
-- -- result_json holds structured per-test-case pass/fail output.
-- -- ============================================================
-- create type submission_status as enum (
--   'pending',    -- evaluator container starting
--   'running',    -- test suite executing
--   'passed',     -- all tests passed
--   'failed',     -- some or all tests failed
--   'error'       -- evaluator crashed or timed out
-- );

-- create table public.submissions (
--   id              uuid              primary key default gen_random_uuid(),
--   session_id      uuid              not null references public.sessions(id) on delete cascade,
--   user_id         text              not null references public.users(id) on delete cascade,
--   challenge_id    text              not null,
--   status          submission_status not null default 'pending',
--   result_json     jsonb,            -- structured per-test pass/fail, null until evaluation complete
--   submitted_at    timestamptz       not null default now(),
--   evaluated_at    timestamptz
-- );

-- comment on table public.submissions is 'Per-submission evaluation results. result_json holds structured test output.';

-- create index submissions_session_id_idx  on public.submissions(session_id);
-- create index submissions_user_id_idx     on public.submissions(user_id);
-- create index submissions_challenge_id_idx on public.submissions(challenge_id);

-- -- ============================================================
-- -- updated_at trigger (applied to users)
-- -- ============================================================
-- create or replace function public.set_updated_at()
-- returns trigger language plpgsql as $$
-- begin
--   new.updated_at = now();
--   return new;
-- end;
-- $$;

-- create trigger users_set_updated_at
--   before update on public.users
--   for each row execute function public.set_updated_at();

-- -- ============================================================
-- -- Row Level Security
-- -- The orchestrator connects via service role key (bypasses RLS).
-- -- RLS is defined here for future direct-from-client queries and
-- -- security auditing. All policies require authenticated uid match.
-- -- ============================================================
-- alter table public.users               enable row level security;
-- alter table public.sessions            enable row level security;
-- alter table public.session_file_diffs  enable row level security;
-- alter table public.submissions         enable row level security;

-- -- Users can only read/update their own record
-- create policy "users: own record" on public.users
--   for all using (id = auth.uid()::text);

-- -- Users can only see their own sessions
-- create policy "sessions: own sessions" on public.sessions
--   for all using (user_id = auth.uid()::text);

-- -- Users can only see diffs from their own sessions
-- create policy "session_file_diffs: own sessions" on public.session_file_diffs
--   for all using (
--     session_id in (
--       select id from public.sessions where user_id = auth.uid()::text
--     )
--   );

-- -- Users can only see their own submissions
-- create policy "submissions: own submissions" on public.submissions
--   for all using (user_id = auth.uid()::text);