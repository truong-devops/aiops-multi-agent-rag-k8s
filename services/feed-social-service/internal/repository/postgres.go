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

const commentColumns = `
	id, video_id, user_id, body, status, COALESCE(request_id, ''), COALESCE(correlation_id, ''), created_at, updated_at
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

func (s *PostgresStore) GetSocialCounters(ctx context.Context, videoID string) (domain.VideoSocialCounters, error) {
	item, err := findFeedItemWithCountersByVideoID(ctx, s.db, videoID)
	if err != nil {
		return domain.VideoSocialCounters{}, err
	}
	if item.Item.Status != domain.FeedItemStatusActive {
		return domain.VideoSocialCounters{}, domain.NotFound(domain.CodeFeedItemNotFound, "Feed item was not found.")
	}
	return item.Counters, nil
}

func (s *PostgresStore) SetVideoLike(ctx context.Context, mutation SocialMutation, liked bool) (domain.VideoSocialCounters, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.VideoSocialCounters{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := ensureActiveFeedItem(ctx, tx, mutation.VideoID); err != nil {
		return domain.VideoSocialCounters{}, false, err
	}
	now := mutation.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := ensureCounters(ctx, tx, mutation.VideoID, now); err != nil {
		return domain.VideoSocialCounters{}, false, err
	}
	changed := false
	if liked {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO likes (
				id, video_id, user_id, status, request_id, correlation_id, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, $7)
			ON CONFLICT (video_id, user_id) DO UPDATE
			SET status = EXCLUDED.status,
			    request_id = EXCLUDED.request_id,
			    correlation_id = EXCLUDED.correlation_id,
			    updated_at = EXCLUDED.updated_at
			WHERE likes.status <> $4
		`, domain.NewID("like"), strings.TrimSpace(mutation.VideoID), strings.TrimSpace(mutation.UserID), domain.LikeStatusActive, mutation.RequestID, mutation.CorrelationID, now)
		if err != nil {
			return domain.VideoSocialCounters{}, false, err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return domain.VideoSocialCounters{}, false, err
		}
		changed = rows > 0
		if changed {
			if _, err := tx.ExecContext(ctx, `
				UPDATE video_social_counters
				SET like_count = like_count + 1, updated_at = $2
				WHERE video_id = $1
			`, mutation.VideoID, now); err != nil {
				return domain.VideoSocialCounters{}, false, err
			}
		}
	} else {
		result, err := tx.ExecContext(ctx, `
			UPDATE likes
			SET status = $3,
			    request_id = NULLIF($4, ''),
			    correlation_id = NULLIF($5, ''),
			    updated_at = $6
			WHERE video_id = $1 AND user_id = $2 AND status = $7
		`, strings.TrimSpace(mutation.VideoID), strings.TrimSpace(mutation.UserID), domain.LikeStatusDeleted, mutation.RequestID, mutation.CorrelationID, now, domain.LikeStatusActive)
		if err != nil {
			return domain.VideoSocialCounters{}, false, err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return domain.VideoSocialCounters{}, false, err
		}
		changed = rows > 0
		if changed {
			if _, err := tx.ExecContext(ctx, `
				UPDATE video_social_counters
				SET like_count = GREATEST(like_count - 1, 0), updated_at = $2
				WHERE video_id = $1
			`, mutation.VideoID, now); err != nil {
				return domain.VideoSocialCounters{}, false, err
			}
		}
	}
	counters, err := findCountersByVideoID(ctx, tx, mutation.VideoID)
	if err != nil {
		return domain.VideoSocialCounters{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.VideoSocialCounters{}, false, err
	}
	return counters, changed, nil
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

func (s *PostgresStore) CreateComment(ctx context.Context, comment domain.Comment) (domain.Comment, domain.VideoSocialCounters, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureActiveFeedItem(ctx, tx, comment.VideoID); err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, err
	}
	if err := ensureCounters(ctx, tx, comment.VideoID, comment.CreatedAt); err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO comments (
			id, video_id, user_id, body, status, request_id, correlation_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), $8, $9)
	`, comment.ID, comment.VideoID, comment.UserID, comment.Body, comment.Status, comment.RequestID, comment.CorrelationID, comment.CreatedAt, comment.UpdatedAt)
	if err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE video_social_counters
		SET comment_count = comment_count + 1, updated_at = $2
		WHERE video_id = $1
	`, comment.VideoID, comment.CreatedAt); err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, err
	}
	counters, err := findCountersByVideoID(ctx, tx, comment.VideoID)
	if err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, err
	}
	return comment, counters, nil
}

