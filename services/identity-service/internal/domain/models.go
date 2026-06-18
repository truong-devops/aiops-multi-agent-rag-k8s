package domain

import "time"

const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"

	SessionStatusActive      = "active"
	SessionStatusRevoked     = "revoked"
	SessionStatusCompromised = "compromised"

	RefreshTokenStatusActive  = "active"
	RefreshTokenStatusUsed    = "used"
	RefreshTokenStatusRevoked = "revoked"

	OAuthProviderGoogle = "google"
)

type User struct {
	ID            string
	Email         string
	Username      string
	DisplayName   string
	AvatarURL     string
	Status        string
	Roles         []string
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type UserCredential struct {
	UserID       string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID         string
	UserID     string
	UserAgent  string
	IPAddress  string
	Status     string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
}

type RefreshToken struct {
	ID         string
	SessionID  string
	TokenHash  string
	Status     string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	UsedAt     *time.Time
	RevokedAt  *time.Time
	ReplacedBy string
}

type OAuthAccount struct {
	ID             string
	UserID         string
	Provider       string
	ProviderUserID string
	ProviderEmail  string
	EmailVerified  bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OAuthState struct {
	State        string
	Provider     string
	Nonce        string
	CodeVerifier string
	RedirectURI  string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

type AuditLog struct {
	ID        string
	UserID    string
	SessionID string
	EventType string
	Provider  string
	IPAddress string
	UserAgent string
	Success   bool
	ErrorCode string
	CreatedAt time.Time
}
