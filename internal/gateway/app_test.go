package gateway

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewRegistersLauncherAndMethodAgnosticGameRoutes(t *testing.T) {
	t.Parallel()

	siteDirectory := t.TempDir()
	writeTestSiteFile(t, siteDirectory, "index.html", "<!doctype html><title>Launcher</title>")
	writeTestSiteFile(t, siteDirectory, "404.html", "<!doctype html><title>Not found</title>")

	var upstreamMethod string
	var upstreamPath string
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamMethod = request.Method
		upstreamPath = request.URL.Path
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("proxied"))
	}))
	defer upstreamServer.Close()

	upstreamURL, err := url.Parse(upstreamServer.URL)
	if err != nil {
		t.Fatalf("parse upstream URL: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := New(Config{
		SiteDirectory: siteDirectory,
		PublicOrigin:  "https://example.test",
		HealthTimeout: time.Second,
		Routes: []RouteConfig{
			{
				Key:        "test",
				Name:       "Test Game",
				Prefix:     "/play/test",
				Upstream:   upstreamURL,
				HealthPath: "/healthz",
			},
		},
	}, logger)
	if err != nil {
		t.Fatalf("construct app: %v", err)
	}

	rootResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(rootResponse, httptest.NewRequest(http.MethodGet, "https://example.test/", nil))
	if rootResponse.Code != http.StatusOK {
		t.Fatalf("GET / returned %d, want %d", rootResponse.Code, http.StatusOK)
	}
	if !strings.Contains(rootResponse.Body.String(), "Launcher") {
		t.Fatalf("GET / did not return the launcher: %q", rootResponse.Body.String())
	}

	proxyResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(proxyResponse, httptest.NewRequest(http.MethodPost, "https://example.test/play/test/action", strings.NewReader("payload")))
	if proxyResponse.Code != http.StatusOK {
		t.Fatalf("POST game route returned %d, want %d", proxyResponse.Code, http.StatusOK)
	}
	if upstreamMethod != http.MethodPost {
		t.Fatalf("upstream method = %q, want %q", upstreamMethod, http.MethodPost)
	}
	if upstreamPath != "/action" {
		t.Fatalf("upstream path = %q, want %q", upstreamPath, "/action")
	}

	unknownResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(unknownResponse, httptest.NewRequest(http.MethodPost, "https://example.test/unknown", nil))
	if unknownResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST unknown route returned %d, want %d", unknownResponse.Code, http.StatusMethodNotAllowed)
	}
	if allow := unknownResponse.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Fatalf("Allow header = %q, want %q", allow, "GET, HEAD")
	}
}

func writeTestSiteFile(t *testing.T, directory, name, contents string) {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
