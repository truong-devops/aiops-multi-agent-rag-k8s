package service

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/repository"
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	JWKSURL      string
	Scopes       []string
}

type GoogleOAuthService struct {
	store  repository.Store
	config GoogleOAuthConfig
	client *http.Client
	now    func() time.Time
}

type GoogleStartResult struct {
	AuthorizationURL string `json:"authorization_url"`
	State            string `json:"state"`
	CodeVerifier     string `json:"code_verifier"`
}

type GoogleTokenInput struct {
	Code         string
	State        string
	CodeVerifier string
	RedirectURI  string
	UserAgent    string
	IPAddress    string
}

type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

func NewGoogleOAuthService(store repository.Store, cfg GoogleOAuthConfig) *GoogleOAuthService {
	return &GoogleOAuthService{
		store:  store,
		config: cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func (s *GoogleOAuthService) Start(ctx context.Context, redirectURI string) (GoogleStartResult, error) {
	if !s.configured() {
		return GoogleStartResult{}, domain.NewError(http.StatusServiceUnavailable, domain.CodeGoogleNotConfigured, "Google OAuth is not configured.")
	}
	redirectURI = strings.TrimSpace(redirectURI)
	if redirectURI == "" {
		return GoogleStartResult{}, domain.ValidationError("redirect_uri is required.")
	}

	now := s.now()
	state := domain.NewSecret("state", 24)
	nonce := domain.NewSecret("nonce", 24)
	codeVerifier := randomVerifier()
	challenge := codeChallenge(codeVerifier)
	oauthState := domain.OAuthState{
		State:        state,
		Provider:     domain.OAuthProviderGoogle,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		RedirectURI:  redirectURI,
		CreatedAt:    now,
		ExpiresAt:    now.Add(10 * time.Minute),
	}
	if err := s.store.SaveOAuthState(ctx, oauthState); err != nil {
		return GoogleStartResult{}, err
	}

	authURL, err := url.Parse(s.config.AuthURL)
	if err != nil {
		return GoogleStartResult{}, err
	}
	query := authURL.Query()
	query.Set("client_id", s.config.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(s.config.Scopes, " "))
	query.Set("state", state)
	query.Set("nonce", nonce)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("access_type", "offline")
	query.Set("prompt", "consent")
	authURL.RawQuery = query.Encode()

	return GoogleStartResult{
		AuthorizationURL: authURL.String(),
		State:            state,
		CodeVerifier:     codeVerifier,
	}, nil
}

func (s *GoogleOAuthService) Exchange(ctx context.Context, input GoogleTokenInput) (GoogleIdentity, error) {
	if !s.configured() {
		return GoogleIdentity{}, domain.NewError(http.StatusServiceUnavailable, domain.CodeGoogleNotConfigured, "Google OAuth is not configured.")
	}
	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.State) == "" || strings.TrimSpace(input.CodeVerifier) == "" || strings.TrimSpace(input.RedirectURI) == "" {
		return GoogleIdentity{}, domain.ValidationError("code, state, code_verifier and redirect_uri are required.")
	}

	state, err := s.store.ConsumeOAuthState(ctx, input.State, s.now())
	if err != nil || state.RedirectURI != input.RedirectURI || state.CodeVerifier != input.CodeVerifier {
		return GoogleIdentity{}, domain.NewError(http.StatusUnauthorized, domain.CodeGoogleStateInvalid, "Google OAuth state is invalid.")
	}

	idToken, err := s.exchangeCode(ctx, input.Code, input.CodeVerifier, input.RedirectURI)
	if err != nil {
		return GoogleIdentity{}, domain.NewError(http.StatusBadGateway, domain.CodeGoogleTokenExchangeFailed, "Google token exchange failed.")
	}
	identity, err := s.validateIDToken(ctx, idToken, state.Nonce)
	if err != nil {
		return GoogleIdentity{}, domain.NewError(http.StatusUnauthorized, domain.CodeGoogleIDTokenInvalid, "Google ID token is invalid.")
	}
	if !identity.EmailVerified {
		return GoogleIdentity{}, domain.NewError(http.StatusUnauthorized, domain.CodeGoogleEmailNotVerified, "Google email is not verified.")
	}
	return identity, nil
}

func (s *GoogleOAuthService) UpsertIdentity(ctx context.Context, identity GoogleIdentity) (domain.User, error) {
	now := s.now()
	user := domain.User{
		ID:            domain.NewID("usr"),
		Email:         identity.Email,
		DisplayName:   identity.Name,
		AvatarURL:     identity.Picture,
		Status:        domain.UserStatusActive,
		Roles:         []string{"user"},
		EmailVerified: identity.EmailVerified,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if user.DisplayName == "" {
		user.DisplayName = identity.Email
	}
	account := domain.OAuthAccount{
		ID:             domain.NewID("oauth"),
		Provider:       domain.OAuthProviderGoogle,
		ProviderUserID: identity.Subject,
		ProviderEmail:  identity.Email,
		EmailVerified:  identity.EmailVerified,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return s.store.UpsertOAuthUser(ctx, user, account)
}

func (s *GoogleOAuthService) configured() bool {
	return strings.TrimSpace(s.config.ClientID) != "" && strings.TrimSpace(s.config.ClientSecret) != ""
}

func (s *GoogleOAuthService) exchangeCode(ctx context.Context, code string, codeVerifier string, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", s.config.ClientID)
	form.Set("client_secret", s.config.ClientSecret)
	form.Set("code", code)
	form.Set("code_verifier", codeVerifier)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("google token endpoint status %d", resp.StatusCode)
	}

	var decoded struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}
	if decoded.IDToken == "" {
		return "", fmt.Errorf("google token response missing id_token")
	}
	return decoded.IDToken, nil
}

func (s *GoogleOAuthService) validateIDToken(ctx context.Context, idToken string, nonce string) (GoogleIdentity, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return GoogleIdentity{}, fmt.Errorf("id token must have three parts")
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}
	if err := decodeJWTPart(parts[0], &header); err != nil {
		return GoogleIdentity{}, err
	}
	if header.Algorithm != "RS256" || header.KeyID == "" {
		return GoogleIdentity{}, fmt.Errorf("untrusted google id token header")
	}
	if err := s.verifyGoogleSignature(ctx, header.KeyID, []byte(parts[0]+"."+parts[1]), parts[2]); err != nil {
		return GoogleIdentity{}, err
	}

	var claims struct {
		Issuer        string `json:"iss"`
		Audience      string `json:"aud"`
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		Nonce         string `json:"nonce"`
		ExpiresAt     int64  `json:"exp"`
		IssuedAt      int64  `json:"iat"`
	}
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return GoogleIdentity{}, err
	}
	now := s.now().Unix()
	if claims.Issuer != "https://accounts.google.com" && claims.Issuer != "accounts.google.com" {
		return GoogleIdentity{}, fmt.Errorf("invalid issuer")
	}
	if claims.Audience != s.config.ClientID {
		return GoogleIdentity{}, fmt.Errorf("invalid audience")
	}
	if claims.ExpiresAt <= now || claims.IssuedAt > now+300 {
		return GoogleIdentity{}, fmt.Errorf("invalid token time")
	}
	if claims.Nonce != nonce {
		return GoogleIdentity{}, fmt.Errorf("invalid nonce")
	}
	if claims.Subject == "" || claims.Email == "" {
		return GoogleIdentity{}, fmt.Errorf("missing subject or email")
	}
	return GoogleIdentity{
		Subject:       claims.Subject,
		Email:         strings.ToLower(claims.Email),
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		Picture:       claims.Picture,
	}, nil
}

func (s *GoogleOAuthService) verifyGoogleSignature(ctx context.Context, kid string, signed []byte, encodedSignature string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.config.JWKSURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("google jwks status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var jwks struct {
		Keys []struct {
			KID string `json:"kid"`
			KTY string `json:"kty"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&jwks); err != nil {
		return err
	}
	for _, key := range jwks.Keys {
		if key.KID != kid || key.KTY != "RSA" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			return err
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil {
			return err
		}
		e := int(new(big.Int).SetBytes(eBytes).Int64())
		publicKey := rsaPublicKey(nBytes, e)
		signature, err := base64.RawURLEncoding.DecodeString(encodedSignature)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(signed)
		return verifyRS256(publicKey, hash[:], signature)
	}
	return fmt.Errorf("google jwks key not found")
}

func randomVerifier() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return domain.NewSecret("verifier", 32)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func decodeJWTPart(segment string, out any) error {
	bytes, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, out)
}

func rsaPublicKey(nBytes []byte, exponent int) *rsa.PublicKey {
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: exponent,
	}
}

func verifyRS256(publicKey *rsa.PublicKey, hash []byte, signature []byte) error {
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash, signature)
}
