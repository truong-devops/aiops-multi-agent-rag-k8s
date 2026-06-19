package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/domain"
)

const (
	userColumns = `id, email, username, display_name, avatar_url, status, roles, email_verified, created_at, updated_at`
)

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

func (s *PostgresStore) CreateUserWithCredential(ctx context.Context, user domain.User, credential domain.UserCredential) (domain.User, error) {
	user.Email = normalizeEmail(user.Email)
	user.Username = normalizeUsername(user.Username)

	roles, err := marshalRoles(user.Roles)
	if err != nil {
		return domain.User{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.User{}, err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, username, display_name, avatar_url, status, roles, email_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10)
	`, user.ID, user.Email, nullableString(user.Username), user.DisplayName, user.AvatarURL, user.Status, roles, user.EmailVerified, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return domain.User{}, mapPostgresUserConflict(err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_credentials (user_id, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
	`, credential.UserID, credential.PasswordHash, credential.CreatedAt, credential.UpdatedAt)
	if err != nil {
		return domain.User{}, mapPostgresUserConflict(err)
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, mapPostgresUserConflict(err)
	}
	return user, nil
}

func (s *PostgresStore) FindUserByEmail(ctx context.Context, email string) (domain.User, domain.UserCredential, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.username, u.display_name, u.avatar_url, u.status, u.roles, u.email_verified, u.created_at, u.updated_at,
		       c.user_id, c.password_hash, c.created_at, c.updated_at
		FROM users u
		JOIN user_credentials c ON c.user_id = u.id
		WHERE u.email = $1
	`, normalizeEmail(email))
	user, credential, err := scanUserWithCredential(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.UserCredential{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, domain.UserCredential{}, err
	}
	return user, credential, nil
}

func (s *PostgresStore) FindUserByID(ctx context.Context, userID string) (domain.User, error) {
	return s.findUserByID(ctx, s.db, userID)
}

func (s *PostgresStore) UpdateUserProfile(ctx context.Context, userID string, displayName string, avatarURL string) (domain.User, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE users
		SET display_name = CASE WHEN $2 <> '' THEN $2 ELSE display_name END,
		    avatar_url = CASE WHEN $3 <> '' THEN $3 ELSE avatar_url END,
		    updated_at = $4
		WHERE id = $1
		RETURNING `+userColumns+`
	`, userID, displayName, avatarURL, time.Now().UTC())
	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *PostgresStore) UpsertOAuthUser(ctx context.Context, user domain.User, account domain.OAuthAccount) (domain.User, error) {
	user.Email = normalizeEmail(user.Email)
	user.Username = normalizeUsername(user.Username)
	account.ProviderEmail = normalizeEmail(account.ProviderEmail)

	roles, err := marshalRoles(user.Roles)
	if err != nil {
		return domain.User{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.User{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var existingUserID string
	err = tx.QueryRowContext(ctx, `
		SELECT user_id
		FROM oauth_accounts
		WHERE provider = $1 AND provider_user_id = $2
		FOR UPDATE
	`, account.Provider, account.ProviderUserID).Scan(&existingUserID)
	if err == nil {
		existing, err := s.findUserByID(ctx, tx, existingUserID)
		if err != nil {
			return domain.User{}, err
		}
		if err := tx.Commit(); err != nil {
			return domain.User{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, username, display_name, avatar_url, status, roles, email_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10)
		ON CONFLICT (email) DO NOTHING
	`, user.ID, user.Email, nullableString(user.Username), user.DisplayName, user.AvatarURL, user.Status, roles, user.EmailVerified, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return domain.User{}, mapPostgresUserConflict(err)
	}

	currentUser, err := s.findUserByEmailOnly(ctx, tx, user.Email)
	if err != nil {
		return domain.User{}, err
	}
	account.UserID = currentUser.ID

	var linkedUserID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO oauth_accounts (id, user_id, provider, provider_user_id, provider_email, email_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (provider, provider_user_id)
		DO UPDATE SET provider_email = EXCLUDED.provider_email,
		              email_verified = EXCLUDED.email_verified,
		              updated_at = EXCLUDED.updated_at
		RETURNING user_id
	`, account.ID, account.UserID, account.Provider, account.ProviderUserID, account.ProviderEmail, account.EmailVerified, account.CreatedAt, account.UpdatedAt).Scan(&linkedUserID)
	if err != nil {
		return domain.User{}, err
	}

	linkedUser, err := s.findUserByID(ctx, tx, linkedUserID)
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return linkedUser, nil
}

func (s *PostgresStore) CreateSession(ctx context.Context, session domain.Session, refreshToken domain.RefreshToken) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertSession(ctx, tx, session); err != nil {
		return err
	}
	if err := insertRefreshToken(ctx, tx, refreshToken); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) FindSessionByID(ctx context.Context, sessionID string) (domain.Session, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, user_agent, ip_address, status, created_at, last_seen_at, expires_at, revoked_at
		FROM sessions
		WHERE id = $1
	`, sessionID)
	session, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, ErrNotFound
	}
	if err != nil {
		return domain.Session{}, err
	}
	return session, nil
}

func (s *PostgresStore) FindRefreshTokenByHash(ctx context.Context, tokenHash string) (domain.RefreshToken, domain.Session, domain.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT rt.id, rt.session_id, rt.token_hash, rt.status, rt.created_at, rt.expires_at, rt.used_at, rt.revoked_at, rt.replaced_by,
		       s.id, s.user_id, s.user_agent, s.ip_address, s.status, s.created_at, s.last_seen_at, s.expires_at, s.revoked_at,
		       u.id, u.email, u.username, u.display_name, u.avatar_url, u.status, u.roles, u.email_verified, u.created_at, u.updated_at
		FROM refresh_tokens rt
		JOIN sessions s ON s.id = rt.session_id
		JOIN users u ON u.id = s.user_id
		WHERE rt.token_hash = $1
	`, tokenHash)

	var token domain.RefreshToken
	var tokenUsedAt sql.NullTime
	var tokenRevokedAt sql.NullTime
	var tokenReplacedBy sql.NullString
	var session domain.Session
	var sessionRevokedAt sql.NullTime
	var user domain.User
	var username sql.NullString
	var rolesRaw []byte

	dest := refreshTokenScanDest(&token, &tokenUsedAt, &tokenRevokedAt, &tokenReplacedBy)
	dest = append(dest, sessionScanDest(&session, &sessionRevokedAt)...)
	dest = append(dest, userScanDest(&user, &username, &rolesRaw)...)
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.RefreshToken{}, domain.Session{}, domain.User{}, ErrNotFound
		}
		return domain.RefreshToken{}, domain.Session{}, domain.User{}, err
	}
	if err := finishRefreshTokenScan(&token, tokenUsedAt, tokenRevokedAt, tokenReplacedBy); err != nil {
		return domain.RefreshToken{}, domain.Session{}, domain.User{}, err
	}
	finishSessionScan(&session, sessionRevokedAt)
	if err := finishUserScan(&user, username, rolesRaw); err != nil {
		return domain.RefreshToken{}, domain.Session{}, domain.User{}, err
	}
	return token, session, user, nil
}

