CREATE TABLE IF NOT EXISTS inbox_events (
    id TEXT PRIMARY KEY,
    event_name TEXT NOT NULL,
    event_version TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    status TEXT NOT NULL,
    request_id TEXT,
    correlation_id TEXT,
    received_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS inbox_events_aggregate_idx
    ON inbox_events (aggregate_id, received_at DESC);

CREATE TABLE IF NOT EXISTS processing_jobs (
    id TEXT PRIMARY KEY,
    video_id TEXT NOT NULL,
    owner_id TEXT NOT NULL DEFAULT '',
    input_bucket TEXT NOT NULL,
    input_object_key TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT,
    status TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    locked_by TEXT,
    locked_until TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error_code TEXT,
    error_message TEXT,
    request_id TEXT,
    correlation_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT processing_jobs_status_check CHECK (status IN ('queued', 'running', 'retrying', 'succeeded', 'failed', 'dead_letter', 'cancelled')),
    CONSTRAINT processing_jobs_attempts_check CHECK (attempt_count >= 0 AND max_attempts > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS processing_jobs_video_id_unique
    ON processing_jobs (video_id);

CREATE INDEX IF NOT EXISTS processing_jobs_runnable_idx
    ON processing_jobs (status, next_run_at, priority DESC);

CREATE INDEX IF NOT EXISTS processing_jobs_locked_until_idx
    ON processing_jobs (locked_until);

CREATE TABLE IF NOT EXISTS processing_attempts (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL REFERENCES processing_jobs(id) ON DELETE CASCADE,
    video_id TEXT NOT NULL,
    attempt_no INTEGER NOT NULL,
    worker_id TEXT NOT NULL,
    status TEXT NOT NULL,
    ffmpeg_command_hash TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    exit_code INTEGER,
    error_code TEXT,
    stderr_excerpt TEXT,
    metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT processing_attempts_status_check CHECK (status IN ('running', 'succeeded', 'failed')),
    CONSTRAINT processing_attempts_attempt_no_check CHECK (attempt_no > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS processing_attempts_job_attempt_unique
    ON processing_attempts (job_id, attempt_no);

CREATE INDEX IF NOT EXISTS processing_attempts_video_idx
    ON processing_attempts (video_id, started_at DESC);

CREATE TABLE IF NOT EXISTS dead_letters (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL REFERENCES processing_jobs(id) ON DELETE CASCADE,
    video_id TEXT NOT NULL,
    reason_code TEXT NOT NULL,
    payload JSONB NOT NULL,
    request_id TEXT,
    correlation_id TEXT,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS dead_letters_video_idx
    ON dead_letters (video_id, created_at DESC);
