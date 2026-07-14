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
		Body: io.NopCloser(strings.NewReader(`<form action="/action"></form>`)),
	}

	if err := rewriteResponse(response, "/play/race", upstream); err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	if string(body) != `<form action="/play/race/action"></form>` {
		t.Fatalf("unexpected body: %s", body)
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

func TestAlreadyPrefixedPathIsNotDuplicated(t *testing.T) {
	input := `<a href="/play/golf/assets/app.js"></a>`
	actual := rewriteRootPaths(input, "/play/golf")
	if actual != input {
		t.Fatalf("expected unchanged path, got %s", actual)
	}
}