func (s *PostgresStore) RotateRefreshToken(ctx context.Context, oldTokenID string, newToken domain.RefreshToken, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var sessionID string
	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT session_id, status
		FROM refresh_tokens
		WHERE id = $1
		FOR UPDATE
	`, oldTokenID).Scan(&sessionID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != domain.RefreshTokenStatusActive {
		return ErrRefreshTokenNotActive
	}

	var sessionStatus string
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT status, expires_at
		FROM sessions
		WHERE id = $1
		FOR UPDATE
	`, sessionID).Scan(&sessionStatus, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if sessionStatus != domain.SessionStatusActive || now.After(expiresAt) {
		return ErrRefreshTokenNotActive
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE refresh_tokens
		SET status = $2, used_at = $3, replaced_by = $4
		WHERE id = $1
	`, oldTokenID, domain.RefreshTokenStatusUsed, now, nullableString(newToken.ID))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE sessions
		SET last_seen_at = $2
		WHERE id = $1
	`, sessionID, now)
	if err != nil {
		return err
	}
	if err := insertRefreshToken(ctx, tx, newToken); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) RevokeSession(ctx context.Context, sessionID string, now time.Time) error {
	return s.updateSessionStatus(ctx, sessionID, domain.SessionStatusRevoked, now)
}

