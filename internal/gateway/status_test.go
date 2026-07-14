package gateway

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCOBOLDecisionEngineOwnsReadinessAndLaunchability(t *testing.T) {
	directory := t.TempDir()
	enginePath := filepath.Join(directory, "fake-cobol-core")
	script := `#!/bin/sh
set -eu
checked_at="$(head -n 1 "$1" | cut -d'|' -f2)"
cat > "$2" <<JSON
{"overall":"maintenance","ready":false,"mode":"maintenance","decisionEngine":"gnucobol","checkedAt":"${checked_at}","summary":{"total":1,"up":1,"down":0},"games":{"test":{"name":"Test Game","path":"/play/test/","status":"up","httpStatus":200,"latencyMs":4,"error":"","launchable":false,"reason":"maintenance"}}}
JSON
`
	if err := os.WriteFile(enginePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	upstream, _ := url.Parse("http://test:8080")
	routes := []RouteConfig{{Key: "test", Name: "Test Game", Prefix: "/play/test", Upstream: upstream, HealthPath: "/healthz"}}
	checker := newStatusChecker(routes, time.Second, 5*time.Second, enginePath, "true")
	checkedAt := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := checker.evaluateWithCOBOL(context.Background(), checkedAt, map[string]gameStatus{
		"test": {Name: "Test Game", Path: "/play/test/", Status: "up", HTTPStatus: 200, LatencyMS: 4},
	})
	if err != nil {
		t.Fatalf("evaluate with COBOL: %v", err)
	}
	if result.Overall != "maintenance" || result.Ready || result.Games["test"].Launchable {
		t.Fatalf("COBOL decision was not preserved: %+v", result)
	}
}

func TestMissingCOBOLDecisionEngineFailsClosed(t *testing.T) {
	checkedAt := time.Now().UTC().Format(time.RFC3339Nano)
	result := decisionEngineFailure(checkedAt, map[string]gameStatus{
		"test": {Name: "Test Game", Path: "/play/test/", Status: "up"},
	})
	if result.Ready || result.Overall != "degraded" || result.Games["test"].Launchable {
		t.Fatalf("decision-engine failure must fail closed: %+v", result)
	}
}
