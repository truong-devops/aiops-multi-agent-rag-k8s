package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
)

const videoColumns = `
	id, owner_id, title, description, status, visibility,
	raw_object_key, processed_object_key, thumbnail_object_key,
	content_type, size_bytes, duration_ms, width, height,
	processing_error_code, published_at, deleted_at,
	last_request_id, last_correlation_id,
	created_at, updated_at
`

const outboxColumns = `
	id, event_name, event_version, aggregate_id, producer, environment,
	payload, status, request_id, correlation_id, occurred_at, published_at,
	created_at, attempts, last_error
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

func (s *PostgresStore) CreateVideoWithUploadRequest(ctx context.Context, video domain.Video, upload domain.UploadRequest, history domain.StatusHistory) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertVideo(ctx, tx, video); err != nil {
		return err
	}
	if err := insertUploadRequest(ctx, tx, upload); err != nil {
		return err
	}
	if err := insertStatusHistory(ctx, tx, history); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) FindVideoByID(ctx context.Context, id string) (domain.Video, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+videoColumns+`
		FROM videos
		WHERE id = $1
	`, id)
	video, err := scanVideo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Video{}, domain.NotFound(domain.CodeVideoNotFound, "Video was not found.")
	}
	if err != nil {
		return domain.Video{}, err
	}
	return video, nil
}

func (s *PostgresStore) ListVideos(ctx context.Context, filter ListVideosFilter) ([]domain.Video, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT `+videoColumns+`
		FROM videos
		WHERE ($1 = '' OR owner_id = $1)
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC
		LIMIT $3
	`, strings.TrimSpace(filter.OwnerID), strings.TrimSpace(filter.Status), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	videos := make([]domain.Video, 0)
	for rows.Next() {
		video, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		videos = append(videos, video)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return videos, nil
}

func (s *PostgresStore) FindUploadRequestByID(ctx context.Context, id string) (domain.UploadRequest, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, video_id, owner_id, idempotency_key, bucket, object_key, status, content_type, size_bytes,
		       checksum_sha256, expires_at, completed_at, request_id, correlation_id, created_at, updated_at
		FROM upload_requests
		WHERE id = $1
	`, id)
	upload, err := scanUploadRequest(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.UploadRequest{}, domain.NotFound(domain.CodeUploadRequestNotFound, "Upload request was not found.")
	}
	if err != nil {
		return domain.UploadRequest{}, err
	}
	return upload, nil
}

func (s *PostgresStore) FindUploadIntentByIdempotencyKey(ctx context.Context, ownerID string, idempotencyKey string) (domain.Video, domain.UploadRequest, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+prefixedVideoColumns("v")+`,
		       u.id, u.video_id, u.owner_id, u.idempotency_key, u.bucket, u.object_key, u.status, u.content_type,
		       u.size_bytes, u.checksum_sha256, u.expires_at, u.completed_at, u.request_id, u.correlation_id, u.created_at, u.updated_at
		FROM upload_requests u
		JOIN videos v ON v.id = u.video_id
		WHERE u.owner_id = $1 AND u.idempotency_key = $2
	`, ownerID, idempotencyKey)
	video, upload, err := scanVideoWithUpload(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Video{}, domain.UploadRequest{}, domain.NotFound(domain.CodeUploadRequestNotFound, "Upload request was not found.")
	}
	if err != nil {
		return domain.Video{}, domain.UploadRequest{}, err
	}
	return video, upload, nil
}

func (s *PostgresStore) SaveUploadRequest(ctx context.Context, upload domain.UploadRequest) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE upload_requests
		SET status = $2,
		    size_bytes = $3,
		    checksum_sha256 = NULLIF($4, ''),
		    completed_at = $5,
		    request_id = NULLIF($6, ''),
		    correlation_id = NULLIF($7, ''),
		    updated_at = $8
		WHERE id = $1
	`, upload.ID, upload.Status, nullableInt64(upload.SizeBytes), upload.ChecksumSHA256, upload.CompletedAt, upload.RequestID, upload.CorrelationID, upload.UpdatedAt)
	if err != nil {
		return err
	}
	return requireRowsAffected(result, domain.NotFound(domain.CodeUploadRequestNotFound, "Upload request was not found."))
}

