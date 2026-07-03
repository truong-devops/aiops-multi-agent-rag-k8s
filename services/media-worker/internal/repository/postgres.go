package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

const jobColumns = `
	id, video_id, owner_id, input_bucket, input_object_key, content_type,
	size_bytes, status, priority, attempt_count, max_attempts,
	locked_by, locked_until, next_run_at, started_at, completed_at,
	error_code, error_message, request_id, correlation_id, created_at, updated_at
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

func (s *PostgresStore) CreateJobFromUploadedEvent(ctx context.Context, event domain.UploadedVideoEvent, job domain.ProcessingJob) (domain.ProcessingJob, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.ProcessingJob{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	processedAt := job.CreatedAt
	result, err := tx.ExecContext(ctx, `
		INSERT INTO inbox_events (
			id, event_name, event_version, aggregate_id, status,
			request_id, correlation_id, received_at, processed_at
		)
		VALUES ($1, 'video.uploaded', 'v1', $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, $7)
		ON CONFLICT (id) DO NOTHING
	`, event.EventID, event.VideoID, domain.InboxStatusProcessed, event.RequestID, event.CorrelationID, event.ReceivedAt, processedAt)
	if err != nil {
		return domain.ProcessingJob{}, false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return domain.ProcessingJob{}, false, err
	}
	if rows == 0 {
		existing, err := findJobByVideoID(ctx, tx, event.VideoID)
		if err != nil {
			return domain.ProcessingJob{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return domain.ProcessingJob{}, false, err
		}
		return existing, false, nil
	}

	result, err = insertJob(ctx, tx, job)
	if err != nil {
		return domain.ProcessingJob{}, false, err
	}
	rows, err = result.RowsAffected()
	if err != nil {
		return domain.ProcessingJob{}, false, err
	}
	if rows == 0 {
		existing, err := findJobByVideoID(ctx, tx, job.VideoID)
		if err != nil {
			return domain.ProcessingJob{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return domain.ProcessingJob{}, false, err
		}
		return existing, false, nil
	}
	if err := tx.Commit(); err != nil {
		return domain.ProcessingJob{}, false, err
	}
	return job, true, nil
}

func (s *PostgresStore) FindJobByID(ctx context.Context, id string) (domain.ProcessingJob, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM processing_jobs WHERE id = $1`, id)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProcessingJob{}, domain.NotFound(domain.CodeJobNotFound, "Processing job was not found.")
	}
	return job, err
}

func (s *PostgresStore) FindJobByVideoID(ctx context.Context, videoID string) (domain.ProcessingJob, error) {
	return findJobByVideoID(ctx, s.db, videoID)
}

func (s *PostgresStore) ListJobs(ctx context.Context, filter ListJobsFilter) ([]domain.ProcessingJob, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+jobColumns+`
		FROM processing_jobs
		WHERE ($1 = '' OR video_id = $1)
		  AND ($2 = '' OR status = $2)
		ORDER BY priority DESC, created_at ASC
		LIMIT $3
	`, strings.TrimSpace(filter.VideoID), strings.TrimSpace(filter.Status), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]domain.ProcessingJob, 0)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *PostgresStore) ClaimRunnableJobs(ctx context.Context, workerID string, now time.Time, leaseTTL time.Duration, limit int) ([]domain.ProcessingJob, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	lockedUntil := now.UTC().Add(leaseTTL)
	rows, err := s.db.QueryContext(ctx, `
		WITH candidate AS (
			SELECT id
			FROM processing_jobs
			WHERE status IN ($2, $3)
			  AND next_run_at <= $1
			  AND (locked_until IS NULL OR locked_until <= $1)
			ORDER BY priority DESC, next_run_at ASC
			LIMIT $5
			FOR UPDATE SKIP LOCKED
		)
		UPDATE processing_jobs
		SET locked_by = $4,
		    locked_until = $6,
		    updated_at = $1
		WHERE id IN (SELECT id FROM candidate)
		RETURNING `+jobColumns+`
	`, now.UTC(), domain.JobStatusQueued, domain.JobStatusRetrying, workerID, limit, lockedUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]domain.ProcessingJob, 0)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *PostgresStore) StartAttempt(ctx context.Context, jobID string, workerID string, now time.Time) (domain.ProcessingJob, domain.ProcessingAttempt, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.ProcessingJob{}, domain.ProcessingAttempt{}, err
	}
	defer func() { _ = tx.Rollback() }()

	job, err := findJobByIDForUpdate(ctx, tx, jobID)
	if err != nil {
		return domain.ProcessingJob{}, domain.ProcessingAttempt{}, err
	}
	if job.Status != domain.JobStatusQueued && job.Status != domain.JobStatusRetrying && job.Status != domain.JobStatusRunning {
		return domain.ProcessingJob{}, domain.ProcessingAttempt{}, domain.Conflict(domain.CodeInvalidJobState, "Processing job cannot start an attempt.")
	}
	attemptNo := job.AttemptCount + 1
	attempt := domain.ProcessingAttempt{
		ID:        domain.NewID("att"),
		JobID:     job.ID,
		VideoID:   job.VideoID,
		AttemptNo: attemptNo,
		WorkerID:  workerID,
		Status:    domain.AttemptStatusRunning,
		StartedAt: now.UTC(),
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
		Metrics:   []byte(`{}`),
	}
	row := tx.QueryRowContext(ctx, `
		UPDATE processing_jobs
		SET status = $2,
		    attempt_count = $3,
		    locked_by = $4,
		    started_at = COALESCE(started_at, $5),
		    updated_at = $5
		WHERE id = $1
		RETURNING `+jobColumns+`
	`, job.ID, domain.JobStatusRunning, attemptNo, workerID, now.UTC())
	updated, err := scanJob(row)
	if err != nil {
		return domain.ProcessingJob{}, domain.ProcessingAttempt{}, err
	}
	if err := insertAttempt(ctx, tx, attempt); err != nil {
		return domain.ProcessingJob{}, domain.ProcessingAttempt{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ProcessingJob{}, domain.ProcessingAttempt{}, err
	}
	return updated, attempt, nil
}

