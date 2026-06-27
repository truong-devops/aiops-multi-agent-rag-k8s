package security

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJWTVerifierVerifiesIdentityJWKSRS256Token(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	kid := "test-key"

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"use": "sig",
					"kid": kid,
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
				},
			},
		})
	}))
	defer jwksServer.Close()

	verifier, err := NewJWTVerifier(jwksServer.URL, "issuer-test", "audience-test", time.Minute, time.Second)
	if err != nil {
		t.Fatalf("NewJWTVerifier() error = %v", err)
	}
	now := time.Unix(1000, 0)
	verifier.now = func() time.Time { return now }

	token := signTestToken(t, privateKey, kid, Claims{
		Issuer:    "issuer-test",
		Audience:  "audience-test",
		Subject:   "usr_123",
		Email:     "user@example.com",
		Roles:     []string{"user"},
		SessionID: "sess_123",
		TokenID:   "jwt_123",
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
		ExpiresAt: now.Add(time.Minute).Unix(),
	})

	claims, err := verifier.VerifyAccessToken(context.Background(), token)
	if err != nil {
		t.Fatalf("VerifyAccessToken() error = %v", err)
	}
	if claims.Subject != "usr_123" || claims.Email != "user@example.com" || claims.SessionID != "sess_123" {
		t.Fatalf("claims = %#v", claims)
	}
}

func TestJWTVerifierRejectsExpiredToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	kid := "test-key"

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"kid": kid,
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
				},
			},
		})
	}))
	defer jwksServer.Close()

	verifier, err := NewJWTVerifier(jwksServer.URL, "issuer-test", "audience-test", time.Minute, time.Second)
	if err != nil {
		t.Fatalf("NewJWTVerifier() error = %v", err)
	}
	now := time.Unix(1000, 0)
	verifier.now = func() time.Time { return now }

	token := signTestToken(t, privateKey, kid, Claims{
		Issuer:    "issuer-test",
		Audience:  "audience-test",
		Subject:   "usr_123",
		SessionID: "sess_123",
		NotBefore: now.Add(-2 * time.Minute).Unix(),
		ExpiresAt: now.Add(-time.Minute).Unix(),
	})

	if _, err := verifier.VerifyAccessToken(context.Background(), token); err == nil {
		t.Fatal("VerifyAccessToken() error = nil, want expired token error")
	}
}

func signTestToken(t *testing.T, privateKey *rsa.PrivateKey, kid string, claims Claims) string {
	t.Helper()

	headerBytes, err := json.Marshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": kid,
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signed := []byte(header + "." + payload)
	hash := sha256.Sum256(signed)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(signature)
}
