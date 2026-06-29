CREATE TABLE IF NOT EXISTS videos (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    visibility TEXT NOT NULL,
    raw_object_key TEXT,
    processed_object_key TEXT,
    thumbnail_object_key TEXT,
    content_type TEXT,
    size_bytes BIGINT,
    duration_ms BIGINT,
    width INTEGER,
    height INTEGER,
    processing_error_code TEXT,
    published_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    last_request_id TEXT,
    last_correlation_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT videos_status_check CHECK (status IN ('draft', 'uploaded', 'processing', 'ready', 'failed', 'deleted')),
    CONSTRAINT videos_visibility_check CHECK (visibility IN ('public', 'private', 'unlisted')),
    CONSTRAINT videos_size_bytes_check CHECK (size_bytes IS NULL OR size_bytes >= 0),
    CONSTRAINT videos_duration_ms_check CHECK (duration_ms IS NULL OR duration_ms >= 0),
    CONSTRAINT videos_width_check CHECK (width IS NULL OR width >= 0),
    CONSTRAINT videos_height_check CHECK (height IS NULL OR height >= 0)
);

CREATE TABLE IF NOT EXISTS upload_requests (
    id TEXT PRIMARY KEY,
    video_id TEXT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    owner_id TEXT NOT NULL,
    bucket TEXT NOT NULL,
    object_key TEXT NOT NULL,
    status TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT,
    checksum_sha256 TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    request_id TEXT,
    correlation_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT upload_requests_status_check CHECK (status IN ('created', 'uploaded', 'expired', 'cancelled')),
    CONSTRAINT upload_requests_size_bytes_check CHECK (size_bytes IS NULL OR size_bytes >= 0)
);

CREATE TABLE IF NOT EXISTS video_assets (
    id TEXT PRIMARY KEY,
    video_id TEXT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    asset_type TEXT NOT NULL,
    bucket TEXT NOT NULL,
    object_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT,
    width INTEGER,
    height INTEGER,
    duration_ms BIGINT,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT video_assets_asset_type_check CHECK (asset_type IN ('raw', 'mp4_720p', 'hls_master', 'thumbnail')),
    CONSTRAINT video_assets_size_bytes_check CHECK (size_bytes IS NULL OR size_bytes >= 0),
    CONSTRAINT video_assets_width_check CHECK (width IS NULL OR width >= 0),
    CONSTRAINT video_assets_height_check CHECK (height IS NULL OR height >= 0),
    CONSTRAINT video_assets_duration_ms_check CHECK (duration_ms IS NULL OR duration_ms >= 0)
);

CREATE TABLE IF NOT EXISTS video_status_history (
    id TEXT PRIMARY KEY,
    video_id TEXT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    previous_status TEXT,
    new_status TEXT NOT NULL,
    reason TEXT,
    error_code TEXT,
    request_id TEXT,
    correlation_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT video_status_history_previous_status_check CHECK (previous_status IS NULL OR previous_status IN ('draft', 'uploaded', 'processing', 'ready', 'failed', 'deleted')),
    CONSTRAINT video_status_history_new_status_check CHECK (new_status IN ('draft', 'uploaded', 'processing', 'ready', 'failed', 'deleted'))
);

CREATE TABLE IF NOT EXISTS outbox_events (
    id TEXT PRIMARY KEY,
    event_name TEXT NOT NULL,
    event_version TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL,
    request_id TEXT,
    correlation_id TEXT,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT outbox_events_status_check CHECK (status IN ('pending', 'published', 'failed'))
);

CREATE INDEX IF NOT EXISTS videos_owner_created_at_idx ON videos (owner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS videos_status_updated_at_idx ON videos (status, updated_at DESC);
CREATE INDEX IF NOT EXISTS videos_visibility_published_at_idx ON videos (visibility, published_at DESC);
CREATE INDEX IF NOT EXISTS upload_requests_video_id_idx ON upload_requests (video_id);
CREATE INDEX IF NOT EXISTS upload_requests_owner_status_idx ON upload_requests (owner_id, status);
CREATE INDEX IF NOT EXISTS upload_requests_expires_at_idx ON upload_requests (expires_at);
CREATE INDEX IF NOT EXISTS video_assets_video_id_idx ON video_assets (video_id);
CREATE INDEX IF NOT EXISTS video_status_history_video_created_at_idx ON video_status_history (video_id, created_at DESC);
CREATE INDEX IF NOT EXISTS outbox_events_status_created_at_idx ON outbox_events (status, created_at);
CREATE INDEX IF NOT EXISTS outbox_events_aggregate_id_idx ON outbox_events (aggregate_id);
