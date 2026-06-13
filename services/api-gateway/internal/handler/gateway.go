package handler

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/api-gateway/internal/config"
)

type route struct {
	prefix string
	target *url.URL
	proxy  *httputil.ReverseProxy
}

type Gateway struct {
	routes []route
	logger *slog.Logger
}

func NewGateway(routes []config.Route, logger *slog.Logger) (*Gateway, error) {
	gateway := &Gateway{logger: logger}
	for _, cfgRoute := range routes {
		target := *cfgRoute.Target
		proxy := httputil.NewSingleHostReverseProxy(&target)
		originalDirector := proxy.Director
		prefix := cfgRoute.Prefix

		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.URL.Path = trimPublicAPIPrefix(req.URL.Path)
			req.Host = target.Host
			req.Header.Set("X-Forwarded-Host", req.Host)
			req.Header.Set("X-Gateway", "api-gateway")
		}

		proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
			logger.Error("proxy request failed", "path", req.URL.Path, "error", err)
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		}

		gateway.routes = append(gateway.routes, route{
			prefix: prefix,
			target: &target,
			proxy:  proxy,
		})
	}
	return gateway, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, route := range g.routes {
		if strings.HasPrefix(req.URL.Path, route.prefix) {
			g.logger.Debug("proxying request", "prefix", route.prefix, "target", route.target.String(), "path", req.URL.Path)
			route.proxy.ServeHTTP(w, req)
			return
		}
	}
	http.NotFound(w, req)
}

func trimPublicAPIPrefix(path string) string {
	trimmed := strings.TrimPrefix(path, "/api")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}
