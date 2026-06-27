package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type ReadinessCheck func(context.Context) error

func NewReadinessHandler(checks ...ReadinessCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()

		var combined error
		for _, check := range checks {
			if check == nil {
				continue
			}
			if err := check(ctx); err != nil {
				combined = errors.Join(combined, err)
			}
		}
		if combined != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "not_ready",
				"error":  combined.Error(),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}
}

func (g *Gateway) Ready(context.Context) error {
	if g == nil || len(g.routes) == 0 {
		return errors.New("gateway has no configured routes")
	}
	return nil
}
