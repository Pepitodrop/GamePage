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
	if err := os.WriteFile(policyPath, []byte("STATUS_CACHE_TTL|7s\nMAX_GAMES|3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registryPath, []byte("test|Test Game|/play/test|TEST_UPSTREAM|http://test:8080|/healthz\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	policy, err := loadRuntimePolicy(policyPath)
	if err != nil {
		t.Fatalf("load runtime policy: %v", err)
	}
	if policy.StatusCacheTTL != 7*time.Second || policy.MaxGames != 3 {
		t.Fatalf("unexpected policy: %+v", policy)
	}
	routes, err := loadRouteRegistry(registryPath, policy.MaxGames)
	if err != nil {
		t.Fatalf("load route registry: %v", err)
	}
	if len(routes) != 1 || routes[0].key != "test" || routes[0].prefix != "/play/test" {
		t.Fatalf("unexpected routes: %+v", routes)
	}
}

func TestRouteRegistryRejectsUnsafeJSONFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.tsv")
	contents := "test|Bad \"Name|/play/test|TEST_UPSTREAM|http://test:8080|/healthz\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRouteRegistry(path, 3); err == nil {
		t.Fatal("expected unsafe registry field to be rejected")
	}
}
