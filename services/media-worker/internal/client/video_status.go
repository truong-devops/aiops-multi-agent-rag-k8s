package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type VideoStatusClient interface {
	UpdateStatus(ctx context.Context, input UpdateVideoStatusInput) error
}

type UpdateVideoStatusInput struct {
	VideoID            string
	Status             string
	Reason             string
	ErrorCode          string
	ProcessedObjectKey string
	ThumbnailObjectKey string
	DurationMs         int64
	Width              int
	Height             int
	SizeBytes          int64
	RequestID          string
	CorrelationID      string
}

type HTTPVideoStatusClient struct {
	baseURL       string
	internalToken string
	httpClient    *http.Client
}

type HTTPVideoStatusClientConfig struct {
	BaseURL       string
	InternalToken string
	Timeout       time.Duration
}

func NewHTTPVideoStatusClient(config HTTPVideoStatusClientConfig) (*HTTPVideoStatusClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("video service base url is required")
	}
	if strings.TrimSpace(config.InternalToken) == "" {
		return nil, fmt.Errorf("internal api token is required")
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPVideoStatusClient{
		baseURL:       baseURL,
		internalToken: strings.TrimSpace(config.InternalToken),
		httpClient:    &http.Client{Timeout: timeout},
	}, nil
}

func (c *HTTPVideoStatusClient) UpdateStatus(ctx context.Context, input UpdateVideoStatusInput) error {
	if strings.TrimSpace(input.VideoID) == "" {
		return fmt.Errorf("video_id is required")
	}
	body := map[string]any{
		"status":     strings.TrimSpace(input.Status),
		"reason":     strings.TrimSpace(input.Reason),
		"error_code": strings.TrimSpace(input.ErrorCode),
	}
	if strings.TrimSpace(input.ProcessedObjectKey) != "" {
		body["processed_object_key"] = strings.TrimSpace(input.ProcessedObjectKey)
	}
	if strings.TrimSpace(input.ThumbnailObjectKey) != "" {
		body["thumbnail_object_key"] = strings.TrimSpace(input.ThumbnailObjectKey)
	}
	if input.DurationMs > 0 {
		body["duration_ms"] = input.DurationMs
	}
	if input.Width > 0 {
		body["width"] = input.Width
	}
	if input.Height > 0 {
		body["height"] = input.Height
	}
	if input.SizeBytes > 0 {
		body["size_bytes"] = input.SizeBytes
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+"/v1/videos/"+input.VideoID+"/status", bytes.NewReader(rawBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.internalToken)
	if strings.TrimSpace(input.RequestID) != "" {
		req.Header.Set("X-Request-ID", input.RequestID)
	}
	if strings.TrimSpace(input.CorrelationID) != "" {
		req.Header.Set("X-Correlation-ID", input.CorrelationID)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call video-service status api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("video-service status api returned %d", resp.StatusCode)
	}
	return nil
}