func (s *PostgresStore) CompleteUpload(ctx context.Context, upload domain.UploadRequest, video domain.Video, history domain.StatusHistory, outbox domain.OutboxEvent) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := updateUploadRequest(ctx, tx, upload); err != nil {
		return err
	}
	if err := updateVideoStatus(ctx, tx, video); err != nil {
		return err
	}
	if err := insertStatusHistory(ctx, tx, history); err != nil {
		return err
	}
	if outbox.ID != "" {
		if err := insertOutboxEvent(ctx, tx, outbox); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) SaveVideoStatus(ctx context.Context, video domain.Video, history domain.StatusHistory) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := updateVideoStatus(ctx, tx, video); err != nil {
		return err
	}
	if err := insertStatusHistory(ctx, tx, history); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) ListPendingOutboxEvents(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+outboxColumns+`
		FROM outbox_events
		WHERE status IN ($1, $2)
		  AND attempts < 10
		ORDER BY created_at ASC
		LIMIT $3
	`, domain.OutboxStatusPending, domain.OutboxStatusFailed, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]domain.OutboxEvent, 0, limit)
	for rows.Next() {
		event, err := scanOutboxEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *PostgresStore) MarkOutboxPublished(ctx context.Context, id string, publishedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET status = $2,
		    published_at = $3,
		    last_error = NULL
		WHERE id = $1
	`, id, domain.OutboxStatusPublished, publishedAt.UTC())
	if err != nil {
		return err
	}
	return requireRowsAffected(result, domain.NotFound(domain.CodeVideoNotFound, "Outbox event was not found."))
}

func (s *PostgresStore) MarkOutboxFailed(ctx context.Context, id string, errMessage string) error {
	if len(errMessage) > 1000 {
		errMessage = errMessage[:1000]
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET status = $2,
		    attempts = attempts + 1,
		    last_error = NULLIF($3, '')
		WHERE id = $1
	`, id, domain.OutboxStatusFailed, errMessage)
	if err != nil {
		return err
	}
	return requireRowsAffected(result, domain.NotFound(domain.CodeVideoNotFound, "Outbox event was not found."))
}

func insertVideo(ctx context.Context, exec sqlExecutor, video domain.Video) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO videos (
			id, owner_id, title, description, status, visibility,
			raw_object_key, processed_object_key, thumbnail_object_key,
			content_type, size_bytes, duration_ms, width, height,
			processing_error_code, published_at, deleted_at,
			last_request_id, last_correlation_id,
			created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''),
			NULLIF($10, ''), $11, $12, $13, $14,
			NULLIF($15, ''), $16, $17,
			NULLIF($18, ''), NULLIF($19, ''),
			$20, $21
		)
	`, video.ID, video.OwnerID, video.Title, video.Description, video.Status, video.Visibility,
		video.RawObjectKey, video.ProcessedObjectKey, video.ThumbnailObjectKey,
		video.ContentType, nullableInt64(video.SizeBytes), nullableInt64(video.DurationMs), nullableInt(video.Width), nullableInt(video.Height),
		video.ProcessingErrorCode, video.PublishedAt, video.DeletedAt,
		video.LastRequestID, video.LastCorrelationID,
		video.CreatedAt, video.UpdatedAt)
	return err
}

func insertUploadRequest(ctx context.Context, exec sqlExecutor, upload domain.UploadRequest) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO upload_requests (
			id, video_id, owner_id, idempotency_key, bucket, object_key, status, content_type,
			size_bytes, checksum_sha256, expires_at, completed_at,
			request_id, correlation_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, $9, NULLIF($10, ''), $11, $12, NULLIF($13, ''), NULLIF($14, ''), $15, $16)
	`, upload.ID, upload.VideoID, upload.OwnerID, upload.IdempotencyKey, upload.Bucket, upload.ObjectKey, upload.Status, upload.ContentType,
		nullableInt64(upload.SizeBytes), upload.ChecksumSHA256, upload.ExpiresAt, upload.CompletedAt,
		upload.RequestID, upload.CorrelationID, upload.CreatedAt, upload.UpdatedAt)
	return err
}

