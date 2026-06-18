package security

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/domain"
)

type AccessTokenClaims struct {
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

type SignInput struct {
	UserID    string
	Email     string
	Roles     []string
	SessionID string
	TTL       time.Duration
}

type JWTManager struct {
	issuer   string
	audience string
	kid      string
	private  *rsa.PrivateKey
}

func NewJWTManager(issuer string, audience string, keyPEM string) (*JWTManager, error) {
	var privateKey *rsa.PrivateKey
	var err error
	if strings.TrimSpace(keyPEM) == "" {
		privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	} else {
		privateKey, err = parseRSAPrivateKey(keyPEM)
	}
	if err != nil {
		return nil, err
	}

	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	kidHash := sha256.Sum256(publicDER)
	return &JWTManager{
		issuer:   issuer,
		audience: audience,
		kid:      base64.RawURLEncoding.EncodeToString(kidHash[:8]),
		private:  privateKey,
	}, nil
}

func (m *JWTManager) SignAccessToken(input SignInput, now time.Time) (string, AccessTokenClaims, error) {
	claims := AccessTokenClaims{
		Issuer:    m.issuer,
		Audience:  m.audience,
		Subject:   input.UserID,
		Email:     input.Email,
		Roles:     append([]string(nil), input.Roles...),
		SessionID: input.SessionID,
		TokenID:   domain.NewID("jwt"),
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
		ExpiresAt: now.Add(input.TTL).Unix(),
	}

	token, err := m.sign(claims)
	if err != nil {
		return "", AccessTokenClaims{}, err
	}
	return token, claims, nil
}

func (m *JWTManager) VerifyAccessToken(token string, now time.Time) (AccessTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return AccessTokenClaims{}, errors.New("jwt must have three parts")
	}

	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return AccessTokenClaims{}, fmt.Errorf("decode jwt header: %w", err)
	}
	if header.Algorithm != "RS256" || header.KeyID != m.kid {
		return AccessTokenClaims{}, errors.New("jwt header is not trusted")
	}

	signed := []byte(parts[0] + "." + parts[1])
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return AccessTokenClaims{}, fmt.Errorf("decode jwt signature: %w", err)
	}
	hash := sha256.Sum256(signed)
	if err := rsa.VerifyPKCS1v15(&m.private.PublicKey, crypto.SHA256, hash[:], signature); err != nil {
		return AccessTokenClaims{}, fmt.Errorf("verify jwt signature: %w", err)
	}

	var claims AccessTokenClaims
	if err := decodeSegment(parts[1], &claims); err != nil {
		return AccessTokenClaims{}, fmt.Errorf("decode jwt claims: %w", err)
	}
	if claims.Issuer != m.issuer || claims.Audience != m.audience {
		return AccessTokenClaims{}, errors.New("jwt issuer or audience is invalid")
	}
	if now.Unix() < claims.NotBefore || now.Unix() >= claims.ExpiresAt {
		return AccessTokenClaims{}, errors.New("jwt is not valid at this time")
	}
	if claims.Subject == "" || claims.SessionID == "" {
		return AccessTokenClaims{}, errors.New("jwt missing subject or session")
	}
	return claims, nil
}

func (m *JWTManager) JWKS() map[string]any {
	publicKey := &m.private.PublicKey
	return map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": m.kid,
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
			},
		},
	}
}

func (m *JWTManager) sign(claims AccessTokenClaims) (string, error) {
	headerBytes, err := json.Marshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": m.kid,
	})
	if err != nil {
		return "", err
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signed := []byte(header + "." + payload)
	hash := sha256.Sum256(signed)
	signature, err := rsa.SignPKCS1v15(rand.Reader, m.private, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func parseRSAPrivateKey(keyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, errors.New("SIGNING_KEY_PEM must contain a PEM block")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse RSA private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("SIGNING_KEY_PEM must be an RSA private key")
	}
	return key, nil
}

func decodeSegment(segment string, out any) error {
	bytes, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, out)
}
