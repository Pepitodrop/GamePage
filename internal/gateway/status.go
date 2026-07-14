package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type statusChecker struct {
	client  *http.Client
	routes  []RouteConfig
	mu      sync.Mutex
	cached  statusResponse
	expires time.Time
}

type statusResponse struct {
	Overall   string                `json:"overall"`
	CheckedAt time.Time             `json:"checkedAt"`
	Games     map[string]gameStatus `json:"games"`
}

type gameStatus struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Status     string `json:"status"`
	HTTPStatus int    `json:"httpStatus,omitempty"`
	LatencyMS  int64  `json:"latencyMs"`
	Error      string `json:"error,omitempty"`
}

func newStatusChecker(routes []RouteConfig, timeout time.Duration) *statusChecker {
	return &statusChecker{
		client: &http.Client{Timeout: timeout},
		routes: routes,
	}
}

func (checker *statusChecker) check(ctx context.Context, force bool) statusResponse {
	checker.mu.Lock()
	defer checker.mu.Unlock()

	if !force && time.Now().Before(checker.expires) && checker.cached.CheckedAt.Unix() != 0 {
		return checker.cached
	}

	result := statusResponse{
		Overall:   "ok",
		CheckedAt: time.Now().UTC(),
		Games:     make(map[string]gameStatus, len(checker.routes)),
	}

	var waitGroup sync.WaitGroup
	var resultMu sync.Mutex
	for _, route := range checker.routes {
		route := route
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			status := checker.checkRoute(ctx, route)
			resultMu.Lock()
			result.Games[route.Key] = status
			if status.Status != "up" {
				result.Overall = "degraded"
			}
			resultMu.Unlock()
		}()
	}
	waitGroup.Wait()

	checker.cached = result
	checker.expires = time.Now().Add(5 * time.Second)
	return result
}

func (checker *statusChecker) checkRoute(ctx context.Context, route RouteConfig) gameStatus {
	status := gameStatus{Name: route.Name, Path: route.Prefix + "/", Status: "down"}
	healthURL := *route.Upstream
	healthURL.Path = joinURLPath(route.Upstream.Path, route.HealthPath)
	healthURL.RawQuery = ""

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL.String(), nil)
	if err != nil {
		status.Error = "invalid health request"
		return status
	}
	request.Header.Set("User-Agent", "GamePage-Health/1.0")
	started := time.Now()
	response, err := checker.client.Do(request)
	status.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		status.Error = "unreachable"
		return status
	}
	defer response.Body.Close()
	status.HTTPStatus = response.StatusCode
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		status.Status = "up"
		return status
	}
	status.Error = http.StatusText(response.StatusCode)
	return status
}

func joinURLPath(basePath, suffix string) string {
	base, _ := url.JoinPath(basePath+"/", suffix)
	if base == "" {
		return "/"
	}
	return base
}

func writeStatusJSON(writer http.ResponseWriter, status statusResponse, responseCode int) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(responseCode)
	_ = json.NewEncoder(writer).Encode(status)
}
