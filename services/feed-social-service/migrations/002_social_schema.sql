CREATE TABLE IF NOT EXISTS likes (
    id TEXT PRIMARY KEY,
    video_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    status TEXT NOT NULL,
    request_id TEXT,
    correlation_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT likes_status_check CHECK (status IN ('active', 'deleted')),
    CONSTRAINT likes_video_user_unique UNIQUE (video_id, user_id)
);

CREATE INDEX IF NOT EXISTS likes_video_status_idx
    ON likes (video_id, status);

CREATE INDEX IF NOT EXISTS likes_user_updated_idx
    ON likes (user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS comments (
    id TEXT PRIMARY KEY,
    video_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    body TEXT NOT NULL,
    status TEXT NOT NULL,
    request_id TEXT,
    correlation_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT comments_status_check CHECK (status IN ('visible', 'hidden', 'deleted', 'blocked'))
);

CREATE INDEX IF NOT EXISTS comments_video_visible_created_idx
    ON comments (video_id, status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS comments_user_created_idx
    ON comments (user_id, created_at DESC);
