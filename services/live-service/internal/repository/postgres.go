package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/domain"
)

const sessionColumns = `
	id, creator_id, title, description, status, stream_key_hash,
	ingest_path, playback_path, scheduled_at, started_at, ended_at,
	failure_code, last_request_id, last_correlation_id, created_at, updated_at
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

func (s *PostgresStore) CreateSession(ctx context.Context, session domain.LiveSession, key domain.StreamKey, event domain.LiveEvent) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertSession(ctx, tx, session); err != nil {
		return err
	}
	if err := insertStreamKey(ctx, tx, key); err != nil {
		return err
	}
	if err := insertLiveEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) FindSessionByID(ctx context.Context, id string) (domain.LiveSession, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+sessionColumns+`
		FROM live_sessions
		WHERE id = $1
	`, id)
	session, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.LiveSession{}, domain.NotFound(domain.CodeLiveSessionNotFound, "Live session was not found.")
	}
	if err != nil {
		return domain.LiveSession{}, err
	}
	return session, nil
}

func (s *PostgresStore) ListSessions(ctx context.Context, filter ListSessionsFilter) ([]domain.LiveSession, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 101 {
		limit = 101
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+sessionColumns+`
		FROM live_sessions
		WHERE ($1 = '' OR creator_id = $1)
		  AND ($2 = '' OR status = $2)
		  AND (
		    $3::timestamptz IS NULL
		    OR created_at < $3
		    OR (created_at = $3 AND id < $4)
		  )
		ORDER BY created_at DESC, id DESC
		LIMIT $5
	`, strings.TrimSpace(filter.CreatorID), strings.TrimSpace(filter.Status), nullableTime(filter.BeforeCreatedAt), strings.TrimSpace(filter.BeforeSessionID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make([]domain.LiveSession, 0, limit)
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (s *PostgresStore) UpdateSessionState(ctx context.Context, session domain.LiveSession, event domain.LiveEvent) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE live_sessions
		SET status = $2,
		    started_at = $3,
		    ended_at = $4,
		    failure_code = NULLIF($5, ''),
		    last_request_id = NULLIF($6, ''),
		    last_correlation_id = NULLIF($7, ''),
		    updated_at = $8
		WHERE id = $1
	`, session.ID, session.Status, session.StartedAt, session.EndedAt, session.FailureCode, session.LastRequestID, session.LastCorrelationID, session.UpdatedAt)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.NotFound(domain.CodeLiveSessionNotFound, "Live session was not found.")
	}
	if err := insertLiveEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func insertSession(ctx context.Context, tx *sql.Tx, session domain.LiveSession) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO live_sessions (
			id, creator_id, title, description, status, stream_key_hash,
			ingest_path, playback_path, scheduled_at, started_at, ended_at,
			failure_code, last_request_id, last_correlation_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULLIF($12, ''), NULLIF($13, ''), NULLIF($14, ''), $15, $16)
	`, session.ID, session.CreatorID, session.Title, session.Description, session.Status, session.StreamKeyHash,
		session.IngestPath, session.PlaybackPath, session.ScheduledAt, session.StartedAt, session.EndedAt,
		session.FailureCode, session.LastRequestID, session.LastCorrelationID, session.CreatedAt, session.UpdatedAt)
	return err
}

func insertStreamKey(ctx context.Context, tx *sql.Tx, key domain.StreamKey) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO stream_keys (id, live_session_id, key_hash, status, created_at, rotated_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, key.ID, key.LiveSessionID, key.KeyHash, key.Status, key.CreatedAt, key.RotatedAt, key.RevokedAt)
	return err
}

func insertLiveEvent(ctx context.Context, tx *sql.Tx, event domain.LiveEvent) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO live_events (id, live_session_id, event_type, payload, request_id, correlation_id, occurred_at)
		VALUES ($1, $2, $3, $4::jsonb, NULLIF($5, ''), NULLIF($6, ''), $7)
	`, event.ID, event.LiveSessionID, event.EventType, event.Payload, event.RequestID, event.CorrelationID, event.OccurredAt)
	return err
}

func scanSession(scanner sqlScanner) (domain.LiveSession, error) {
	var session domain.LiveSession
	var scheduledAt, startedAt, endedAt sql.NullTime
	var failureCode, requestID, correlationID sql.NullString
	err := scanner.Scan(
		&session.ID,
		&session.CreatorID,
		&session.Title,
		&session.Description,
		&session.Status,
		&session.StreamKeyHash,
		&session.IngestPath,
		&session.PlaybackPath,
		&scheduledAt,
		&startedAt,
		&endedAt,
		&failureCode,
		&requestID,
		&correlationID,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		return domain.LiveSession{}, err
	}
	if scheduledAt.Valid {
		session.ScheduledAt = &scheduledAt.Time
	}
	if startedAt.Valid {
		session.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}
	session.FailureCode = failureCode.String
	session.LastRequestID = requestID.String
	session.LastCorrelationID = correlationID.String
	return session, nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
