package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type statusChecker struct {
	client          *http.Client
	routes          []RouteConfig
	cacheTTL        time.Duration
	coreExecutable  string
	maintenanceMode string
	mu              sync.Mutex
	cached          statusResponse
	expires         time.Time
}

type statusResponse struct {
	Overall        string                `json:"overall"`
	Ready          bool                  `json:"ready"`
	Mode           string                `json:"mode"`
	DecisionEngine string                `json:"decisionEngine"`
	CheckedAt      string                `json:"checkedAt"`
	Summary        statusSummary         `json:"summary"`
	Games          map[string]gameStatus `json:"games"`
}

type statusSummary struct {
	Total int `json:"total"`
	Up    int `json:"up"`
	Down  int `json:"down"`
}

type gameStatus struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Status     string `json:"status"`
	HTTPStatus int    `json:"httpStatus"`
	LatencyMS  int64  `json:"latencyMs"`
	Error      string `json:"error,omitempty"`
	Launchable bool   `json:"launchable"`
	Reason     string `json:"reason"`
}

func newStatusChecker(routes []RouteConfig, timeout, cacheTTL time.Duration, coreExecutable, maintenanceMode string) *statusChecker {
	return &statusChecker{
		client:          &http.Client{Timeout: timeout},
		routes:          routes,
		cacheTTL:        cacheTTL,
		coreExecutable:  coreExecutable,
		maintenanceMode: maintenanceMode,
	}
}

func (checker *statusChecker) check(ctx context.Context, force bool) statusResponse {
	checker.mu.Lock()
	defer checker.mu.Unlock()

	if !force && time.Now().Before(checker.expires) && checker.cached.CheckedAt != "" {
		return checker.cached
	}

	checkedAt := time.Now().UTC().Format(time.RFC3339Nano)
	probes := make(map[string]gameStatus, len(checker.routes))
	var waitGroup sync.WaitGroup
	var resultMu sync.Mutex
	for _, route := range checker.routes {
		route := route
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			probe := checker.checkRoute(ctx, route)
			resultMu.Lock()
			probes[route.Key] = probe
			resultMu.Unlock()
		}()
	}
	waitGroup.Wait()

	result, err := checker.evaluateWithCOBOL(ctx, checkedAt, probes)
	if err != nil {
		result = decisionEngineFailure(checkedAt, probes)
	}
	checker.cached = result
	checker.expires = time.Now().Add(checker.cacheTTL)
	return result
}

func (checker *statusChecker) checkRoute(ctx context.Context, route RouteConfig) gameStatus {
	status := gameStatus{Name: route.Name, Path: route.Prefix + "/", Status: "down", Error: "unreachable"}
	healthURL := *route.Upstream
	healthURL.Path = joinURLPath(route.Upstream.Path, route.HealthPath)
	healthURL.RawQuery = ""

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL.String(), nil)
	if err != nil {
		status.Error = "invalid-request"
		return status
	}
	request.Header.Set("User-Agent", "GamePage-Health/2.0")
	started := time.Now()
	response, err := checker.client.Do(request)
	status.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		return status
	}
	defer response.Body.Close()
	status.HTTPStatus = response.StatusCode
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		status.Status = "up"
		status.Error = ""
		return status
	}
	status.Error = "http-error"
	return status
}

func (checker *statusChecker) evaluateWithCOBOL(ctx context.Context, checkedAt string, probes map[string]gameStatus) (statusResponse, error) {
	if strings.TrimSpace(checker.coreExecutable) == "" {
		return statusResponse{}, fmt.Errorf("COBOL decision engine is not configured")
	}
	workDirectory, err := os.MkdirTemp("", "gamepage-cobol-")
	if err != nil {
		return statusResponse{}, fmt.Errorf("create COBOL work directory: %w", err)
	}
	defer os.RemoveAll(workDirectory)

	inputPath := filepath.Join(workDirectory, "status.in")
	outputPath := filepath.Join(workDirectory, "status.json")
	var input bytes.Buffer
	maintenance := safeEngineField(checker.maintenanceMode)
	fmt.Fprintf(&input, "META|%s|%s\n", checkedAt, maintenance)
	for _, route := range checker.routes {
		probe, exists := probes[route.Key]
		if !exists {
			return statusResponse{}, fmt.Errorf("missing health probe for %s", route.Key)
		}
		fields := []string{route.Key, probe.Name, probe.Path, probe.Status, fmt.Sprint(probe.HTTPStatus), fmt.Sprint(probe.LatencyMS), probe.Error}
		for index := range fields {
			fields[index] = safeEngineField(fields[index])
		}
		fmt.Fprintf(&input, "GAME|%s\n", strings.Join(fields, "|"))
	}
	if err := os.WriteFile(inputPath, input.Bytes(), 0o600); err != nil {
		return statusResponse{}, fmt.Errorf("write COBOL input: %w", err)
	}

	command := exec.CommandContext(ctx, checker.coreExecutable, inputPath, outputPath)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return statusResponse{}, fmt.Errorf("COBOL decision engine failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		return statusResponse{}, fmt.Errorf("read COBOL output: %w", err)
	}
	var result statusResponse
	if err := json.Unmarshal(output, &result); err != nil {
		return statusResponse{}, fmt.Errorf("decode COBOL output: %w", err)
	}
	if err := validateCOBOLResult(result, checkedAt, checker.routes); err != nil {
		return statusResponse{}, err
	}
	return result, nil
}

func validateCOBOLResult(result statusResponse, checkedAt string, routes []RouteConfig) error {
	if result.DecisionEngine != "gnucobol" || result.CheckedAt != checkedAt {
		return fmt.Errorf("COBOL result identity mismatch")
	}
	if result.Overall != "ok" && result.Overall != "degraded" && result.Overall != "maintenance" && result.Overall != "configuration-error" {
		return fmt.Errorf("COBOL returned invalid overall state %q", result.Overall)
	}
	if len(result.Games) != len(routes) || result.Summary.Total != len(routes) {
		return fmt.Errorf("COBOL returned incomplete game data")
	}
	for _, route := range routes {
		status, exists := result.Games[route.Key]
		if !exists || status.Name != route.Name || status.Path != route.Prefix+"/" {
			return fmt.Errorf("COBOL returned invalid route data for %q", route.Key)
		}
	}
	return nil
}

func safeEngineField(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !isSafeCOBOLField(value) {
		return "INVALID"
	}
	return value
}

func decisionEngineFailure(checkedAt string, probes map[string]gameStatus) statusResponse {
	games := make(map[string]gameStatus, len(probes))
	up := 0
	for key, probe := range probes {
		if probe.Status == "up" {
			up++
		}
		probe.Launchable = false
		probe.Reason = "decision-engine-unavailable"
		games[key] = probe
	}
	return statusResponse{
		Overall:        "degraded",
		Ready:          false,
		Mode:           "fail-safe",
		DecisionEngine: "unavailable",
		CheckedAt:      checkedAt,
		Summary:        statusSummary{Total: len(games), Up: up, Down: len(games) - up},
		Games:          games,
	}
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
