package gateway

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config contains all runtime settings for the launcher and reverse proxy.
type Config struct {
	ListenAddress string
	SiteDirectory string
	PublicOrigin  string
	HealthTimeout time.Duration
	Routes        []RouteConfig
}

// RouteConfig describes one game mounted below the public launcher domain.
type RouteConfig struct {
	Key        string
	Name       string
	Prefix     string
	Upstream   *url.URL
	HealthPath string
}

// LoadConfig reads and validates environment-based configuration.
func LoadConfig() (Config, error) {
	timeout, err := time.ParseDuration(envOrDefault("HEALTH_TIMEOUT", "3s"))
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("HEALTH_TIMEOUT must be a positive duration")
	}

	origin := strings.TrimRight(envOrDefault("PUBLIC_ORIGIN", "https://game.luisbenedikt.de"), "/")
	if _, err := parseHTTPURL(origin); err != nil {
		return Config{}, fmt.Errorf("PUBLIC_ORIGIN: %w", err)
	}

	routeInputs := []struct {
		key, name, prefix, envName, fallback, healthPath string
	}{
		{"trump", "Trump vs. Shakespeare", "/play/trump", "TRUMP_UPSTREAM", "http://trump-vs-shakespeare:8000", "/readyz"},
		{"golf", "Crazy Mini Golf", "/play/golf", "GOLF_UPSTREAM", "http://crazy-mini-golf:8080", "/healthz"},
		{"race", "Crazy Race", "/play/race", "RACE_UPSTREAM", "http://crazy-race:8080", "/health"},
	}

	routes := make([]RouteConfig, 0, len(routeInputs))
	for _, input := range routeInputs {
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
		ListenAddress: envOrDefault("LISTEN_ADDRESS", ":8080"),
		SiteDirectory: envOrDefault("SITE_DIRECTORY", "/app/site"),
		PublicOrigin:  origin,
		HealthTimeout: timeout,
		Routes:        routes,
	}, nil
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