func updateUploadRequest(ctx context.Context, exec sqlExecutor, upload domain.UploadRequest) error {
	result, err := exec.ExecContext(ctx, `
		UPDATE upload_requests
		SET status = $2,
		    size_bytes = $3,
		    checksum_sha256 = NULLIF($4, ''),
		    completed_at = $5,
		    request_id = NULLIF($6, ''),
		    correlation_id = NULLIF($7, ''),
		    updated_at = $8
		WHERE id = $1
	`, upload.ID, upload.Status, nullableInt64(upload.SizeBytes), upload.ChecksumSHA256, upload.CompletedAt, upload.RequestID, upload.CorrelationID, upload.UpdatedAt)
	if err != nil {
		return err
	}
	return requireRowsAffected(result, domain.NotFound(domain.CodeUploadRequestNotFound, "Upload request was not found."))
}

func updateVideoStatus(ctx context.Context, exec sqlExecutor, video domain.Video) error {
	result, err := exec.ExecContext(ctx, `
		UPDATE videos
		SET status = $2,
		    raw_object_key = NULLIF($3, ''),
		    processed_object_key = NULLIF($4, ''),
		    thumbnail_object_key = NULLIF($5, ''),
		    content_type = NULLIF($6, ''),
		    size_bytes = $7,
		    duration_ms = $8,
		    width = $9,
		    height = $10,
		    processing_error_code = NULLIF($11, ''),
		    published_at = $12,
		    deleted_at = $13,
		    last_request_id = NULLIF($14, ''),
		    last_correlation_id = NULLIF($15, ''),
		    updated_at = $16
		WHERE id = $1
	`, video.ID, video.Status,
		video.RawObjectKey, video.ProcessedObjectKey, video.ThumbnailObjectKey,
		video.ContentType, nullableInt64(video.SizeBytes), nullableInt64(video.DurationMs), nullableInt(video.Width), nullableInt(video.Height),
		video.ProcessingErrorCode, video.PublishedAt, video.DeletedAt,
		video.LastRequestID, video.LastCorrelationID, video.UpdatedAt)
	if err != nil {
		return err
	}
	return requireRowsAffected(result, domain.NotFound(domain.CodeVideoNotFound, "Video was not found."))
}

func insertStatusHistory(ctx context.Context, exec sqlExecutor, history domain.StatusHistory) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO video_status_history (
			id, video_id, previous_status, new_status, reason, error_code,
			request_id, correlation_id, created_at
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9)
	`, history.ID, history.VideoID, history.PreviousStatus, history.NewStatus, history.Reason, history.ErrorCode,
		history.RequestID, history.CorrelationID, history.CreatedAt)
	return err
}

func insertOutboxEvent(ctx context.Context, exec sqlExecutor, event domain.OutboxEvent) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO outbox_events (
			id, event_name, event_version, aggregate_id, producer, environment,
			payload, status, request_id, correlation_id, occurred_at, published_at,
			created_at, attempts, last_error
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, NULLIF($9, ''), NULLIF($10, ''), $11, $12, $13, $14, NULLIF($15, ''))
	`, event.ID, event.EventName, event.EventVersion, event.AggregateID, event.Producer, event.Environment,
		string(event.Payload), event.Status, event.RequestID, event.CorrelationID, event.OccurredAt, event.PublishedAt,
		event.CreatedAt, event.Attempts, event.LastError)
	return err
}

