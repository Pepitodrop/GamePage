package gateway

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadCOBOLRuntimePolicyAndRegistry(t *testing.T) {
	directory := t.TempDir()
	policyPath := filepath.Join(directory, "policy.tsv")
	registryPath := filepath.Join(directory, "routes.tsv")
	policy := "STATUS_CACHE_TTL|7s\nMAX_GAMES|3\nMINIMUM_LAUNCHABLE_GAMES|1\nOPTIONAL_FAILURES_ALLOW_READY|true\nSLOW_GAMES_REMAIN_LAUNCHABLE|true\n"
	if err := os.WriteFile(policyPath, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	registry := "test|Test Game|/play/test|TEST_UPSTREAM|http://test:8080|/healthz|TEST_ENABLED|TEST_REQUIRED|TEST_MAX_LATENCY_MS|Y|Y|2500\n"
	if err := os.WriteFile(registryPath, []byte(registry), 0o600); err != nil {
		t.Fatal(err)
	}

	loadedPolicy, err := loadRuntimePolicy(policyPath)
	if err != nil {
		t.Fatalf("load runtime policy: %v", err)
	}
	if loadedPolicy.StatusCacheTTL != 7*time.Second || loadedPolicy.MaxGames != 3 || loadedPolicy.MinimumLaunchableGames != 1 {
		t.Fatalf("unexpected policy: %+v", loadedPolicy)
	}
	routes, err := loadRouteRegistry(registryPath, loadedPolicy.MaxGames)
	if err != nil {
		t.Fatalf("load route registry: %v", err)
	}
	if len(routes) != 1 || routes[0].key != "test" || routes[0].prefix != "/play/test" {
		t.Fatalf("unexpected routes: %+v", routes)
	}
	if routes[0].enabledEnv != "TEST_ENABLED" || routes[0].requiredEnv != "TEST_REQUIRED" || routes[0].defaultMaxLatency != 2500 {
		t.Fatalf("unexpected policy fields: %+v", routes[0])
	}
}

func TestRouteRegistryRejectsUnsafeJSONFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.tsv")
	contents := "test|Bad \"Name|/play/test|TEST_UPSTREAM|http://test:8080|/healthz|TEST_ENABLED|TEST_REQUIRED|TEST_MAX_LATENCY_MS|Y|Y|2500\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRouteRegistry(path, 3); err == nil {
		t.Fatal("expected unsafe registry field to be rejected")
	}
}

func TestRouteRegistryRejectsInvalidPolicyDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.tsv")
	contents := "test|Test Game|/play/test|TEST_UPSTREAM|http://test:8080|/healthz|TEST_ENABLED|TEST_REQUIRED|TEST_MAX_LATENCY_MS|MAYBE|Y|0\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRouteRegistry(path, 3); err == nil {
		t.Fatal("expected invalid policy defaults to be rejected")
	}
}
