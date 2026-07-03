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
	VideoID       string
	Status        string
	Reason        string
	ErrorCode     string
	RequestID     string
	CorrelationID string
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
	body := map[string]string{
		"status":     strings.TrimSpace(input.Status),
		"reason":     strings.TrimSpace(input.Reason),
		"error_code": strings.TrimSpace(input.ErrorCode),
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
