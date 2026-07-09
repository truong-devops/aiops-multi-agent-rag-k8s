package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/api-gateway/internal/config"
)

type route struct {
	name            string
	prefix          string
	target          *url.URL
	nestedResources map[string]struct{}
	proxy           *httputil.ReverseProxy
}

type Gateway struct {
	routes []route
	logger *slog.Logger
}

func NewGateway(routes []config.Route, upstreamTimeout time.Duration, logger *slog.Logger) (*Gateway, error) {
	if logger == nil {
		logger = slog.Default()
	}
	gateway := &Gateway{logger: logger}
	for _, cfgRoute := range routes {
		if cfgRoute.Prefix == "" || !strings.HasPrefix(cfgRoute.Prefix, "/api/") {
			return nil, fmt.Errorf("route %q must start with /api/", cfgRoute.Prefix)
		}
		if cfgRoute.Target == nil || cfgRoute.Target.Scheme == "" || cfgRoute.Target.Host == "" {
			return nil, fmt.Errorf("route %q target must include scheme and host", cfgRoute.Prefix)
		}

		target := *cfgRoute.Target
		prefix := cfgRoute.Prefix
		name := cfgRoute.Name
		if name == "" {
			name = target.Host
		}
		nestedResources := map[string]struct{}{}
		for _, item := range cfgRoute.NestedResources {
			item = strings.Trim(strings.ToLower(item), "/ ")
			if item != "" {
				nestedResources[item] = struct{}{}
			}
		}

		proxy := httputil.NewSingleHostReverseProxy(&target)
		originalDirector := proxy.Director

		proxy.Director = func(req *http.Request) {
			originalHost := req.Host
			originalScheme := forwardedProto(req)
			clientIP := remoteIP(req)
			originalDirector(req)
			req.URL.Path = trimPublicAPIPrefix(req.URL.Path)
			req.URL.RawPath = ""
			req.Host = target.Host
			req.Header.Del("X-Forwarded-For")
			req.Header.Set("X-Forwarded-Host", originalHost)
			req.Header.Set("X-Forwarded-Proto", originalScheme)
			if clientIP != "" {
				req.Header.Set("X-Real-IP", clientIP)
			}
			req.Header.Set("X-Gateway", "api-gateway")
		}

		proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
			logger.Error("proxy request failed", "upstream_service", name, "path", req.URL.Path, "error", err)
			WriteError(w, req, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "Upstream service is unavailable.")
		}

		if upstreamTimeout > 0 {
			proxy.Transport = &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				ResponseHeaderTimeout: upstreamTimeout,
			}
		}

		gateway.routes = append(gateway.routes, route{
			name:            name,
			prefix:          prefix,
			target:          &target,
			nestedResources: nestedResources,
			proxy:           proxy,
		})
	}
	sort.SliceStable(gateway.routes, func(i, j int) bool {
		return len(gateway.routes[i].prefix) > len(gateway.routes[j].prefix)
	})
	return gateway, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, route := range g.routes {
		if matchRoute(req.URL.Path, route.prefix, route.nestedResources) {
			g.logger.Debug(
				"proxying request",
				"upstream_service", route.name,
				"prefix", route.prefix,
				"target", route.target.String(),
				"path", req.URL.Path,
			)
			route.proxy.ServeHTTP(w, req)
			return
		}
	}
	WriteError(w, req, http.StatusNotFound, "ROUTE_NOT_FOUND", "No API route matched the request path.")
}

func matchRoute(path string, prefix string, nestedResources map[string]struct{}) bool {
	if len(nestedResources) > 0 {
		return matchNestedResource(path, prefix, nestedResources)
	}
	if strings.HasSuffix(prefix, "/") {
		return strings.HasPrefix(path, prefix)
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func matchNestedResource(path string, prefix string, nestedResources map[string]struct{}) bool {
	if !strings.HasSuffix(prefix, "/") || !strings.HasPrefix(path, prefix) {
		return false
	}
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	_, ok := nestedResources[strings.ToLower(parts[1])]
	return ok
}

func trimPublicAPIPrefix(path string) string {
	trimmed := strings.TrimPrefix(path, "/api")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

func forwardedProto(req *http.Request) string {
	if req.TLS != nil {
		return "https"
	}
	return "http"
}

func remoteIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		return host
	}
	return req.RemoteAddr
}

func WriteError(w http.ResponseWriter, req *http.Request, status int, code string, message string) {
	requestID := req.Header.Get("X-Request-ID")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
			Details: map[string]string{},
		},
		RequestID: requestID,
	})
}

type errorResponse struct {
	Error     errorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}
