package storage

import (
	"context"
	"io"
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

func TestS3ObjectStoreDownloadObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("method = %s", req.Method)
		}
		if req.URL.Path != "/raw-videos/raw/vid_123/source.mp4" {
			t.Fatalf("path = %s", req.URL.Path)
		}
		if req.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		_, _ = w.Write([]byte("raw-video"))
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
	body, err := store.DownloadObject(context.Background(), ObjectRef{
		Bucket:    "raw-videos",
		ObjectKey: "raw/vid_123/source.mp4",
	})
	if err != nil {
		t.Fatalf("DownloadObject() error = %v", err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(raw) != "raw-video" {
		t.Fatalf("body = %q", string(raw))
	}
}

func TestS3ObjectStoreUploadObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			t.Fatalf("method = %s", req.Method)
		}
		if req.URL.Path != "/processed-videos/processed/vid_123/source.mp4" {
			t.Fatalf("path = %s", req.URL.Path)
		}
		if req.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		if req.Header.Get("Content-Type") != "video/mp4" {
			t.Fatalf("content-type = %q", req.Header.Get("Content-Type"))
		}
		if req.ContentLength != int64(len("processed-video")) {
			t.Fatalf("content-length = %d", req.ContentLength)
		}
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if string(raw) != "processed-video" {
			t.Fatalf("body = %q", string(raw))
		}
		w.Header().Set("ETag", `"etag-upload"`)
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
	metadata, err := store.UploadObject(context.Background(), UploadObjectInput{
		Bucket:      "processed-videos",
		ObjectKey:   "processed/vid_123/source.mp4",
		ContentType: "video/mp4",
		SizeBytes:   int64(len("processed-video")),
		Body:        strings.NewReader("processed-video"),
	})
	if err != nil {
		t.Fatalf("UploadObject() error = %v", err)
	}
	if metadata.SizeBytes != int64(len("processed-video")) || metadata.ContentType != "video/mp4" || metadata.ETag != "etag-upload" {
		t.Fatalf("metadata = %#v", metadata)
	}
}
