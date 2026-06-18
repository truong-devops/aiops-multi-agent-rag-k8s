package service

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/security"
)

type AuthService struct {
	store           repository.Store
	jwt             *security.JWTManager
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	now             func() time.Time
}

type RegisterInput struct {
	Email       string
	Username    string
	DisplayName string
	Password    string
	IPAddress   string
	UserAgent   string
}

type LoginInput struct {
	Email     string
	Password  string
	IPAddress string
	UserAgent string
}

type AuthResult struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
	User         domain.User
}

type RefreshResult struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
}

func NewAuthService(store repository.Store, jwt *security.JWTManager, accessTTL time.Duration, refreshTTL time.Duration) *AuthService {
	return &AuthService{
		store:           store,
		jwt:             jwt,
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
		now:             func() time.Time { return time.Now().UTC() },
	}
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (domain.User, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return domain.User{}, err
	}
	username := normalizeUsername(input.Username)
	displayName := strings.TrimSpace(input.DisplayName)
	if len(input.Password) < 8 {
		return domain.User{}, domain.NewError(http.StatusBadRequest, domain.CodeWeakPassword, "Password must be at least 8 characters.")
	}

	now := s.now()
	passwordHash, err := security.HashPassword(input.Password)
	if err != nil {
		return domain.User{}, err
	}
	user := domain.User{
		ID:            domain.NewID("usr"),
		Email:         email,
		Username:      username,
		DisplayName:   displayName,
		Status:        domain.UserStatusActive,
		Roles:         []string{"user"},
		EmailVerified: false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if user.DisplayName == "" {
		user.DisplayName = email
	}
	credential := domain.UserCredential{
		UserID:       user.ID,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	created, err := s.store.CreateUserWithCredential(ctx, user, credential)
	if errors.Is(err, repository.ErrEmailAlreadyExists) {
		_ = s.audit(ctx, domain.AuditLog{EventType: "user.registered", IPAddress: input.IPAddress, UserAgent: input.UserAgent, Success: false, ErrorCode: domain.CodeEmailAlreadyExists})
		return domain.User{}, domain.NewError(http.StatusConflict, domain.CodeEmailAlreadyExists, "Email already exists.")
	}
	if errors.Is(err, repository.ErrUsernameAlreadyExists) {
		_ = s.audit(ctx, domain.AuditLog{EventType: "user.registered", IPAddress: input.IPAddress, UserAgent: input.UserAgent, Success: false, ErrorCode: domain.CodeUsernameAlreadyExists})
		return domain.User{}, domain.NewError(http.StatusConflict, domain.CodeUsernameAlreadyExists, "Username already exists.")
	}
	if err != nil {
		return domain.User{}, err
	}
	_ = s.audit(ctx, domain.AuditLog{UserID: created.ID, EventType: "user.registered", IPAddress: input.IPAddress, UserAgent: input.UserAgent, Success: true})
	return created, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	user, credential, err := s.store.FindUserByEmail(ctx, input.Email)
	if err != nil || !security.VerifyPassword(input.Password, credential.PasswordHash) {
		_ = s.audit(ctx, domain.AuditLog{EventType: "auth.login_failed", IPAddress: input.IPAddress, UserAgent: input.UserAgent, Success: false, ErrorCode: domain.CodeInvalidCredentials})
		return AuthResult{}, domain.NewError(http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid email or password.")
	}
	if user.Status != domain.UserStatusActive {
		_ = s.audit(ctx, domain.AuditLog{UserID: user.ID, EventType: "auth.login_failed", IPAddress: input.IPAddress, UserAgent: input.UserAgent, Success: false, ErrorCode: domain.CodeUserDisabled})
		return AuthResult{}, domain.NewError(http.StatusForbidden, domain.CodeUserDisabled, "User account is disabled.")
	}

	result, sessionID, err := s.issueTokenPair(ctx, user, input.UserAgent, input.IPAddress)
	if err != nil {
		return AuthResult{}, err
	}
	_ = s.audit(ctx, domain.AuditLog{UserID: user.ID, SessionID: sessionID, EventType: "auth.login_succeeded", IPAddress: input.IPAddress, UserAgent: input.UserAgent, Success: true})
	return result, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (RefreshResult, error) {
	now := s.now()
	tokenHash := security.HashRefreshToken(refreshToken)
	token, session, user, err := s.store.FindRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return RefreshResult{}, domain.NewError(http.StatusUnauthorized, domain.CodeInvalidRefreshToken, "Refresh token is invalid.")
	}
	if token.Status != domain.RefreshTokenStatusActive {
		_ = s.store.MarkSessionCompromised(ctx, session.ID, now)
		_ = s.audit(ctx, domain.AuditLog{UserID: user.ID, SessionID: session.ID, EventType: "auth.refresh_reuse_detected", Success: false, ErrorCode: domain.CodeRefreshTokenReused})
		return RefreshResult{}, domain.NewError(http.StatusUnauthorized, domain.CodeRefreshTokenReused, "Refresh token was reused.")
	}
	if session.Status != domain.SessionStatusActive || now.After(session.ExpiresAt) {
		return RefreshResult{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionRevoked, "Session is revoked or expired.")
	}
	if now.After(token.ExpiresAt) {
		return RefreshResult{}, domain.NewError(http.StatusUnauthorized, domain.CodeInvalidRefreshToken, "Refresh token is expired.")
	}

	newRawToken := domain.NewSecret("rt", 32)
	newToken := domain.RefreshToken{
		ID:        domain.NewID("rft"),
		SessionID: session.ID,
		TokenHash: security.HashRefreshToken(newRawToken),
		Status:    domain.RefreshTokenStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(s.refreshTokenTTL),
	}
	if err := s.store.RotateRefreshToken(ctx, token.ID, newToken, now); err != nil {
		return RefreshResult{}, err
	}

	accessToken, _, err := s.jwt.SignAccessToken(security.SignInput{
		UserID:    user.ID,
		Email:     user.Email,
		Roles:     user.Roles,
		SessionID: session.ID,
		TTL:       s.accessTokenTTL,
	}, now)
	if err != nil {
		return RefreshResult{}, err
	}
	_ = s.audit(ctx, domain.AuditLog{UserID: user.ID, SessionID: session.ID, EventType: "auth.token_refreshed", Success: true})
	return RefreshResult{
		AccessToken:  accessToken,
		RefreshToken: newRawToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.accessTokenTTL.Seconds()),
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := security.HashRefreshToken(refreshToken)
	_, session, user, err := s.store.FindRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil
	}
	now := s.now()
	_ = s.store.RevokeSession(ctx, session.ID, now)
	_ = s.audit(ctx, domain.AuditLog{UserID: user.ID, SessionID: session.ID, EventType: "auth.logout", Success: true})
	return nil
}

func (s *AuthService) Authenticate(ctx context.Context, authorizationHeader string) (domain.User, security.AccessTokenClaims, error) {
	token := strings.TrimSpace(strings.TrimPrefix(authorizationHeader, "Bearer "))
	if token == "" || token == authorizationHeader {
		return domain.User{}, security.AccessTokenClaims{}, domain.Unauthorized("Missing bearer token.")
	}
	claims, err := s.jwt.VerifyAccessToken(token, s.now())
	if err != nil {
		return domain.User{}, security.AccessTokenClaims{}, domain.Unauthorized("Access token is invalid.")
	}
	user, err := s.store.FindUserByID(ctx, claims.Subject)
	if err != nil || user.Status != domain.UserStatusActive {
		return domain.User{}, security.AccessTokenClaims{}, domain.Unauthorized("User is not available.")
	}
	return user, claims, nil
}

func (s *AuthService) GetUser(ctx context.Context, userID string) (domain.User, error) {
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return domain.User{}, domain.NewError(http.StatusNotFound, "USER_NOT_FOUND", "User was not found.")
	}
	return user, nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID string, displayName string, avatarURL string) (domain.User, error) {
	user, err := s.store.UpdateUserProfile(ctx, userID, strings.TrimSpace(displayName), strings.TrimSpace(avatarURL))
	if err != nil {
		return domain.User{}, domain.NewError(http.StatusNotFound, "USER_NOT_FOUND", "User was not found.")
	}
	_ = s.audit(ctx, domain.AuditLog{UserID: user.ID, EventType: "user.updated", Success: true})
	return user, nil
}

func (s *AuthService) JWKS() map[string]any {
	return s.jwt.JWKS()
}

func (s *AuthService) IssueForOAuthUser(ctx context.Context, user domain.User, userAgent string, ipAddress string) (AuthResult, error) {
	result, sessionID, err := s.issueTokenPair(ctx, user, userAgent, ipAddress)
	if err != nil {
		return AuthResult{}, err
	}
	_ = s.audit(ctx, domain.AuditLog{UserID: user.ID, SessionID: sessionID, EventType: "auth.google_login_succeeded", Provider: domain.OAuthProviderGoogle, IPAddress: ipAddress, UserAgent: userAgent, Success: true})
	return result, nil
}

func (s *AuthService) issueTokenPair(ctx context.Context, user domain.User, userAgent string, ipAddress string) (AuthResult, string, error) {
	now := s.now()
	session := domain.Session{
		ID:         domain.NewID("sess"),
		UserID:     user.ID,
		UserAgent:  userAgent,
		IPAddress:  ipAddress,
		Status:     domain.SessionStatusActive,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(s.refreshTokenTTL),
	}
	rawRefreshToken := domain.NewSecret("rt", 32)
	refreshToken := domain.RefreshToken{
		ID:        domain.NewID("rft"),
		SessionID: session.ID,
		TokenHash: security.HashRefreshToken(rawRefreshToken),
		Status:    domain.RefreshTokenStatusActive,
		CreatedAt: now,
		ExpiresAt: session.ExpiresAt,
	}
	if err := s.store.CreateSession(ctx, session, refreshToken); err != nil {
		return AuthResult{}, "", err
	}

	accessToken, _, err := s.jwt.SignAccessToken(security.SignInput{
		UserID:    user.ID,
		Email:     user.Email,
		Roles:     user.Roles,
		SessionID: session.ID,
		TTL:       s.accessTokenTTL,
	}, now)
	if err != nil {
		return AuthResult{}, "", err
	}

	return AuthResult{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.accessTokenTTL.Seconds()),
		User:         user,
	}, session.ID, nil
}

func (s *AuthService) audit(ctx context.Context, log domain.AuditLog) error {
	log.ID = domain.NewID("aud")
	log.CreatedAt = s.now()
	return s.store.WriteAuditLog(ctx, log)
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", domain.ValidationError("Email is required.")
	}
	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", domain.ValidationError("Email is invalid.")
	}
	return normalized, nil
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
