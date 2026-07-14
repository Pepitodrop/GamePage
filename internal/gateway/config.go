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
	ListenAddress          string
	SiteDirectory          string
	PublicOrigin           string
	HealthTimeout          time.Duration
	StatusCacheTTL         time.Duration
	COBOLCoreExecutable    string
	COBOLRegistryPath      string
	COBOLPolicyPath        string
	MaintenanceMode        string
	MinimumLaunchableGames string
	Routes                 []RouteConfig
}

// RouteConfig describes one game mounted below the public launcher domain.
type RouteConfig struct {
	Key                string
	Name               string
	Prefix             string
	Upstream           *url.URL
	HealthPath         string
	EnabledOverride    string
	RequiredOverride   string
	MaxLatencyOverride string
}

type registryRoute struct {
	key, name, prefix, upstreamEnv, fallback, healthPath string
	enabledEnv, requiredEnv, latencyEnv                  string
	defaultEnabled, defaultRequired                     string
	defaultMaxLatency                                   int
}

type runtimePolicy struct {
	StatusCacheTTL        time.Duration
	MaxGames              int
	MinimumLaunchableGames int
}

// LoadConfig loads COBOL-generated application policy and validates transport settings.
// Raw policy overrides are deliberately not interpreted here; COBOL resolves them.
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
		upstream, err := parseHTTPURL(envOrDefault(input.upstreamEnv, input.fallback))
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", input.upstreamEnv, err)
		}
		routes = append(routes, RouteConfig{
			Key:                input.key,
			Name:               input.name,
			Prefix:             input.prefix,
			Upstream:           upstream,
			HealthPath:         input.healthPath,
			EnabledOverride:    envRaw(input.enabledEnv),
			RequiredOverride:   envRaw(input.requiredEnv),
			MaxLatencyOverride: envRaw(input.latencyEnv),
		})
	}

	return Config{
		ListenAddress:          envOrDefault("LISTEN_ADDRESS", ":8080"),
		SiteDirectory:          envOrDefault("SITE_DIRECTORY", "/app/site"),
		PublicOrigin:           origin,
		HealthTimeout:          timeout,
		StatusCacheTTL:         cacheTTL,
		COBOLCoreExecutable:    envOrDefault("COBOL_CORE_EXECUTABLE", "/app/gamepage-core"),
		COBOLRegistryPath:      registryPath,
		COBOLPolicyPath:        policyPath,
		MaintenanceMode:        envRaw("MAINTENANCE_MODE"),
		MinimumLaunchableGames: envRaw("MINIMUM_LAUNCHABLE_GAMES"),
		Routes:                 routes,
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
	minimum, err := strconv.Atoi(values["MINIMUM_LAUNCHABLE_GAMES"])
	if err != nil || minimum < 1 || minimum > maxGames {
		return runtimePolicy{}, fmt.Errorf("MINIMUM_LAUNCHABLE_GAMES must be between 1 and MAX_GAMES")
	}
	if values["OPTIONAL_FAILURES_ALLOW_READY"] != "true" {
		return runtimePolicy{}, fmt.Errorf("OPTIONAL_FAILURES_ALLOW_READY must be true")
	}
	if values["SLOW_GAMES_REMAIN_LAUNCHABLE"] != "true" {
		return runtimePolicy{}, fmt.Errorf("SLOW_GAMES_REMAIN_LAUNCHABLE must be true")
	}
	return runtimePolicy{
		StatusCacheTTL:        cacheTTL,
		MaxGames:              maxGames,
		MinimumLaunchableGames: minimum,
	}, nil
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
		if len(parts) != 12 {
			return nil, fmt.Errorf("invalid route record %q", line)
		}
		for index := range parts {
			parts[index] = strings.TrimSpace(parts[index])
			if !isSafeCOBOLField(parts[index]) {
				return nil, fmt.Errorf("route field contains a forbidden JSON or record character")
			}
		}
		defaultMaxLatency, err := strconv.Atoi(parts[11])
		if err != nil || defaultMaxLatency < 1 || defaultMaxLatency > 999999 {
			return nil, fmt.Errorf("invalid default latency for route %q", parts[0])
		}
		route := registryRoute{
			key: parts[0], name: parts[1], prefix: parts[2], upstreamEnv: parts[3],
			fallback: parts[4], healthPath: parts[5], enabledEnv: parts[6],
			requiredEnv: parts[7], latencyEnv: parts[8], defaultEnabled: parts[9],
			defaultRequired: parts[10], defaultMaxLatency: defaultMaxLatency,
		}
		if !isIdentifier(route.key) {
			return nil, fmt.Errorf("invalid route key %q", route.key)
		}
		if route.name == "" || !strings.HasPrefix(route.prefix, "/play/") || strings.HasSuffix(route.prefix, "/") {
			return nil, fmt.Errorf("invalid name or prefix for route %q", route.key)
		}
		if !isEnvironmentName(route.upstreamEnv) || !isEnvironmentName(route.enabledEnv) ||
			!isEnvironmentName(route.requiredEnv) || !isEnvironmentName(route.latencyEnv) ||
			!strings.HasPrefix(route.healthPath, "/") {
			return nil, fmt.Errorf("invalid environment name or health path for route %q", route.key)
		}
		if route.defaultEnabled != "Y" && route.defaultEnabled != "N" {
			return nil, fmt.Errorf("invalid enabled default for route %q", route.key)
		}
		if route.defaultRequired != "Y" && route.defaultRequired != "N" {
			return nil, fmt.Errorf("invalid required default for route %q", route.key)
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

func envRaw(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}
