package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPVideoStatusClientUpdateStatus(t *testing.T) {
	var gotToken string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("method = %s", req.Method)
		}
		if req.URL.Path != "/v1/videos/vid_123/status" {
			t.Fatalf("path = %s", req.URL.Path)
		}
		gotToken = req.Header.Get("X-Internal-Token")
		if err := json.NewDecoder(req.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewHTTPVideoStatusClient(HTTPVideoStatusClientConfig{
		BaseURL:       server.URL,
		InternalToken: "internal-secret",
	})
	if err != nil {
		t.Fatalf("NewHTTPVideoStatusClient() error = %v", err)
	}

	err = client.UpdateStatus(context.Background(), UpdateVideoStatusInput{
		VideoID:   "vid_123",
		Status:    "processing",
		Reason:    "worker_started",
		ErrorCode: "",
	})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if gotToken != "internal-secret" {
		t.Fatalf("token = %q", gotToken)
	}
	if gotBody["status"] != "processing" || gotBody["reason"] != "worker_started" {
		t.Fatalf("body = %#v", gotBody)
	}
}