func prefixedVideoColumns(alias string) string {
	columns := []string{
		"id", "owner_id", "title", "description", "status", "visibility",
		"raw_object_key", "processed_object_key", "thumbnail_object_key",
		"content_type", "size_bytes", "duration_ms", "width", "height",
		"processing_error_code", "published_at", "deleted_at",
		"last_request_id", "last_correlation_id",
		"created_at", "updated_at",
	}
	prefixed := make([]string, 0, len(columns))
	for _, column := range columns {
		prefixed = append(prefixed, alias+"."+column)
	}
	return strings.Join(prefixed, ", ")
}

func scanVideo(scanner sqlScanner) (domain.Video, error) {
	var video domain.Video
	var rawObjectKey sql.NullString
	var processedObjectKey sql.NullString
	var thumbnailObjectKey sql.NullString
	var contentType sql.NullString
	var sizeBytes sql.NullInt64
	var durationMs sql.NullInt64
	var width sql.NullInt64
	var height sql.NullInt64
	var processingErrorCode sql.NullString
	var publishedAt sql.NullTime
	var deletedAt sql.NullTime
	var lastRequestID sql.NullString
	var lastCorrelationID sql.NullString

	err := scanner.Scan(
		&video.ID, &video.OwnerID, &video.Title, &video.Description, &video.Status, &video.Visibility,
		&rawObjectKey, &processedObjectKey, &thumbnailObjectKey,
		&contentType, &sizeBytes, &durationMs, &width, &height,
		&processingErrorCode, &publishedAt, &deletedAt,
		&lastRequestID, &lastCorrelationID,
		&video.CreatedAt, &video.UpdatedAt,
	)
	if err != nil {
		return domain.Video{}, err
	}
	video.RawObjectKey = rawObjectKey.String
	video.ProcessedObjectKey = processedObjectKey.String
	video.ThumbnailObjectKey = thumbnailObjectKey.String
	video.ContentType = contentType.String
	video.SizeBytes = sizeBytes.Int64
	video.DurationMs = durationMs.Int64
	video.Width = int(width.Int64)
	video.Height = int(height.Int64)
	video.ProcessingErrorCode = processingErrorCode.String
	if publishedAt.Valid {
		video.PublishedAt = &publishedAt.Time
	}
	if deletedAt.Valid {
		video.DeletedAt = &deletedAt.Time
	}
	video.LastRequestID = lastRequestID.String
	video.LastCorrelationID = lastCorrelationID.String
	return video, nil
}

func scanUploadRequest(scanner sqlScanner) (domain.UploadRequest, error) {
	var upload domain.UploadRequest
	var idempotencyKey sql.NullString
	var sizeBytes sql.NullInt64
	var checksumSHA256 sql.NullString
	var completedAt sql.NullTime
	var requestID sql.NullString
	var correlationID sql.NullString

	err := scanner.Scan(
		&upload.ID, &upload.VideoID, &upload.OwnerID, &idempotencyKey, &upload.Bucket, &upload.ObjectKey,
		&upload.Status, &upload.ContentType, &sizeBytes, &checksumSHA256,
		&upload.ExpiresAt, &completedAt, &requestID, &correlationID,
		&upload.CreatedAt, &upload.UpdatedAt,
	)
	if err != nil {
		return domain.UploadRequest{}, err
	}
	upload.IdempotencyKey = idempotencyKey.String
	upload.SizeBytes = sizeBytes.Int64
	upload.ChecksumSHA256 = checksumSHA256.String
	if completedAt.Valid {
		upload.CompletedAt = &completedAt.Time
	}
	upload.RequestID = requestID.String
	upload.CorrelationID = correlationID.String
	return upload, nil
}

