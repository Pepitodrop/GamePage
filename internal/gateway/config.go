package gateway

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains runtime settings for the COBOL-owned application and the Go transport gateway.
type Config struct {
	ListenAddress       string
	SiteDirectory       string
	PublicOrigin        string
	HealthTimeout       time.Duration
	StatusCacheTTL      time.Duration
	COBOLCoreExecutable string
	COBOLRegistryPath   string
	COBOLPolicyPath     string
	MaintenanceMode     string
	Routes              []RouteConfig
}

// RouteConfig describes one game mounted below the public launcher domain.
type RouteConfig struct {
	Key        string
	Name       string
	Prefix     string
	Upstream   *url.URL
	HealthPath string
}

type registryRoute struct {
	key, name, prefix, envName, fallback, healthPath string
}

type runtimePolicy struct {
	StatusCacheTTL time.Duration
	MaxGames       int
}

// LoadConfig loads COBOL-generated application policy and validates transport settings.
func LoadConfig() (Config, error) {
	policyPath := envOrDefault("COBOL_POLICY_PATH", "/app/site/runtime/policy.tsv")
	policy, err := loadRuntimePolicy(policyPath)
	if err != nil {
		return Config{}, fmt.Errorf("COBOL runtime policy: %w", err)
	}

	cacheTTL, err := time.ParseDuration(envOrDefault("STATUS_CACHE_TTL", policy.StatusCacheTTL.String()))
	if err != nil || cacheTTL <= 0 {
		return Config{}, fmt.Errorf("STATUS_CACHE_TTL must be a positive duration")
	}
	timeout, err := time.ParseDuration(envOrDefault("HEALTH_TIMEOUT", "3s"))
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("HEALTH_TIMEOUT must be a positive duration")
	}

	origin := strings.TrimRight(envOrDefault("PUBLIC_ORIGIN", "https://game.luisbenedikt.de"), "/")
	if _, err := parseHTTPURL(origin); err != nil {
		return Config{}, fmt.Errorf("PUBLIC_ORIGIN: %w", err)
	}

	registryPath := envOrDefault("COBOL_REGISTRY_PATH", "/app/site/runtime/routes.tsv")
	registry, err := loadRouteRegistry(registryPath, policy.MaxGames)
	if err != nil {
		return Config{}, fmt.Errorf("COBOL route registry: %w", err)
	}

	routes := make([]RouteConfig, 0, len(registry))
	for _, input := range registry {
		upstream, err := parseHTTPURL(envOrDefault(input.envName, input.fallback))
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", input.envName, err)
		}
		routes = append(routes, RouteConfig{
			Key:        input.key,
			Name:       input.name,
			Prefix:     input.prefix,
			Upstream:   upstream,
			HealthPath: input.healthPath,
		})
	}

	return Config{
		ListenAddress:       envOrDefault("LISTEN_ADDRESS", ":8080"),
		SiteDirectory:       envOrDefault("SITE_DIRECTORY", "/app/site"),
		PublicOrigin:        origin,
		HealthTimeout:       timeout,
		StatusCacheTTL:      cacheTTL,
		COBOLCoreExecutable: envOrDefault("COBOL_CORE_EXECUTABLE", "/app/gamepage-core"),
		COBOLRegistryPath:   registryPath,
		COBOLPolicyPath:     policyPath,
		MaintenanceMode:     envOrDefault("MAINTENANCE_MODE", "false"),
		Routes:              routes,
	}, nil
}

func loadRuntimePolicy(path string) (runtimePolicy, error) {
	file, err := os.Open(path)
	if err != nil {
		return runtimePolicy{}, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			return runtimePolicy{}, fmt.Errorf("invalid policy record %q", line)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			return runtimePolicy{}, fmt.Errorf("empty policy key or value")
		}
		if _, exists := values[key]; exists {
			return runtimePolicy{}, fmt.Errorf("duplicate policy key %q", key)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return runtimePolicy{}, err
	}

	cacheTTL, err := time.ParseDuration(values["STATUS_CACHE_TTL"])
	if err != nil || cacheTTL <= 0 {
		return runtimePolicy{}, fmt.Errorf("STATUS_CACHE_TTL is missing or invalid")
	}
	maxGames, err := strconv.Atoi(values["MAX_GAMES"])
	if err != nil || maxGames < 1 || maxGames > 10 {
		return runtimePolicy{}, fmt.Errorf("MAX_GAMES must be between 1 and 10")
	}
	return runtimePolicy{StatusCacheTTL: cacheTTL, MaxGames: maxGames}, nil
}

func loadRouteRegistry(path string, maxGames int) ([]registryRoute, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var routes []registryRoute
	seenKeys := map[string]struct{}{}
	seenPrefixes := map[string]struct{}{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 6 {
			return nil, fmt.Errorf("invalid route record %q", line)
		}
		for index := range parts {
			parts[index] = strings.TrimSpace(parts[index])
			if !isSafeCOBOLField(parts[index]) {
				return nil, fmt.Errorf("route field contains a forbidden JSON or record character")
			}
		}
		route := registryRoute{parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]}
		if !isIdentifier(route.key) {
			return nil, fmt.Errorf("invalid route key %q", route.key)
		}
		if route.name == "" || !strings.HasPrefix(route.prefix, "/play/") || strings.HasSuffix(route.prefix, "/") {
			return nil, fmt.Errorf("invalid name or prefix for route %q", route.key)
		}
		if !isEnvironmentName(route.envName) || !strings.HasPrefix(route.healthPath, "/") {
			return nil, fmt.Errorf("invalid environment name or health path for route %q", route.key)
		}
		if _, err := parseHTTPURL(route.fallback); err != nil {
			return nil, fmt.Errorf("route %q fallback: %w", route.key, err)
		}
		if _, exists := seenKeys[route.key]; exists {
			return nil, fmt.Errorf("duplicate route key %q", route.key)
		}
		if _, exists := seenPrefixes[route.prefix]; exists {
			return nil, fmt.Errorf("duplicate route prefix %q", route.prefix)
		}
		seenKeys[route.key] = struct{}{}
		seenPrefixes[route.prefix] = struct{}{}
		routes = append(routes, route)
		if len(routes) > maxGames {
			return nil, fmt.Errorf("registry exceeds COBOL MAX_GAMES policy")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("registry contains no games")
	}
	return routes, nil
}

func isSafeCOBOLField(value string) bool {
	return !strings.ContainsAny(value, "|\"\\\r\n")
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

func isEnvironmentName(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}

func parseHTTPURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("URL scheme must be http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("URL host is required")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("userinfo, query strings, and fragments are not allowed")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed, nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
