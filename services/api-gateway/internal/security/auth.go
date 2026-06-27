package security

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Claims struct {
	Issuer    string   `json:"iss"`
	Audience  string   `json:"aud"`
	Subject   string   `json:"sub"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
	SessionID string   `json:"sid"`
	TokenID   string   `json:"jti"`
	IssuedAt  int64    `json:"iat"`
	NotBefore int64    `json:"nbf"`
	ExpiresAt int64    `json:"exp"`
}

type Verifier interface {
	VerifyAccessToken(ctx context.Context, token string) (Claims, error)
}

type ReadinessChecker interface {
	Ready(ctx context.Context) error
}

type JWTVerifier struct {
	jwksURL  string
	issuer   string
	audience string
	cacheTTL time.Duration
	client   *http.Client
	now      func() time.Time

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

func NewJWTVerifier(jwksURL string, issuer string, audience string, cacheTTL time.Duration, timeout time.Duration) (*JWTVerifier, error) {
	if strings.TrimSpace(jwksURL) == "" {
		return nil, errors.New("JWKS_URL is required when JWT verification is enabled")
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, errors.New("JWT_ISSUER is required when JWT verification is enabled")
	}
	if strings.TrimSpace(audience) == "" {
		return nil, errors.New("JWT_AUDIENCE is required when JWT verification is enabled")
	}
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &JWTVerifier{
		jwksURL:  jwksURL,
		issuer:   issuer,
		audience: audience,
		cacheTTL: cacheTTL,
		client:   &http.Client{Timeout: timeout},
		now:      time.Now,
		keys:     map[string]*rsa.PublicKey{},
	}, nil
}

func (v *JWTVerifier) Ready(ctx context.Context) error {
	if _, err := v.currentKeys(ctx); err != nil {
		return fmt.Errorf("jwks not ready: %w", err)
	}
	return nil
}

func (v *JWTVerifier) VerifyAccessToken(ctx context.Context, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("jwt must have three parts")
	}

	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return Claims{}, fmt.Errorf("decode jwt header: %w", err)
	}
	if header.Algorithm != "RS256" || header.KeyID == "" {
		return Claims{}, errors.New("jwt header is not trusted")
	}

	keys, err := v.currentKeys(ctx)
	if err != nil {
		return Claims{}, err
	}
	publicKey, ok := keys[header.KeyID]
	if !ok {
		return Claims{}, errors.New("jwt key id is not trusted")
	}

	signed := []byte(parts[0] + "." + parts[1])
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, fmt.Errorf("decode jwt signature: %w", err)
	}
	hash := sha256.Sum256(signed)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], signature); err != nil {
		return Claims{}, fmt.Errorf("verify jwt signature: %w", err)
	}

	var claims Claims
	if err := decodeSegment(parts[1], &claims); err != nil {
		return Claims{}, fmt.Errorf("decode jwt claims: %w", err)
	}
	if claims.Issuer != v.issuer || claims.Audience != v.audience {
		return Claims{}, errors.New("jwt issuer or audience is invalid")
	}
	now := v.now().Unix()
	if now < claims.NotBefore || now >= claims.ExpiresAt {
		return Claims{}, errors.New("jwt is not valid at this time")
	}
	if claims.Subject == "" || claims.SessionID == "" {
		return Claims{}, errors.New("jwt missing subject or session")
	}
	return claims, nil
}

func (v *JWTVerifier) currentKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	v.mu.RLock()
	if len(v.keys) > 0 && v.now().Before(v.expiresAt) {
		copied := cloneKeys(v.keys)
		v.mu.RUnlock()
		return copied, nil
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.keys) > 0 && v.now().Before(v.expiresAt) {
		return cloneKeys(v.keys), nil
	}
	keys, err := v.fetchKeys(ctx)
	if err != nil {
		return nil, err
	}
	v.keys = keys
	v.expiresAt = v.now().Add(v.cacheTTL)
	return cloneKeys(v.keys), nil
}

func (v *JWTVerifier) fetchKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			KeyType   string `json:"kty"`
			Use       string `json:"use"`
			KeyID     string `json:"kid"`
			Algorithm string `json:"alg"`
			Modulus   string `json:"n"`
			Exponent  string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, err
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, key := range jwks.Keys {
		if key.KeyType != "RSA" || key.Algorithm != "RS256" || key.KeyID == "" {
			continue
		}
		publicKey, err := parseRSAPublicKey(key.Modulus, key.Exponent)
		if err != nil {
			return nil, fmt.Errorf("parse jwks key %s: %w", key.KeyID, err)
		}
		keys[key.KeyID] = publicKey
	}
	if len(keys) == 0 {
		return nil, errors.New("jwks did not contain trusted RSA signing keys")
	}
	return keys, nil
}

func parseRSAPublicKey(modulus string, exponent string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(modulus)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(exponent)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	if n.Sign() <= 0 || !e.IsInt64() || e.Int64() <= 1 {
		return nil, errors.New("invalid RSA public key values")
	}
	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

func cloneKeys(keys map[string]*rsa.PublicKey) map[string]*rsa.PublicKey {
	copied := make(map[string]*rsa.PublicKey, len(keys))
	for kid, key := range keys {
		copied[kid] = key
	}
	return copied
}

func decodeSegment(segment string, out any) error {
	bytes, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, out)
}
