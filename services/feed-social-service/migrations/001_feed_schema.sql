CREATE TABLE IF NOT EXISTS feed_items (
    id TEXT PRIMARY KEY,
    video_id TEXT NOT NULL UNIQUE,
    owner_id TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    thumbnail_object_key TEXT NOT NULL DEFAULT '',
    playback_object_key TEXT NOT NULL DEFAULT '',
    duration_ms BIGINT,
    visibility TEXT NOT NULL DEFAULT 'public',
    status TEXT NOT NULL,
    ready_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT feed_items_status_check CHECK (status IN ('active', 'hidden', 'deleted')),
    CONSTRAINT feed_items_visibility_check CHECK (visibility IN ('public', 'private', 'unlisted'))
);

CREATE INDEX IF NOT EXISTS feed_items_status_ready_idx
    ON feed_items (status, ready_at DESC, video_id DESC);

CREATE INDEX IF NOT EXISTS feed_items_owner_ready_idx
    ON feed_items (owner_id, ready_at DESC);

CREATE TABLE IF NOT EXISTS video_social_counters (
    video_id TEXT PRIMARY KEY,
    like_count BIGINT NOT NULL DEFAULT 0,
    comment_count BIGINT NOT NULL DEFAULT 0,
    share_count BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS inbox_events (
    id TEXT PRIMARY KEY,
    event_name TEXT NOT NULL,
    event_version TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    status TEXT NOT NULL,
    request_id TEXT,
    correlation_id TEXT,
    received_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ,
    CONSTRAINT inbox_events_status_check CHECK (status IN ('processed', 'duplicate'))
);

CREATE INDEX IF NOT EXISTS inbox_events_aggregate_idx
    ON inbox_events (aggregate_id, received_at DESC);
