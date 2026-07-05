package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

const feedItemColumns = `
	id, video_id, owner_id, title, description, thumbnail_object_key,
	playback_object_key, duration_ms, visibility, status, ready_at, created_at, updated_at
`

type PostgresStore struct {
	db *sql.DB
}

type sqlScanner interface {
	Scan(dest ...any) error
}

func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, errors.New("database url is required")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	if ctx == nil {
		ctx = context.Background()
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *PostgresStore) UpsertFeedItemFromReadyVideo(ctx context.Context, input domain.ReadyVideoInput, item domain.FeedItem) (domain.FeedItem, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.FeedItem{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	if strings.TrimSpace(input.EventID) != "" {
		processedAt := item.UpdatedAt
		result, err := tx.ExecContext(ctx, `
			INSERT INTO inbox_events (
				id, event_name, event_version, aggregate_id, status,
				request_id, correlation_id, received_at, processed_at
			)
			VALUES ($1, 'video.ready', 'v1', $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, $7)
			ON CONFLICT (id) DO NOTHING
		`, input.EventID, item.VideoID, domain.InboxStatusProcessed, input.RequestID, input.CorrelationID, input.ReceivedAt, processedAt)
		if err != nil {
			return domain.FeedItem{}, false, err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return domain.FeedItem{}, false, err
		}
		if rows == 0 {
			existing, err := findFeedItemWithCountersByVideoID(ctx, tx, item.VideoID)
			if err != nil {
				return domain.FeedItem{}, false, err
			}
			if err := tx.Commit(); err != nil {
				return domain.FeedItem{}, false, err
			}
			return existing.Item, false, nil
		}
	}

	var existed bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM feed_items WHERE video_id = $1)`, item.VideoID).Scan(&existed); err != nil {
		return domain.FeedItem{}, false, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO feed_items (
			id, video_id, owner_id, title, description, thumbnail_object_key,
			playback_object_key, duration_ms, visibility, status, ready_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (video_id) DO UPDATE
		SET owner_id = EXCLUDED.owner_id,
		    title = EXCLUDED.title,
		    description = EXCLUDED.description,
		    thumbnail_object_key = EXCLUDED.thumbnail_object_key,
		    playback_object_key = EXCLUDED.playback_object_key,
		    duration_ms = EXCLUDED.duration_ms,
		    visibility = EXCLUDED.visibility,
		    status = EXCLUDED.status,
		    ready_at = EXCLUDED.ready_at,
		    updated_at = EXCLUDED.updated_at
	`, item.ID, item.VideoID, item.OwnerID, item.Title, item.Description, item.ThumbnailObjectKey,
		item.PlaybackObjectKey, nullableInt64(item.DurationMs), item.Visibility, item.Status, item.ReadyAt, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return domain.FeedItem{}, false, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO video_social_counters (video_id, like_count, comment_count, share_count, updated_at)
		VALUES ($1, 0, 0, 0, $2)
		ON CONFLICT (video_id) DO NOTHING
	`, item.VideoID, item.UpdatedAt)
	if err != nil {
		return domain.FeedItem{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.FeedItem{}, false, err
	}
	return item, !existed, nil
}

func (s *PostgresStore) FindFeedItemByVideoID(ctx context.Context, videoID string) (domain.FeedItemWithCounters, error) {
	return findFeedItemWithCountersByVideoID(ctx, s.db, videoID)
}

func (s *PostgresStore) ListFeedItems(ctx context.Context, filter ListFeedFilter) ([]domain.FeedItemWithCounters, error) {
	limit := normalizedLimit(filter.Limit)
	var beforeReadyAt any
	if filter.BeforeReadyAt != nil {
		beforeReadyAt = filter.BeforeReadyAt.UTC()
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+feedItemColumns+`,
		       COALESCE(c.like_count, 0), COALESCE(c.comment_count, 0), COALESCE(c.share_count, 0), COALESCE(c.updated_at, f.updated_at)
		FROM feed_items f
		LEFT JOIN video_social_counters c ON c.video_id = f.video_id
		WHERE f.status = $1
		  AND (
		    $2::timestamptz IS NULL
		    OR f.ready_at < $2::timestamptz
		    OR (f.ready_at = $2::timestamptz AND ($3 = '' OR f.video_id < $3))
		  )
		ORDER BY f.ready_at DESC, f.video_id DESC
		LIMIT $4
	`, domain.FeedItemStatusActive, beforeReadyAt, strings.TrimSpace(filter.BeforeVideoID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.FeedItemWithCounters, 0)
	for rows.Next() {
		item, err := scanFeedItemWithCounters(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func findFeedItemWithCountersByVideoID(ctx context.Context, q queryer, videoID string) (domain.FeedItemWithCounters, error) {
	row := q.QueryRowContext(ctx, `
		SELECT `+feedItemColumns+`,
		       COALESCE(c.like_count, 0), COALESCE(c.comment_count, 0), COALESCE(c.share_count, 0), COALESCE(c.updated_at, f.updated_at)
		FROM feed_items f
		LEFT JOIN video_social_counters c ON c.video_id = f.video_id
		WHERE f.video_id = $1
	`, videoID)
	item, err := scanFeedItemWithCounters(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.FeedItemWithCounters{}, domain.NotFound(domain.CodeFeedItemNotFound, "Feed item was not found.")
	}
	return item, err
}

func scanFeedItemWithCounters(scanner sqlScanner) (domain.FeedItemWithCounters, error) {
	var item domain.FeedItem
	var durationMs sql.NullInt64
	var counters domain.VideoSocialCounters
	err := scanner.Scan(
		&item.ID, &item.VideoID, &item.OwnerID, &item.Title, &item.Description, &item.ThumbnailObjectKey,
		&item.PlaybackObjectKey, &durationMs, &item.Visibility, &item.Status, &item.ReadyAt, &item.CreatedAt, &item.UpdatedAt,
		&counters.LikeCount, &counters.CommentCount, &counters.ShareCount, &counters.UpdatedAt,
	)
	if err != nil {
		return domain.FeedItemWithCounters{}, err
	}
	item.DurationMs = durationMs.Int64
	counters.VideoID = item.VideoID
	return domain.FeedItemWithCounters{Item: item, Counters: counters}, nil
}

func nullableInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: value > 0}
}
