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
			"Content-Type": {"text/html; charset=utf-8"},
			"Location":     {"/room/ABC"},
			"Set-Cookie":   {"session=abc; Path=/; HttpOnly; SameSite=Lax"},
			"ETag":         {`"old"`},
		},
		Body: io.NopCloser(strings.NewReader(`<!doctype html><html><head><title>Race</title></head><body><form action="/action"></form></body></html>`)),
	}

	if err := rewriteResponse(response, "/play/race", upstream); err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	bodyText := string(body)
	for _, expected := range []string{
		`action="/play/race/action"`,
		`href="/favicon.svg"`,
		`href="/assets/gamepage-nav.css"`,
		`class="gamepage-back-link"`,
		`href="/"`,
	} {
		if !strings.Contains(bodyText, expected) {
			t.Fatalf("expected %q in rewritten body:\n%s", expected, bodyText)
		}
	}
	for _, unexpected := range []string{
		`href="/play/race/favicon.svg"`,
		`href="/play/race/assets/gamepage-nav.css"`,
	} {
		if strings.Contains(bodyText, unexpected) {
			t.Fatalf("did not expect %q in rewritten body:\n%s", unexpected, bodyText)
		}
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
	input := `<!doctype html><html><head></head><body><main>Game</main></body></html>`
	once := injectGamePageChrome(input)
	twice := injectGamePageChrome(once)
	if once != twice {
		t.Fatalf("expected idempotent injection:\nonce:  %s\ntwice: %s", once, twice)
	}
}

func TestAlreadyPrefixedPathIsNotDuplicated(t *testing.T) {
	input := `<a href="/play/golf/assets/app.js"></a>`
	actual := rewriteRootPaths(input, "/play/golf")
	if actual != input {
		t.Fatalf("expected unchanged path, got %s", actual)
	}
}