func (s *PostgresStore) ListComments(ctx context.Context, filter ListCommentsFilter) ([]domain.Comment, error) {
	if err := ensureActiveFeedItem(ctx, s.db, filter.VideoID); err != nil {
		return nil, err
	}
	limit := normalizedLimit(filter.Limit)
	var beforeCreatedAt any
	if filter.BeforeCreatedAt != nil {
		beforeCreatedAt = filter.BeforeCreatedAt.UTC()
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+commentColumns+`
		FROM comments
		WHERE video_id = $1
		  AND status = $2
		  AND (
		    $3::timestamptz IS NULL
		    OR created_at < $3::timestamptz
		    OR (created_at = $3::timestamptz AND ($4 = '' OR id < $4))
		  )
		ORDER BY created_at DESC, id DESC
		LIMIT $5
	`, strings.TrimSpace(filter.VideoID), domain.CommentStatusVisible, beforeCreatedAt, strings.TrimSpace(filter.BeforeCommentID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := make([]domain.Comment, 0)
	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return comments, nil
}

func (s *PostgresStore) DeleteComment(ctx context.Context, commentID string, actorID string, actorRole string, now time.Time) (domain.Comment, domain.VideoSocialCounters, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	row := tx.QueryRowContext(ctx, `
		SELECT `+commentColumns+`
		FROM comments
		WHERE id = $1
		FOR UPDATE
	`, strings.TrimSpace(commentID))
	comment, err := scanComment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Comment{}, domain.VideoSocialCounters{}, false, domain.NotFound(domain.CodeCommentNotFound, "Comment was not found.")
	}
	if err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, false, err
	}
	if comment.UserID != strings.TrimSpace(actorID) && strings.TrimSpace(actorRole) != "admin" {
		return domain.Comment{}, domain.VideoSocialCounters{}, false, domain.Forbidden("Only the comment owner can delete this comment.")
	}
	if err := ensureCounters(ctx, tx, comment.VideoID, comment.UpdatedAt); err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, false, err
	}
	if comment.Status == domain.CommentStatusDeleted {
		counters, err := findCountersByVideoID(ctx, tx, comment.VideoID)
		if err != nil {
			return domain.Comment{}, domain.VideoSocialCounters{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return domain.Comment{}, domain.VideoSocialCounters{}, false, err
		}
		return comment, counters, false, nil
	}
	now = now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	wasVisible := comment.Status == domain.CommentStatusVisible
	comment.Status = domain.CommentStatusDeleted
	comment.Body = ""
	comment.UpdatedAt = now
	if _, err := tx.ExecContext(ctx, `
		UPDATE comments
		SET status = $2, body = '', updated_at = $3
		WHERE id = $1
	`, comment.ID, domain.CommentStatusDeleted, now); err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, false, err
	}
	if wasVisible {
		if _, err := tx.ExecContext(ctx, `
			UPDATE video_social_counters
			SET comment_count = GREATEST(comment_count - 1, 0), updated_at = $2
			WHERE video_id = $1
		`, comment.VideoID, now); err != nil {
			return domain.Comment{}, domain.VideoSocialCounters{}, false, err
		}
	}
	counters, err := findCountersByVideoID(ctx, tx, comment.VideoID)
	if err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, false, err
	}
	return comment, counters, true, nil
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

func ensureActiveFeedItem(ctx context.Context, q queryer, videoID string) error {
	var status string
	err := q.QueryRowContext(ctx, `
		SELECT status
		FROM feed_items
		WHERE video_id = $1
	`, strings.TrimSpace(videoID)).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NotFound(domain.CodeFeedItemNotFound, "Feed item was not found.")
	}
	if err != nil {
		return err
	}
	if status != domain.FeedItemStatusActive {
		return domain.NotFound(domain.CodeFeedItemNotFound, "Feed item was not found.")
	}
	return nil
}

func ensureCounters(ctx context.Context, tx *sql.Tx, videoID string, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO video_social_counters (video_id, like_count, comment_count, share_count, updated_at)
		VALUES ($1, 0, 0, 0, $2)
		ON CONFLICT (video_id) DO NOTHING
	`, strings.TrimSpace(videoID), now.UTC())
	return err
}

func findCountersByVideoID(ctx context.Context, q queryer, videoID string) (domain.VideoSocialCounters, error) {
	var counters domain.VideoSocialCounters
	err := q.QueryRowContext(ctx, `
		SELECT video_id, like_count, comment_count, share_count, updated_at
		FROM video_social_counters
		WHERE video_id = $1
	`, strings.TrimSpace(videoID)).Scan(&counters.VideoID, &counters.LikeCount, &counters.CommentCount, &counters.ShareCount, &counters.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.VideoSocialCounters{}, domain.NotFound(domain.CodeFeedItemNotFound, "Feed item was not found.")
	}
	return counters, err
}

func scanComment(scanner sqlScanner) (domain.Comment, error) {
	var comment domain.Comment
	err := scanner.Scan(
		&comment.ID,
		&comment.VideoID,
		&comment.UserID,
		&comment.Body,
		&comment.Status,
		&comment.RequestID,
		&comment.CorrelationID,
		&comment.CreatedAt,
		&comment.UpdatedAt,
	)
	return comment, err
}
