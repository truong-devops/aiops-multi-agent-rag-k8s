package repository

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/domain"
)

var (
	ErrNotFound              = errors.New("not found")
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUsernameAlreadyExists = errors.New("username already exists")
)

type UserRepository interface {
	CreateUserWithCredential(ctx context.Context, user domain.User, credential domain.UserCredential) (domain.User, error)
	FindUserByEmail(ctx context.Context, email string) (domain.User, domain.UserCredential, error)
	FindUserByID(ctx context.Context, userID string) (domain.User, error)
	UpdateUserProfile(ctx context.Context, userID string, displayName string, avatarURL string) (domain.User, error)
	UpsertOAuthUser(ctx context.Context, user domain.User, account domain.OAuthAccount) (domain.User, error)
}

type SessionRepository interface {
	CreateSession(ctx context.Context, session domain.Session, refreshToken domain.RefreshToken) error
	FindRefreshTokenByHash(ctx context.Context, tokenHash string) (domain.RefreshToken, domain.Session, domain.User, error)
	RotateRefreshToken(ctx context.Context, oldTokenID string, newToken domain.RefreshToken, now time.Time) error
	RevokeSession(ctx context.Context, sessionID string, now time.Time) error
	MarkSessionCompromised(ctx context.Context, sessionID string, now time.Time) error
}

type OAuthStateRepository interface {
	SaveOAuthState(ctx context.Context, state domain.OAuthState) error
	ConsumeOAuthState(ctx context.Context, state string, now time.Time) (domain.OAuthState, error)
}

type AuditRepository interface {
	WriteAuditLog(ctx context.Context, log domain.AuditLog) error
}

type Store interface {
	UserRepository
	SessionRepository
	OAuthStateRepository
	AuditRepository
}

type MemoryStore struct {
	mu sync.RWMutex

	usersByID        map[string]domain.User
	userCredentials  map[string]domain.UserCredential
	userIDByEmail    map[string]string
	userIDByUsername map[string]string

	oauthAccountsByProviderID map[string]domain.OAuthAccount

	sessionsByID         map[string]domain.Session
	refreshTokensByID    map[string]domain.RefreshToken
	refreshTokenIDByHash map[string]string
	oauthStatesByState   map[string]domain.OAuthState
	auditLogs            []domain.AuditLog
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		usersByID:                 map[string]domain.User{},
		userCredentials:           map[string]domain.UserCredential{},
		userIDByEmail:             map[string]string{},
		userIDByUsername:          map[string]string{},
		oauthAccountsByProviderID: map[string]domain.OAuthAccount{},
		sessionsByID:              map[string]domain.Session{},
		refreshTokensByID:         map[string]domain.RefreshToken{},
		refreshTokenIDByHash:      map[string]string{},
		oauthStatesByState:        map[string]domain.OAuthState{},
		auditLogs:                 []domain.AuditLog{},
	}
}

func (s *MemoryStore) CreateUserWithCredential(_ context.Context, user domain.User, credential domain.UserCredential) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	email := normalizeEmail(user.Email)
	if _, exists := s.userIDByEmail[email]; exists {
		return domain.User{}, ErrEmailAlreadyExists
	}
	username := normalizeUsername(user.Username)
	if username != "" {
		if _, exists := s.userIDByUsername[username]; exists {
			return domain.User{}, ErrUsernameAlreadyExists
		}
	}

	user.Email = email
	user.Username = username
	s.usersByID[user.ID] = user
	s.userCredentials[user.ID] = credential
	s.userIDByEmail[email] = user.ID
	if username != "" {
		s.userIDByUsername[username] = user.ID
	}
	return user, nil
}

func (s *MemoryStore) FindUserByEmail(_ context.Context, email string) (domain.User, domain.UserCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, exists := s.userIDByEmail[normalizeEmail(email)]
	if !exists {
		return domain.User{}, domain.UserCredential{}, ErrNotFound
	}
	user := s.usersByID[userID]
	credential, exists := s.userCredentials[userID]
	if !exists {
		return domain.User{}, domain.UserCredential{}, ErrNotFound
	}
	return user, credential, nil
}

func (s *MemoryStore) FindUserByID(_ context.Context, userID string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.usersByID[userID]
	if !exists {
		return domain.User{}, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStore) UpdateUserProfile(_ context.Context, userID string, displayName string, avatarURL string) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.usersByID[userID]
	if !exists {
		return domain.User{}, ErrNotFound
	}
	if displayName != "" {
		user.DisplayName = displayName
	}
	if avatarURL != "" {
		user.AvatarURL = avatarURL
	}
	user.UpdatedAt = time.Now().UTC()
	s.usersByID[userID] = user
	return user, nil
}

