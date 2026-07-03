package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestS3ObjectStoreVerifyObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodHead {
			t.Fatalf("method = %s", req.Method)
		}
		if req.URL.Path != "/raw-videos/raw/vid_123/source.mp4" {
			t.Fatalf("path = %s", req.URL.Path)
		}
		if req.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", "2048")
		w.Header().Set("ETag", `"etag-123"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store, err := NewS3ObjectStore(S3ObjectStoreConfig{
		Endpoint:  strings.TrimPrefix(server.URL, "http://"),
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Region:    "us-east-1",
	})
	if err != nil {
		t.Fatalf("NewS3ObjectStore() error = %v", err)
	}
	metadata, err := store.VerifyObject(context.Background(), VerifyObjectInput{
		Bucket:    "raw-videos",
		ObjectKey: "raw/vid_123/source.mp4",
	})
	if err != nil {
		t.Fatalf("VerifyObject() error = %v", err)
	}
	if metadata.SizeBytes != 2048 || metadata.ContentType != "video/mp4" || metadata.ETag != "etag-123" {
		t.Fatalf("metadata = %#v", metadata)
	}
}
