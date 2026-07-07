CREATE TABLE IF NOT EXISTS live_sessions (
    id text PRIMARY KEY,
    creator_id text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    status text NOT NULL,
    stream_key_hash text NOT NULL,
    ingest_path text NOT NULL,
    playback_path text NOT NULL,
    scheduled_at timestamptz,
    started_at timestamptz,
    ended_at timestamptz,
    failure_code text,
    last_request_id text,
    last_correlation_id text,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT live_sessions_status_check CHECK (status IN ('scheduled', 'live', 'ended', 'failed', 'cancelled')),
    CONSTRAINT live_sessions_title_not_blank CHECK (length(btrim(title)) > 0)
);

CREATE INDEX IF NOT EXISTS live_sessions_creator_created_idx
    ON live_sessions (creator_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS live_sessions_status_updated_idx
    ON live_sessions (status, updated_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS stream_keys (
    id text PRIMARY KEY,
    live_session_id text NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
    key_hash text NOT NULL,
    status text NOT NULL,
    created_at timestamptz NOT NULL,
    rotated_at timestamptz,
    revoked_at timestamptz,
    CONSTRAINT stream_keys_status_check CHECK (status IN ('active', 'rotated', 'revoked'))
);

CREATE UNIQUE INDEX IF NOT EXISTS stream_keys_active_session_idx
    ON stream_keys (live_session_id)
    WHERE status = 'active';

CREATE TABLE IF NOT EXISTS live_events (
    id text PRIMARY KEY,
    live_session_id text NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    request_id text,
    correlation_id text,
    occurred_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS live_events_session_time_idx
    ON live_events (live_session_id, occurred_at DESC);