func (s *MemoryStore) UpsertOAuthUser(_ context.Context, user domain.User, account domain.OAuthAccount) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := oauthKey(account.Provider, account.ProviderUserID)
	if existingAccount, exists := s.oauthAccountsByProviderID[key]; exists {
		existingUser := s.usersByID[existingAccount.UserID]
		return existingUser, nil
	}

	email := normalizeEmail(user.Email)
	if existingID, exists := s.userIDByEmail[email]; exists {
		account.UserID = existingID
		account.ID = domain.NewID("oauth")
		s.oauthAccountsByProviderID[key] = account
		return s.usersByID[existingID], nil
	}

	user.Email = email
	user.Username = normalizeUsername(user.Username)
	s.usersByID[user.ID] = user
	s.userIDByEmail[email] = user.ID
	if user.Username != "" {
		s.userIDByUsername[user.Username] = user.ID
	}
	account.UserID = user.ID
	s.oauthAccountsByProviderID[key] = account
	return user, nil
}

func (s *MemoryStore) CreateSession(_ context.Context, session domain.Session, refreshToken domain.RefreshToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessionsByID[session.ID] = session
	s.refreshTokensByID[refreshToken.ID] = refreshToken
	s.refreshTokenIDByHash[refreshToken.TokenHash] = refreshToken.ID
	return nil
}

func (s *MemoryStore) FindRefreshTokenByHash(_ context.Context, tokenHash string) (domain.RefreshToken, domain.Session, domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tokenID, exists := s.refreshTokenIDByHash[tokenHash]
	if !exists {
		return domain.RefreshToken{}, domain.Session{}, domain.User{}, ErrNotFound
	}
	token := s.refreshTokensByID[tokenID]
	session, exists := s.sessionsByID[token.SessionID]
	if !exists {
		return domain.RefreshToken{}, domain.Session{}, domain.User{}, ErrNotFound
	}
	user, exists := s.usersByID[session.UserID]
	if !exists {
		return domain.RefreshToken{}, domain.Session{}, domain.User{}, ErrNotFound
	}
	return token, session, user, nil
}

func (s *MemoryStore) RotateRefreshToken(_ context.Context, oldTokenID string, newToken domain.RefreshToken, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldToken, exists := s.refreshTokensByID[oldTokenID]
	if !exists {
		return ErrNotFound
	}
	oldToken.Status = domain.RefreshTokenStatusUsed
	oldToken.UsedAt = &now
	oldToken.ReplacedBy = newToken.ID
	s.refreshTokensByID[oldTokenID] = oldToken

	session := s.sessionsByID[oldToken.SessionID]
	session.LastSeenAt = now
	s.sessionsByID[session.ID] = session

	s.refreshTokensByID[newToken.ID] = newToken
	s.refreshTokenIDByHash[newToken.TokenHash] = newToken.ID
	return nil
}

func (s *MemoryStore) RevokeSession(_ context.Context, sessionID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessionsByID[sessionID]
	if !exists {
		return ErrNotFound
	}
	session.Status = domain.SessionStatusRevoked
	session.RevokedAt = &now
	s.sessionsByID[sessionID] = session

	for tokenID, token := range s.refreshTokensByID {
		if token.SessionID != sessionID || token.Status != domain.RefreshTokenStatusActive {
			continue
		}
		token.Status = domain.RefreshTokenStatusRevoked
		token.RevokedAt = &now
		s.refreshTokensByID[tokenID] = token
	}
	return nil
}

func (s *MemoryStore) MarkSessionCompromised(_ context.Context, sessionID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessionsByID[sessionID]
	if !exists {
		return ErrNotFound
	}
	session.Status = domain.SessionStatusCompromised
	session.RevokedAt = &now
	s.sessionsByID[sessionID] = session
	return nil
}

func (s *MemoryStore) SaveOAuthState(_ context.Context, state domain.OAuthState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.oauthStatesByState[state.State] = state
	return nil
}

func (s *MemoryStore) ConsumeOAuthState(_ context.Context, state string, now time.Time) (domain.OAuthState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	oauthState, exists := s.oauthStatesByState[state]
	if !exists || now.After(oauthState.ExpiresAt) {
		return domain.OAuthState{}, ErrNotFound
	}
	delete(s.oauthStatesByState, state)
	return oauthState, nil
}

func (s *MemoryStore) WriteAuditLog(_ context.Context, log domain.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.auditLogs = append(s.auditLogs, log)
	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func oauthKey(provider string, providerUserID string) string {
	return provider + ":" + providerUserID
}
