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

func TestRewriteResponse(t *testing.T) {
	upstream, _ := url.Parse("http://crazy-race:8080")
	response := &http.Response{
		Header: http.Header{
			"Content-Type":            {"text/html; charset=utf-8"},
			"Content-Security-Policy": {"default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; form-action 'self'"},
			"Location":                {"/room/ABC"},
			"Set-Cookie":              {"session=abc; Path=/; HttpOnly; SameSite=Lax"},
			"ETag":                    {`"old"`},
		},
		Body: io.NopCloser(strings.NewReader(`<!doctype html><html><head><link rel="icon" href="data:image/svg+xml,%3Csvg%3E"><title>Race</title></head><body><form action="/action"></form></body></html>`)),
	}

	if err := rewriteResponse(response, "/play/race", upstream); err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	bodyText := string(body)
	for _, expected := range []string{
		`action="/play/race/action"`,
		`href="data:image/svg+xml,%3Csvg%3E"`,
		`href="/assets/gamepage-nav.css"`,
		`class="gamepage-back-link"`,
		`href="/"`,
	} {
		if !strings.Contains(bodyText, expected) {
			t.Fatalf("expected %q in rewritten body:\n%s", expected, bodyText)
		}
	}
	for _, unexpected := range []string{
		`href="/favicon.svg"`,
		`href="/play/race/assets/gamepage-nav.css"`,
	} {
		if strings.Contains(bodyText, unexpected) {
			t.Fatalf("did not expect %q in rewritten body:\n%s", unexpected, bodyText)
		}
	}
	if got := response.Header.Get("Content-Security-Policy"); !strings.Contains(got, "style-src 'unsafe-inline' 'self'") {
		t.Fatalf("expected navigation stylesheet to be allowed, got %q", got)
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
}

func TestNativeGameFaviconsArePreserved(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		iconHTML string
		expected string
	}{
		{
			name:     "Trump vs Shakespeare",
			prefix:   "/play/trump",
			iconHTML: `<link rel="icon" href="/static/icon.svg?v=1.0.2" type="image/svg+xml">`,
			expected: `href="/play/trump/static/icon.svg?v=1.0.2"`,
		},
		{
			name:     "Crazy Mini Golf",
			prefix:   "/play/golf",
			iconHTML: `<link rel="icon" type="image/svg+xml" href="./favicon.svg">`,
			expected: `href="./favicon.svg"`,
		},
		{
			name:     "Crazy Race",
			prefix:   "/play/race",
			iconHTML: `<link rel="icon" href="data:image/svg+xml,%3Csvg%3E">`,
			expected: `href="data:image/svg+xml,%3Csvg%3E"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := `<!doctype html><html><head>` + test.iconHTML + `</head><body><main>Game</main></body></html>`
			actual := injectGamePageChrome(rewriteRootPaths(input, test.prefix))
			if !strings.Contains(actual, test.expected) {
				t.Fatalf("expected native icon %q in:\n%s", test.expected, actual)
			}
			if strings.Contains(actual, `href="/favicon.svg"`) {
				t.Fatalf("GamePage fallback must not replace the native icon:\n%s", actual)
			}
			if !strings.Contains(actual, `class="gamepage-back-link"`) {
				t.Fatalf("expected launcher back link in:\n%s", actual)
			}
		})
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
	if strings.Contains(bodyText, "gamepage-back-link") {
		t.Fatalf("navigation must not be injected into CSS: %s", bodyText)
	}
}

func TestInjectGamePageChromeIsIdempotent(t *testing.T) {
	input := `<!doctype html><html><head><link rel="icon" href="./favicon.svg"></head><body><main>Game</main></body></html>`
	once := injectGamePageChrome(input)
	twice := injectGamePageChrome(once)
	if once != twice {
		t.Fatalf("expected idempotent injection:\nonce:  %s\ntwice: %s", once, twice)
	}
}

func TestEnsureStyleSourceSelf(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "adds self to existing style directive",
			input:    "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'",
			expected: "style-src 'unsafe-inline' 'self'",
		},
		{
			name:     "keeps existing self",
			input:    "default-src 'none'; style-src 'self' 'unsafe-inline'",
			expected: "default-src 'none'; style-src 'self' 'unsafe-inline'",
		},
		{
			name:     "adds style directive when missing",
			input:    "default-src 'none'; img-src data:",
			expected: "default-src 'none'; img-src data:; style-src 'self'",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := ensureStyleSourceSelf(test.input)
			if !strings.Contains(actual, test.expected) {
				t.Fatalf("expected %q in %q", test.expected, actual)
			}
		})
	}
}

func TestAlreadyPrefixedPathIsNotDuplicated(t *testing.T) {
	input := `<a href="/play/golf/assets/app.js"></a>`
	actual := rewriteRootPaths(input, "/play/golf")
	if actual != input {
		t.Fatalf("expected unchanged path, got %s", actual)
	}
}
