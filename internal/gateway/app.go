package gateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// App exposes the Go transport layer for the COBOL-owned GamePage application.
type App struct {
	handler http.Handler
}

// New constructs a production HTTP transport from validated COBOL-owned configuration.
func New(config Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if info, err := os.Stat(config.SiteDirectory); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("site directory %q is not readable", config.SiteDirectory)
	}

	checker := newStatusChecker(
		config.Routes,
		config.HealthTimeout,
		config.StatusCacheTTL,
		config.COBOLCoreExecutable,
		config.MaintenanceMode,
		config.MinimumLaunchableGames,
	)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status":          "ok",
			"transport":       "go",
			"applicationCore": "cobol",
		})
	})
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, request *http.Request) {
		status := checker.check(request.Context(), true)
		code := http.StatusOK
		if !status.Ready {
			code = http.StatusServiceUnavailable
		}
		writeStatusJSON(writer, status, code)
	})
	mux.HandleFunc("GET /api/status", func(writer http.ResponseWriter, request *http.Request) {
		writeStatusJSON(writer, checker.check(request.Context(), false), http.StatusOK)
	})

	for _, routeConfig := range config.Routes {
		route := newGameRoute(routeConfig, logger)
		prefix := routeConfig.Prefix
		mux.HandleFunc(prefix, func(writer http.ResponseWriter, request *http.Request) {
			target := prefix + "/"
			if request.URL.RawQuery != "" {
				target += "?" + request.URL.RawQuery
			}
			http.Redirect(writer, request, target, http.StatusPermanentRedirect)
		})
		mux.Handle(prefix+"/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			decision := checker.check(request.Context(), false)
			game, exists := decision.Games[routeConfig.Key]
			if !exists || !game.Launchable {
				reason := "decision-unavailable"
				if exists && game.Reason != "" {
					reason = game.Reason
				}
				writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
				writer.Header().Set("Cache-Control", "no-store")
				writer.Header().Set("Retry-After", "30")
				writer.WriteHeader(http.StatusServiceUnavailable)
				_, _ = fmt.Fprintf(writer, "%s is not launchable (%s).\n", routeConfig.Name, reason)
				return
			}
			route.serveHTTP(writer, request)
		}))
	}

	assets := http.FileServer(http.Dir(config.SiteDirectory))
	mux.Handle("GET /assets/", withStaticHeaders(assets, "public, max-age=3600"))
	mux.HandleFunc("GET /favicon.svg", exactFile(config.SiteDirectory, "favicon.svg", "image/svg+xml", "public, max-age=86400"))
	mux.HandleFunc("GET /robots.txt", exactFile(config.SiteDirectory, "robots.txt", "text/plain; charset=utf-8", "public, max-age=3600"))
	mux.HandleFunc("GET /.well-known/security.txt", exactFile(config.SiteDirectory, "security.txt", "text/plain; charset=utf-8", "public, max-age=3600"))

	// Game routes intentionally accept multiple HTTP methods. A method-agnostic
	// catch-all avoids the Go 1.22+ ServeMux pattern conflict and enforces
	// GET/HEAD only for launcher and not-found responses.
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.Header().Set("Allow", "GET, HEAD")
			http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if request.URL.Path != "/" {
			serveNotFound(config.SiteDirectory, writer)
			return
		}
		serveLauncher(config.SiteDirectory, writer, request)
	})

	handler := requestLogMiddleware(logger, securityHeaders(mux))
	return &App{handler: handler}, nil
}

// Handler returns the complete HTTP handler.
func (app *App) Handler() http.Handler {
	return app.handler
}

func serveLauncher(siteDirectory string, writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	http.ServeFile(writer, request, filepath.Join(siteDirectory, "index.html"))
}

func serveNotFound(siteDirectory string, writer http.ResponseWriter) {
	contents, err := os.ReadFile(filepath.Join(siteDirectory, "404.html"))
	if err != nil {
		http.Error(writer, "Not found", http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNotFound)
	_, _ = writer.Write(contents)
}

func exactFile(siteDirectory, name, contentType, cacheControl string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/"+name && !(name == "security.txt" && request.URL.Path == "/.well-known/security.txt") {
			serveNotFound(siteDirectory, writer)
			return
		}
		writer.Header().Set("Content-Type", contentType)
		writer.Header().Set("Cache-Control", cacheControl)
		http.ServeFile(writer, request, filepath.Join(siteDirectory, name))
	}
}

func withStaticHeaders(next http.Handler, cacheControl string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if contentType := mime.TypeByExtension(filepath.Ext(request.URL.Path)); contentType != "" {
			writer.Header().Set("Content-Type", contentType)
		}
		writer.Header().Set("Cache-Control", cacheControl)
		next.ServeHTTP(writer, request)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		writer.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		if request.TLS != nil || strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https") {
			writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(writer, request)
	})
}

func requestLogMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		next.ServeHTTP(writer, request)
		logger.Info("request", "method", request.Method, "path", request.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}

// MarshalConfigSummary returns a safe diagnostic summary without credentials.
func MarshalConfigSummary(config Config) string {
	routes := make(map[string]string, len(config.Routes))
	for _, route := range config.Routes {
		routes[route.Key] = route.Prefix
	}
	payload, _ := json.Marshal(map[string]any{
		"listen":          config.ListenAddress,
		"origin":          config.PublicOrigin,
		"routes":          routes,
		"applicationCore": "cobol",
		"registry":        config.COBOLRegistryPath,
		"policy":          config.COBOLPolicyPath,
	})
	return string(payload)
}