func (s *PostgresStore) MarkAttemptSucceeded(ctx context.Context, jobID string, attemptID string, now time.Time, metrics []byte) (domain.ProcessingJob, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.ProcessingJob{}, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE processing_attempts
		SET status = $3,
		    finished_at = $4,
		    metrics = $5::jsonb,
		    updated_at = $4
		WHERE id = $1 AND job_id = $2
	`, attemptID, jobID, domain.AttemptStatusSucceeded, now.UTC(), string(metricsOrEmpty(metrics)))
	if err != nil {
		return domain.ProcessingJob{}, err
	}
	if err := requireRowsAffected(result, domain.NotFound(domain.CodeAttemptNotFound, "Processing attempt was not found.")); err != nil {
		return domain.ProcessingJob{}, err
	}
	row := tx.QueryRowContext(ctx, `
		UPDATE processing_jobs
		SET status = $2,
		    completed_at = $3,
		    locked_by = NULL,
		    locked_until = NULL,
		    updated_at = $3
		WHERE id = $1
		RETURNING `+jobColumns+`
	`, jobID, domain.JobStatusSucceeded, now.UTC())
	job, err := scanJob(row)
	if err != nil {
		return domain.ProcessingJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ProcessingJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) MarkAttemptFailed(ctx context.Context, jobID string, attemptID string, now time.Time, errorCode string, errorMessage string, retryAt *time.Time, deadLetter *domain.DeadLetter) (domain.ProcessingJob, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.ProcessingJob{}, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE processing_attempts
		SET status = $3,
		    finished_at = $4,
		    error_code = NULLIF($5, ''),
		    stderr_excerpt = NULLIF($6, ''),
		    updated_at = $4
		WHERE id = $1 AND job_id = $2
	`, attemptID, jobID, domain.AttemptStatusFailed, now.UTC(), errorCode, truncate(errorMessage, 2000))
	if err != nil {
		return domain.ProcessingJob{}, err
	}
	if err := requireRowsAffected(result, domain.NotFound(domain.CodeAttemptNotFound, "Processing attempt was not found.")); err != nil {
		return domain.ProcessingJob{}, err
	}

	status := domain.JobStatusFailed
	var completedAt any = now.UTC()
	var nextRunAt any = now.UTC()
	if retryAt != nil {
		status = domain.JobStatusRetrying
		completedAt = nil
		nextRunAt = retryAt.UTC()
	}
	if deadLetter != nil {
		status = domain.JobStatusDeadLetter
		if err := insertDeadLetter(ctx, tx, *deadLetter); err != nil {
			return domain.ProcessingJob{}, err
		}
	}
	row := tx.QueryRowContext(ctx, `
		UPDATE processing_jobs
		SET status = $2,
		    next_run_at = $3,
		    completed_at = $4,
		    error_code = NULLIF($5, ''),
		    error_message = NULLIF($6, ''),
		    locked_by = NULL,
		    locked_until = NULL,
		    updated_at = $7
		WHERE id = $1
		RETURNING `+jobColumns+`
	`, jobID, status, nextRunAt, completedAt, errorCode, truncate(errorMessage, 500), now.UTC())
	job, err := scanJob(row)
	if err != nil {
		return domain.ProcessingJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ProcessingJob{}, err
	}
	return job, nil
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertJob(ctx context.Context, exec sqlExecutor, job domain.ProcessingJob) (sql.Result, error) {
	return exec.ExecContext(ctx, `
		INSERT INTO processing_jobs (
			id, video_id, owner_id, input_bucket, input_object_key, content_type,
			size_bytes, status, priority, attempt_count, max_attempts,
			locked_by, locked_until, next_run_at, started_at, completed_at,
			error_code, error_message, request_id, correlation_id, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			NULLIF($12, ''), $13, $14, $15, $16,
			NULLIF($17, ''), NULLIF($18, ''), NULLIF($19, ''), NULLIF($20, ''), $21, $22
		)
		ON CONFLICT (video_id) DO NOTHING
	`, job.ID, job.VideoID, job.OwnerID, job.InputBucket, job.InputObjectKey, job.ContentType,
		nullableInt64(job.SizeBytes), job.Status, job.Priority, job.AttemptCount, job.MaxAttempts,
		job.LockedBy, job.LockedUntil, job.NextRunAt, job.StartedAt, job.CompletedAt,
		job.ErrorCode, job.ErrorMessage, job.RequestID, job.CorrelationID, job.CreatedAt, job.UpdatedAt)
}

