package storage

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestS3PresignerPresignPutObject(t *testing.T) {
	signer, err := NewS3Presigner(S3PresignerConfig{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Region:    "us-east-1",
	})
	if err != nil {
		t.Fatalf("NewS3Presigner() error = %v", err)
	}
	rawURL, err := signer.PresignPutObject(context.Background(), PresignPutObjectInput{
		Bucket:      "raw-videos",
		ObjectKey:   "raw/vid_123/source.mp4",
		ContentType: "video/mp4",
		Expires:     15 * time.Minute,
		Now:         time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("PresignPutObject() error = %v", err)
	}
	for _, want := range []string{
		"http://localhost:9000/raw-videos/raw/vid_123/source.mp4",
		"X-Amz-Algorithm=AWS4-HMAC-SHA256",
		"X-Amz-Credential=minioadmin%2F20260701%2Fus-east-1%2Fs3%2Faws4_request",
		"X-Amz-Signature=",
	} {
		if !strings.Contains(rawURL, want) {
			t.Fatalf("url %q missing %q", rawURL, want)
		}
	}
}
