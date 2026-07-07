CREATE TABLE IF NOT EXISTS follows (
    id TEXT PRIMARY KEY,
    follower_id TEXT NOT NULL,
    followee_id TEXT NOT NULL,
    status TEXT NOT NULL,
    request_id TEXT,
    correlation_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT follows_status_check CHECK (status IN ('active', 'deleted', 'blocked')),
    CONSTRAINT follows_not_self_check CHECK (follower_id <> followee_id),
    CONSTRAINT follows_pair_unique UNIQUE (follower_id, followee_id)
);

CREATE INDEX IF NOT EXISTS follows_follower_status_idx
    ON follows (follower_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS follows_followee_status_idx
    ON follows (followee_id, status, updated_at DESC);
