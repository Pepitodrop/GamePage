package gateway

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

type gameRoute struct {
	config RouteConfig
	proxy  *httputil.ReverseProxy
}

func newGameRoute(config RouteConfig, logger *slog.Logger) *gameRoute {
	proxy := httputil.NewSingleHostReverseProxy(config.Upstream)
	baseDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		publicHost := request.Host
		forwardedProto := request.Header.Get("X-Forwarded-Proto")
		if forwardedProto == "" {
			if request.TLS != nil {
				forwardedProto = "https"
			} else {
				forwardedProto = "http"
			}
		}

		baseDirector(request)
		request.Header.Del("Accept-Encoding")
		request.Header.Set("X-Forwarded-Host", publicHost)
		request.Header.Set("X-Forwarded-Proto", forwardedProto)
		request.Header.Set("X-Forwarded-Prefix", config.Prefix)
		request.Header.Set("X-GamePage-Gateway", "1")
	}
	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		DisableCompression:    true,
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		return rewriteResponse(response, config.Prefix, config.Upstream)
	}
	proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
		logger.Error("upstream request failed", "game", config.Key, "path", request.URL.Path, "error", err)
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprintf(writer, "<!doctype html><html lang=\"en\"><meta charset=\"utf-8\"><title>Game unavailable</title><body><h1>%s is temporarily unavailable</h1><p>Return to the <a href=\"/\">game launcher</a> and try again.</p></body></html>", config.Name)
	}

	return &gameRoute{config: config, proxy: proxy}
}

func (route *gameRoute) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, route.config.Prefix)
	if path == "" {
		path = "/"
	}

	clone := request.Clone(request.Context())
	clonedURL := *request.URL
	clonedURL.Path = path
	clonedURL.RawPath = ""
	clone.URL = &clonedURL
	route.proxy.ServeHTTP(writer, clone)
}
