package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
)

type UploadSigner interface {
	PresignPutObject(ctx context.Context, input PresignPutObjectInput) (string, error)
}

type PresignPutObjectInput struct {
	Bucket      string
	ObjectKey   string
	ContentType string
	Expires     time.Duration
	Now         time.Time
}

type S3Presigner struct {
	endpoint  string
	accessKey string
	secretKey string
	region    string
	useSSL    bool
}

type S3PresignerConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Region    string
	UseSSL    bool
}

func NewS3Presigner(config S3PresignerConfig) (*S3Presigner, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(config.Endpoint), "/")
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if strings.TrimSpace(config.AccessKey) == "" {
		return nil, fmt.Errorf("access key is required")
	}
	if strings.TrimSpace(config.SecretKey) == "" {
		return nil, fmt.Errorf("secret key is required")
	}
	region := strings.TrimSpace(config.Region)
	if region == "" {
		region = "us-east-1"
	}
	return &S3Presigner{
		endpoint:  endpoint,
		accessKey: strings.TrimSpace(config.AccessKey),
		secretKey: strings.TrimSpace(config.SecretKey),
		region:    region,
		useSSL:    config.UseSSL,
	}, nil
}

func (s *S3Presigner) PresignPutObject(_ context.Context, input PresignPutObjectInput) (string, error) {
	if strings.TrimSpace(input.Bucket) == "" || strings.TrimSpace(input.ObjectKey) == "" {
		return "", domain.ValidationError("bucket and object key are required for presigned upload.")
	}
	expires := input.Expires
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	if expires > 7*24*time.Hour {
		return "", domain.ValidationError("presigned upload expiry cannot exceed seven days.")
	}
	now := input.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	host := s.endpoint
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	objectPath := "/" + pathEscape(strings.Trim(input.Bucket, "/")) + "/" + pathEscape(strings.TrimLeft(input.ObjectKey, "/"))
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	credentialScope := dateStamp + "/" + s.region + "/s3/aws4_request"

	query := url.Values{}
	query.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	query.Set("X-Amz-Credential", s.accessKey+"/"+credentialScope)
	query.Set("X-Amz-Date", amzDate)
	query.Set("X-Amz-Expires", fmt.Sprintf("%.0f", expires.Seconds()))
	query.Set("X-Amz-SignedHeaders", "host")

	canonicalQuery := canonicalQueryString(query)
	canonicalRequest := strings.Join([]string{
		"PUT",
		objectPath,
		canonicalQuery,
		"host:" + host + "\n",
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")
	hashedCanonical := sha256Hex([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hashedCanonical,
	}, "\n")
	signingKey := signingKey(s.secretKey, dateStamp, s.region)
	signature := hmacHex(signingKey, []byte(stringToSign))
	query.Set("X-Amz-Signature", signature)

	return (&url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     objectPath,
		RawQuery: canonicalQueryString(query),
	}).String(), nil
}

func canonicalQueryString(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		vals := values[key]
		sort.Strings(vals)
		for _, value := range vals {
			parts = append(parts, queryEscape(key)+"="+queryEscape(value))
		}
	}
	return strings.Join(parts, "&")
}

func pathEscape(value string) string {
	parts := strings.Split(value, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func queryEscape(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
}

func signingKey(secret string, dateStamp string, region string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte("s3"))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func hmacHex(key []byte, data []byte) string {
	return hex.EncodeToString(hmacSHA256(key, data))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