func scanVideoWithUpload(scanner sqlScanner) (domain.Video, domain.UploadRequest, error) {
	var video domain.Video
	var rawObjectKey sql.NullString
	var processedObjectKey sql.NullString
	var thumbnailObjectKey sql.NullString
	var videoContentType sql.NullString
	var videoSizeBytes sql.NullInt64
	var durationMs sql.NullInt64
	var width sql.NullInt64
	var height sql.NullInt64
	var processingErrorCode sql.NullString
	var publishedAt sql.NullTime
	var deletedAt sql.NullTime
	var lastRequestID sql.NullString
	var lastCorrelationID sql.NullString

	var upload domain.UploadRequest
	var idempotencyKey sql.NullString
	var uploadSizeBytes sql.NullInt64
	var checksumSHA256 sql.NullString
	var completedAt sql.NullTime
	var uploadRequestID sql.NullString
	var uploadCorrelationID sql.NullString

	err := scanner.Scan(
		&video.ID, &video.OwnerID, &video.Title, &video.Description, &video.Status, &video.Visibility,
		&rawObjectKey, &processedObjectKey, &thumbnailObjectKey,
		&videoContentType, &videoSizeBytes, &durationMs, &width, &height,
		&processingErrorCode, &publishedAt, &deletedAt,
		&lastRequestID, &lastCorrelationID,
		&video.CreatedAt, &video.UpdatedAt,
		&upload.ID, &upload.VideoID, &upload.OwnerID, &idempotencyKey, &upload.Bucket, &upload.ObjectKey,
		&upload.Status, &upload.ContentType, &uploadSizeBytes, &checksumSHA256,
		&upload.ExpiresAt, &completedAt, &uploadRequestID, &uploadCorrelationID,
		&upload.CreatedAt, &upload.UpdatedAt,
	)
	if err != nil {
		return domain.Video{}, domain.UploadRequest{}, err
	}
	video.RawObjectKey = rawObjectKey.String
	video.ProcessedObjectKey = processedObjectKey.String
	video.ThumbnailObjectKey = thumbnailObjectKey.String
	video.ContentType = videoContentType.String
	video.SizeBytes = videoSizeBytes.Int64
	video.DurationMs = durationMs.Int64
	video.Width = int(width.Int64)
	video.Height = int(height.Int64)
	video.ProcessingErrorCode = processingErrorCode.String
	if publishedAt.Valid {
		video.PublishedAt = &publishedAt.Time
	}
	if deletedAt.Valid {
		video.DeletedAt = &deletedAt.Time
	}
	video.LastRequestID = lastRequestID.String
	video.LastCorrelationID = lastCorrelationID.String

	upload.IdempotencyKey = idempotencyKey.String
	upload.SizeBytes = uploadSizeBytes.Int64
	upload.ChecksumSHA256 = checksumSHA256.String
	if completedAt.Valid {
		upload.CompletedAt = &completedAt.Time
	}
	upload.RequestID = uploadRequestID.String
	upload.CorrelationID = uploadCorrelationID.String
	return video, upload, nil
}

func scanOutboxEvent(scanner sqlScanner) (domain.OutboxEvent, error) {
	var event domain.OutboxEvent
	var requestID sql.NullString
	var correlationID sql.NullString
	var publishedAt sql.NullTime
	var lastError sql.NullString

	err := scanner.Scan(
		&event.ID, &event.EventName, &event.EventVersion, &event.AggregateID, &event.Producer, &event.Environment,
		&event.Payload, &event.Status, &requestID, &correlationID, &event.OccurredAt, &publishedAt,
		&event.CreatedAt, &event.Attempts, &lastError,
	)
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	event.RequestID = requestID.String
	event.CorrelationID = correlationID.String
	if publishedAt.Valid {
		event.PublishedAt = &publishedAt.Time
	}
	event.LastError = lastError.String
	return event, nil
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func requireRowsAffected(result sql.Result, notFound error) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return notFound
	}
	return nil
}

func nullableInt(value int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(value), Valid: value > 0}
}

func nullableInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: value > 0}
}