func insertAttempt(ctx context.Context, exec sqlExecutor, attempt domain.ProcessingAttempt) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO processing_attempts (
			id, job_id, video_id, attempt_no, worker_id, status,
			ffmpeg_command_hash, started_at, finished_at, exit_code,
			error_code, stderr_excerpt, metrics, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9, $10, NULLIF($11, ''), NULLIF($12, ''), $13::jsonb, $14, $15)
	`, attempt.ID, attempt.JobID, attempt.VideoID, attempt.AttemptNo, attempt.WorkerID, attempt.Status,
		attempt.FFmpegCommandHash, attempt.StartedAt, attempt.FinishedAt, nullableIntPtr(attempt.ExitCode),
		attempt.ErrorCode, attempt.StderrExcerpt, string(metricsOrEmpty(attempt.Metrics)), attempt.CreatedAt, attempt.UpdatedAt)
	return err
}

func insertDeadLetter(ctx context.Context, exec sqlExecutor, deadLetter domain.DeadLetter) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO dead_letters (
			id, job_id, video_id, reason_code, payload, request_id, correlation_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, NULLIF($6, ''), NULLIF($7, ''), $8)
	`, deadLetter.ID, deadLetter.JobID, deadLetter.VideoID, deadLetter.ReasonCode,
		string(metricsOrEmpty(deadLetter.Payload)), deadLetter.RequestID, deadLetter.CorrelationID, deadLetter.CreatedAt)
	return err
}

func findJobByIDForUpdate(ctx context.Context, tx *sql.Tx, id string) (domain.ProcessingJob, error) {
	row := tx.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM processing_jobs WHERE id = $1 FOR UPDATE`, id)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProcessingJob{}, domain.NotFound(domain.CodeJobNotFound, "Processing job was not found.")
	}
	return job, err
}

func findJobByVideoID(ctx context.Context, q queryer, videoID string) (domain.ProcessingJob, error) {
	row := q.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM processing_jobs WHERE video_id = $1`, videoID)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProcessingJob{}, domain.NotFound(domain.CodeJobNotFound, "Processing job was not found.")
	}
	return job, err
}

func scanJob(scanner sqlScanner) (domain.ProcessingJob, error) {
	var job domain.ProcessingJob
	var sizeBytes sql.NullInt64
	var lockedBy sql.NullString
	var lockedUntil sql.NullTime
	var startedAt sql.NullTime
	var completedAt sql.NullTime
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var requestID sql.NullString
	var correlationID sql.NullString
	err := scanner.Scan(
		&job.ID, &job.VideoID, &job.OwnerID, &job.InputBucket, &job.InputObjectKey, &job.ContentType,
		&sizeBytes, &job.Status, &job.Priority, &job.AttemptCount, &job.MaxAttempts,
		&lockedBy, &lockedUntil, &job.NextRunAt, &startedAt, &completedAt,
		&errorCode, &errorMessage, &requestID, &correlationID, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return domain.ProcessingJob{}, err
	}
	job.SizeBytes = sizeBytes.Int64
	job.LockedBy = lockedBy.String
	if lockedUntil.Valid {
		job.LockedUntil = &lockedUntil.Time
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}
	job.ErrorCode = errorCode.String
	job.ErrorMessage = errorMessage.String
	job.RequestID = requestID.String
	job.CorrelationID = correlationID.String
	return job, nil
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

func nullableInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: value > 0}
}

func nullableIntPtr(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