func (s *PostgresStore) MarkSessionCompromised(ctx context.Context, sessionID string, now time.Time) error {
	return s.updateSessionStatus(ctx, sessionID, domain.SessionStatusCompromised, now)
}

func (s *PostgresStore) SaveOAuthState(ctx context.Context, state domain.OAuthState) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_states (state, provider, nonce, code_verifier, redirect_uri, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (state)
		DO UPDATE SET provider = EXCLUDED.provider,
		              nonce = EXCLUDED.nonce,
		              code_verifier = EXCLUDED.code_verifier,
		              redirect_uri = EXCLUDED.redirect_uri,
		              created_at = EXCLUDED.created_at,
		              expires_at = EXCLUDED.expires_at
	`, state.State, state.Provider, state.Nonce, state.CodeVerifier, state.RedirectURI, state.CreatedAt, state.ExpiresAt)
	return err
}

func (s *PostgresStore) ConsumeOAuthState(ctx context.Context, state string, now time.Time) (domain.OAuthState, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.OAuthState{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT state, provider, nonce, code_verifier, redirect_uri, created_at, expires_at
		FROM oauth_states
		WHERE state = $1
		FOR UPDATE
	`, state)
	oauthState, err := scanOAuthState(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OAuthState{}, ErrNotFound
	}
	if err != nil {
		return domain.OAuthState{}, err
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM oauth_states WHERE state = $1`, state)
	if err != nil {
		return domain.OAuthState{}, err
	}
	if now.After(oauthState.ExpiresAt) {
		if err := tx.Commit(); err != nil {
			return domain.OAuthState{}, err
		}
		return domain.OAuthState{}, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return domain.OAuthState{}, err
	}
	return oauthState, nil
}

func (s *PostgresStore) WriteAuditLog(ctx context.Context, log domain.AuditLog) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_audit_logs (id, user_id, session_id, event_type, provider, ip_address, user_agent, success, error_code, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, log.ID, nullableString(log.UserID), nullableString(log.SessionID), log.EventType, log.Provider, log.IPAddress, log.UserAgent, log.Success, nullableString(log.ErrorCode), log.CreatedAt)
	return err
}

func (s *PostgresStore) updateSessionStatus(ctx context.Context, sessionID string, status string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE sessions
		SET status = $2, revoked_at = $3
		WHERE id = $1
	`, sessionID, status, now)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE refresh_tokens
		SET status = $2, revoked_at = $3
		WHERE session_id = $1 AND status = $4
	`, sessionID, domain.RefreshTokenStatusRevoked, now, domain.RefreshTokenStatusActive)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) findUserByID(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, userID string) (domain.User, error) {
	row := queryer.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, userID)
	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *PostgresStore) findUserByEmailOnly(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, email string) (domain.User, error) {
	row := queryer.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, normalizeEmail(email))
	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func insertSession(ctx context.Context, tx *sql.Tx, session domain.Session) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, user_agent, ip_address, status, created_at, last_seen_at, expires_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, session.ID, session.UserID, session.UserAgent, session.IPAddress, session.Status, session.CreatedAt, session.LastSeenAt, session.ExpiresAt, session.RevokedAt)
	return err
}

func insertRefreshToken(ctx context.Context, tx *sql.Tx, token domain.RefreshToken) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO refresh_tokens (id, session_id, token_hash, status, created_at, expires_at, used_at, revoked_at, replaced_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, token.ID, token.SessionID, token.TokenHash, token.Status, token.CreatedAt, token.ExpiresAt, token.UsedAt, token.RevokedAt, nullableString(token.ReplacedBy))
	return err
}

func scanUser(scanner sqlScanner) (domain.User, error) {
	var user domain.User
	var username sql.NullString
	var rolesRaw []byte
	if err := scanner.Scan(userScanDest(&user, &username, &rolesRaw)...); err != nil {
		return domain.User{}, err
	}
	if err := finishUserScan(&user, username, rolesRaw); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func scanUserWithCredential(scanner sqlScanner) (domain.User, domain.UserCredential, error) {
	var user domain.User
	var username sql.NullString
	var rolesRaw []byte
	var credential domain.UserCredential
	dest := userScanDest(&user, &username, &rolesRaw)
	dest = append(dest, &credential.UserID, &credential.PasswordHash, &credential.CreatedAt, &credential.UpdatedAt)
	if err := scanner.Scan(dest...); err != nil {
		return domain.User{}, domain.UserCredential{}, err
	}
	if err := finishUserScan(&user, username, rolesRaw); err != nil {
		return domain.User{}, domain.UserCredential{}, err
	}
	return user, credential, nil
}

func userScanDest(user *domain.User, username *sql.NullString, rolesRaw *[]byte) []any {
	return []any{
		&user.ID,
		&user.Email,
		username,
		&user.DisplayName,
		&user.AvatarURL,
		&user.Status,
		rolesRaw,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
	}
}

func finishUserScan(user *domain.User, username sql.NullString, rolesRaw []byte) error {
	if username.Valid {
		user.Username = username.String
	}
	roles, err := unmarshalRoles(rolesRaw)
	if err != nil {
		return err
	}
	user.Roles = roles
	return nil
}

func scanSession(scanner sqlScanner) (domain.Session, error) {
	var session domain.Session
	var revokedAt sql.NullTime
	if err := scanner.Scan(sessionScanDest(&session, &revokedAt)...); err != nil {
		return domain.Session{}, err
	}
	finishSessionScan(&session, revokedAt)
	return session, nil
}

func sessionScanDest(session *domain.Session, revokedAt *sql.NullTime) []any {
	return []any{
		&session.ID,
		&session.UserID,
		&session.UserAgent,
		&session.IPAddress,
		&session.Status,
		&session.CreatedAt,
		&session.LastSeenAt,
		&session.ExpiresAt,
		revokedAt,
	}
}

func finishSessionScan(session *domain.Session, revokedAt sql.NullTime) {
	session.RevokedAt = nullTimePtr(revokedAt)
}

func refreshTokenScanDest(token *domain.RefreshToken, usedAt *sql.NullTime, revokedAt *sql.NullTime, replacedBy *sql.NullString) []any {
	return []any{
		&token.ID,
		&token.SessionID,
		&token.TokenHash,
		&token.Status,
		&token.CreatedAt,
		&token.ExpiresAt,
		usedAt,
		revokedAt,
		replacedBy,
	}
}

func finishRefreshTokenScan(token *domain.RefreshToken, usedAt sql.NullTime, revokedAt sql.NullTime, replacedBy sql.NullString) error {
	token.UsedAt = nullTimePtr(usedAt)
	token.RevokedAt = nullTimePtr(revokedAt)
	if replacedBy.Valid {
		token.ReplacedBy = replacedBy.String
	}
	return nil
}

func scanOAuthState(scanner sqlScanner) (domain.OAuthState, error) {
	var state domain.OAuthState
	err := scanner.Scan(
		&state.State,
		&state.Provider,
		&state.Nonce,
		&state.CodeVerifier,
		&state.RedirectURI,
		&state.CreatedAt,
		&state.ExpiresAt,
	)
	if err != nil {
		return domain.OAuthState{}, err
	}
	return state, nil
}

func marshalRoles(roles []string) ([]byte, error) {
	if len(roles) == 0 {
		roles = []string{"user"}
	}
	return json.Marshal(roles)
}

func unmarshalRoles(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var roles []string
	if err := json.Unmarshal(raw, &roles); err != nil {
		return nil, fmt.Errorf("decode user roles: %w", err)
	}
	if roles == nil {
		return []string{}, nil
	}
	return roles, nil
}

func nullableString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func mapPostgresUserConflict(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}
	switch pgErr.ConstraintName {
	case "users_email_unique":
		return ErrEmailAlreadyExists
	case "users_username_unique":
		return ErrUsernameAlreadyExists
	default:
		return err
	}
}
