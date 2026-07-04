package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

type ObjectStore interface {
	VerifyObject(ctx context.Context, input VerifyObjectInput) (ObjectMetadata, error)
	DownloadObject(ctx context.Context, input ObjectRef) (io.ReadCloser, error)
	UploadObject(ctx context.Context, input UploadObjectInput) (ObjectMetadata, error)
}

type VerifyObjectInput struct {
	Bucket    string
	ObjectKey string
}

type ObjectRef struct {
	Bucket    string
	ObjectKey string
}

type UploadObjectInput struct {
	Bucket      string
	ObjectKey   string
	ContentType string
	SizeBytes   int64
	Body        io.Reader
}

type ObjectMetadata struct {
	SizeBytes   int64
	ContentType string
	ETag        string
}

type S3ObjectStore struct {
	endpoint   string
	accessKey  string
	secretKey  string
	region     string
	useSSL     bool
	httpClient *http.Client
}

type S3ObjectStoreConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Region    string
	UseSSL    bool
	Timeout   time.Duration
}

func NewS3ObjectStore(config S3ObjectStoreConfig) (*S3ObjectStore, error) {
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
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &S3ObjectStore{
		endpoint:   strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://"),
		accessKey:  strings.TrimSpace(config.AccessKey),
		secretKey:  strings.TrimSpace(config.SecretKey),
		region:     region,
		useSSL:     config.UseSSL,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

func (s *S3ObjectStore) VerifyObject(ctx context.Context, input VerifyObjectInput) (ObjectMetadata, error) {
	if strings.TrimSpace(input.Bucket) == "" || strings.TrimSpace(input.ObjectKey) == "" {
		return ObjectMetadata{}, domain.ValidationError("bucket and object key are required.")
	}
	req, err := s.newSignedRequest(ctx, http.MethodHead, input.Bucket, input.ObjectKey, "", nil)
	if err != nil {
		return ObjectMetadata{}, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return ObjectMetadata{}, domain.NewError(http.StatusServiceUnavailable, domain.CodeMinIOUnavailable, "Object storage is unavailable.")
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return ObjectMetadata{
			SizeBytes:   resp.ContentLength,
			ContentType: strings.TrimSpace(resp.Header.Get("Content-Type")),
			ETag:        strings.Trim(resp.Header.Get("ETag"), `"`),
		}, nil
	case http.StatusNotFound:
		return ObjectMetadata{}, domain.NotFound(domain.CodeRawObjectNotFound, "Raw video object was not found.")
	default:
		if resp.StatusCode >= 500 {
			return ObjectMetadata{}, domain.NewError(http.StatusServiceUnavailable, domain.CodeMinIOUnavailable, "Object storage metadata check failed.")
		}
		return ObjectMetadata{}, domain.NewError(http.StatusBadGateway, domain.CodeMinIOUnavailable, "Object storage metadata check failed.")
	}
}

func (s *S3ObjectStore) DownloadObject(ctx context.Context, input ObjectRef) (io.ReadCloser, error) {
	if strings.TrimSpace(input.Bucket) == "" || strings.TrimSpace(input.ObjectKey) == "" {
		return nil, domain.ValidationError("bucket and object key are required.")
	}
	req, err := s.newSignedRequest(ctx, http.MethodGet, input.Bucket, input.ObjectKey, "", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, domain.NewError(http.StatusServiceUnavailable, domain.CodeMinIOUnavailable, "Object storage is unavailable.")
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, nil
	case http.StatusNotFound:
		_ = resp.Body.Close()
		return nil, domain.NotFound(domain.CodeRawObjectNotFound, "Raw video object was not found.")
	default:
		_ = resp.Body.Close()
		if resp.StatusCode >= 500 {
			return nil, domain.NewError(http.StatusServiceUnavailable, domain.CodeMinIOUnavailable, "Object storage download failed.")
		}
		return nil, domain.NewError(http.StatusBadGateway, domain.CodeMinIOUnavailable, "Object storage download failed.")
	}
}

func (s *S3ObjectStore) UploadObject(ctx context.Context, input UploadObjectInput) (ObjectMetadata, error) {
	if strings.TrimSpace(input.Bucket) == "" || strings.TrimSpace(input.ObjectKey) == "" || input.Body == nil {
		return ObjectMetadata{}, domain.ValidationError("bucket, object key and body are required.")
	}
	req, err := s.newSignedRequest(ctx, http.MethodPut, input.Bucket, input.ObjectKey, input.ContentType, input.Body)
	if err != nil {
		return ObjectMetadata{}, err
	}
	if input.SizeBytes > 0 {
		req.ContentLength = input.SizeBytes
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return ObjectMetadata{}, domain.NewError(http.StatusServiceUnavailable, domain.CodeMinIOUnavailable, "Object storage is unavailable.")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode >= 500 {
			return ObjectMetadata{}, domain.NewError(http.StatusServiceUnavailable, domain.CodeMinIOUnavailable, "Object storage upload failed.")
		}
		return ObjectMetadata{}, domain.NewError(http.StatusBadGateway, domain.CodeMinIOUnavailable, "Object storage upload failed.")
	}
	return ObjectMetadata{
		SizeBytes:   input.SizeBytes,
		ContentType: strings.TrimSpace(input.ContentType),
		ETag:        strings.Trim(resp.Header.Get("ETag"), `"`),
	}, nil
}

type NoopObjectStore struct{}

func (NoopObjectStore) VerifyObject(context.Context, VerifyObjectInput) (ObjectMetadata, error) {
	return ObjectMetadata{}, nil
}

func (NoopObjectStore) DownloadObject(context.Context, ObjectRef) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (NoopObjectStore) UploadObject(context.Context, UploadObjectInput) (ObjectMetadata, error) {
	return ObjectMetadata{}, nil
}

func (s *S3ObjectStore) newSignedRequest(ctx context.Context, method string, bucket string, objectKey string, contentType string, body io.Reader) (*http.Request, error) {
	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	objectPath := "/" + pathEscape(strings.Trim(bucket, "/")) + "/" + pathEscape(strings.TrimLeft(objectKey, "/"))
	target := (&url.URL{Scheme: scheme, Host: s.endpoint, Path: objectPath}).String()
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	req.Host = s.endpoint
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", strings.TrimSpace(contentType))
	}
	headers := map[string]string{
		"host":                 s.endpoint,
		"x-amz-content-sha256": "UNSIGNED-PAYLOAD",
		"x-amz-date":           amzDate,
	}
	if contentType != "" {
		headers["content-type"] = strings.TrimSpace(contentType)
	}
	req.Header.Set("Authorization", s.authorizationHeader(method, objectPath, now, headers))
	return req, nil
}

func (s *S3ObjectStore) authorizationHeader(method string, objectPath string, now time.Time, headers map[string]string) string {
	amzDate := now.UTC().Format("20060102T150405Z")
	dateStamp := now.UTC().Format("20060102")
	credentialScope := dateStamp + "/" + s.region + "/s3/aws4_request"
	signedHeaders := sortedHeaderNames(headers)
	canonicalRequest := strings.Join([]string{
		method,
		objectPath,
		"",
		canonicalHeaderString(headers, signedHeaders),
		strings.Join(signedHeaders, ";"),
		"UNSIGNED-PAYLOAD",
	}, "\n")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signature := hmacHex(signingKey(s.secretKey, dateStamp, s.region), []byte(stringToSign))
	return "AWS4-HMAC-SHA256 Credential=" + s.accessKey + "/" + credentialScope +
		", SignedHeaders=" + strings.Join(signedHeaders, ";") +
		", Signature=" + signature
}

func pathEscape(value string) string {
	parts := strings.Split(value, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func sortedHeaderNames(headers map[string]string) []string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, strings.ToLower(strings.TrimSpace(name)))
	}
	sort.Strings(names)
	return names
}

func canonicalHeaderString(headers map[string]string, names []string) string {
	values := map[string]string{}
	for name, value := range headers {
		values[strings.ToLower(strings.TrimSpace(name))] = strings.Join(strings.Fields(value), " ")
	}
	var builder strings.Builder
	for _, name := range names {
		builder.WriteString(name)
		builder.WriteString(":")
		builder.WriteString(values[name])
		builder.WriteString("\n")
	}
	return builder.String()
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
