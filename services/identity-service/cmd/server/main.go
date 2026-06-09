package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

const serviceName = "identity-service"

func main() {
	port := getenv("PORT", "8080")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", textHandler("ok\n"))
	mux.HandleFunc("/readyz", textHandler("ready\n"))
	mux.HandleFunc("/metrics", textHandler("# metrics placeholder\n"))

	slog.Info("starting service", "service", serviceName, "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("service stopped", "service", serviceName, "error", err)
		os.Exit(1)
	}
}

func textHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprint(w, body)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
