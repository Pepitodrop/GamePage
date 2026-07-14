package gateway

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRewriteRootPaths(t *testing.T) {
	input := `<a href="/">home</a><form action="/create"><script>fetch('/api/rooms');const ws=` + "`${protocol}//${location.host}/ws/ABC`;" + `</script><a href="//example.com/x">external</a>`
	actual := rewriteRootPaths(input, "/play/trump")
	for _, expected := range []string{
		`href="/play/trump/"`,
		`action="/play/trump/create"`,
		`fetch('/play/trump/api/rooms')`,
		"`${protocol}//${location.host}/play/trump/ws/ABC`",
		`href="//example.com/x"`,
	} {
		if !strings.Contains(actual, expected) {
			t.Fatalf("expected %q in rewritten body:\n%s", expected, actual)
		}
	}
}

func TestRewriteResponseInjectsReliableGameFaviconAndNavigation(t *testing.T) {
	upstream, _ := url.Parse("http://crazy-race:8080")
	response := &http.Response{
		Header: http.Header{
			"Content-Type":            {"text/html; charset=utf-8"},
			"Content-Security-Policy": {"default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; form-action 'self'"},
			"Location":                {"/room/ABC"},
			"Set-Cookie":              {"session=abc; Path=/; HttpOnly; SameSite=Lax"},
			"ETag":                    {`"old"`},
		},
		Body: io.NopCloser(strings.NewReader(`<!doctype html><html><head><title>Race</title><link rel="icon" href="/favicon.svg"></head><body><form action="/action"></form></body></html>`)),
	}

	if err := rewriteResponse(response, "/play/race", upstream); err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	bodyText := string(body)
	for _, expected := range []string{
		`action="/play/race/action"`,
		`href="/play/race/favicon.svg"`,
		`href="/assets/gamepage-nav.css"`,
		`class="gamepage-back-link"`,
		`href="/"`,
		`data-gamepage-favicon="race"`,
		`href="data:image/svg+xml,`,
	} {
		if !strings.Contains(bodyText, expected) {
			t.Fatalf("expected %q in rewritten body:\n%s", expected, bodyText)
		}
	}
	if strings.LastIndex(bodyText, `data-gamepage-favicon="race"`) < strings.LastIndex(bodyText, `href="/play/race/favicon.svg"`) {
		t.Fatalf("reliable favicon must be the final icon candidate:\n%s", bodyText)
	}
	if got := response.Header.Get("Location"); got != "/play/race/room/ABC" {
		t.Fatalf("unexpected location: %s", got)
	}
	if got := response.Header.Get("Set-Cookie"); !strings.Contains(got, "Path=/play/race/") {
		t.Fatalf("unexpected cookie: %s", got)
	}
	if got := response.Header.Get("ETag"); got != "" {
		t.Fatalf("etag should be removed, got %s", got)
	}
	policy := response.Header.Get("Content-Security-Policy")
	for _, expected := range []string{"style-src 'unsafe-inline' 'self'", "img-src 'self' data:"} {
		if !strings.Contains(policy, expected) {
			t.Fatalf("expected CSP %q in %q", expected, policy)
		}
	}
}

func TestGameFaviconsAreDistinctAndBranded(t *testing.T) {
	icons := map[string]string{
		"trump": gameFaviconMarkup("/play/trump"),
		"golf":  gameFaviconMarkup("/play/golf"),
		"race":  gameFaviconMarkup("/play/race"),
	}
	seen := map[string]bool{}
	for name, markup := range icons {
		if !strings.Contains(markup, `data-gamepage-favicon="`+name+`"`) {
			t.Fatalf("%s favicon is not labelled correctly: %s", name, markup)
		}
		if !strings.Contains(markup, `href="data:image/svg+xml,`) {
			t.Fatalf("%s favicon is not self-contained: %s", name, markup)
		}
		if seen[markup] {
			t.Fatalf("%s favicon duplicates another game", name)
		}
		seen[markup] = true
	}
}

func TestNonHTMLResponseDoesNotInjectGamePageChrome(t *testing.T) {
	upstream, _ := url.Parse("http://crazy-mini-golf:8080")
	response := &http.Response{
		Header: http.Header{"Content-Type": {"text/css"}},
		Body:   io.NopCloser(strings.NewReader(`body{background:url(/background.svg)}`)),
	}

	if err := rewriteResponse(response, "/play/golf", upstream); err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	bodyText := string(body)
	if bodyText != `body{background:url(/play/golf/background.svg)}` {
		t.Fatalf("unexpected rewritten CSS: %s", bodyText)
	}
	if strings.Contains(bodyText, "gamepage-back-link") || strings.Contains(bodyText, "gamepage-favicon") {
		t.Fatalf("navigation and favicon must not be injected into CSS: %s", bodyText)
	}
}

func TestInjectGamePageChromeIsIdempotent(t *testing.T) {
	input := `<!doctype html><html><head></head><body><main>Game</main></body></html>`
	once := injectGamePageChrome(input, "/play/golf")
	twice := injectGamePageChrome(once, "/play/golf")
	if once != twice {
		t.Fatalf("expected idempotent injection:\nonce:  %s\ntwice: %s", once, twice)
	}
}

func TestEnsureCSPSourceAddsOrPreservesSource(t *testing.T) {
	policy := "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'"
	policy = ensureCSPSource(policy, "style-src", "'self'")
	policy = ensureCSPSource(policy, "img-src", "data:")
	policy = ensureCSPSource(policy, "img-src", "data:")
	if strings.Count(policy, "data:") != 1 {
		t.Fatalf("source should be added once: %s", policy)
	}
	if !strings.Contains(policy, "style-src 'unsafe-inline' 'self'") {
		t.Fatalf("style source missing: %s", policy)
	}
}

func TestAlreadyPrefixedPathIsNotDuplicated(t *testing.T) {
	input := `<a href="/play/golf/assets/app.js"></a>`
	actual := rewriteRootPaths(input, "/play/golf")
	if actual != input {
		t.Fatalf("expected unchanged path, got %s", actual)
	}
}
