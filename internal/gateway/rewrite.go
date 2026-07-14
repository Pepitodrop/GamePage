package gateway

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const maxRewriteBodyBytes = 16 << 20

const (
	gamePageStylesheet = `<link rel="stylesheet" href="/assets/gamepage-nav.css">`
	gamePageBackLink   = `<a class="gamepage-back-link" href="/" aria-label="Back to Game Mainframe"><span aria-hidden="true">←</span> Game Mainframe</a>`
)

func rewriteResponse(response *http.Response, prefix string, upstream *url.URL) error {
	rewriteLocation(response.Header, prefix, upstream)
	rewriteCookies(response.Header, prefix)
	rewriteContentSecurityPolicy(response.Header)

	contentType := response.Header.Get("Content-Type")
	if response.Body == nil || !isRewritableContentType(contentType) {
		return nil
	}
	if encoding := strings.TrimSpace(response.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return fmt.Errorf("cannot rewrite encoded upstream response (%s)", encoding)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxRewriteBodyBytes+1))
	if err != nil {
		return fmt.Errorf("read upstream response: %w", err)
	}
	if len(body) > maxRewriteBodyBytes {
		return fmt.Errorf("upstream text response exceeds %d bytes", maxRewriteBodyBytes)
	}
	if err := response.Body.Close(); err != nil {
		return fmt.Errorf("close upstream response: %w", err)
	}

	rewrittenText := rewriteRootPaths(string(body), prefix)
	if isHTMLContentType(contentType) {
		rewrittenText = injectGamePageChrome(rewrittenText)
	}
	rewritten := []byte(rewrittenText)
	response.Body = io.NopCloser(bytes.NewReader(rewritten))
	response.ContentLength = int64(len(rewritten))
	response.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
	response.Header.Del("ETag")
	response.Header.Del("Content-MD5")
	return nil
}

func isRewritableContentType(contentType string) bool {
	contentType = normalizedContentType(contentType)
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	switch contentType {
	case "application/javascript", "application/json", "application/manifest+json", "application/xhtml+xml", "application/xml", "image/svg+xml":
		return true
	default:
		return false
	}
}

func isHTMLContentType(contentType string) bool {
	switch normalizedContentType(contentType) {
	case "text/html", "application/xhtml+xml":
		return true
	default:
		return false
	}
}

func normalizedContentType(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
}

func injectGamePageChrome(input string) string {
	if !strings.Contains(input, `href="/assets/gamepage-nav.css"`) {
		input = insertBeforeClosingTag(input, "head", gamePageStylesheet)
	}
	if !strings.Contains(input, `class="gamepage-back-link"`) {
		input = insertAfterOpeningTag(input, "body", gamePageBackLink)
	}
	return input
}

func insertBeforeClosingTag(input, tag, markup string) string {
	lowerInput := strings.ToLower(input)
	closingTag := "</" + strings.ToLower(tag) + ">"
	index := strings.Index(lowerInput, closingTag)
	if index < 0 {
		return input
	}
	return input[:index] + markup + input[index:]
}

func insertAfterOpeningTag(input, tag, markup string) string {
	lowerInput := strings.ToLower(input)
	openingTag := "<" + strings.ToLower(tag)
	start := strings.Index(lowerInput, openingTag)
	if start < 0 {
		return input
	}
	endRelative := strings.Index(lowerInput[start:], ">")
	if endRelative < 0 {
		return input
	}
	end := start + endRelative + 1
	return input[:end] + markup + input[end:]
}

func rewriteContentSecurityPolicy(header http.Header) {
	policies := header.Values("Content-Security-Policy")
	if len(policies) == 0 {
		return
	}

	header.Del("Content-Security-Policy")
	for _, policy := range policies {
		header.Add("Content-Security-Policy", ensureStyleSourceSelf(policy))
	}
}

func ensureStyleSourceSelf(policy string) string {
	directives := strings.Split(policy, ";")
	foundStyleSource := false

	for index, directive := range directives {
		fields := strings.Fields(directive)
		if len(fields) == 0 || !strings.EqualFold(fields[0], "style-src") {
			continue
		}
		foundStyleSource = true
		for _, source := range fields[1:] {
			if source == "'self'" {
				return policy
			}
		}
		directives[index] = strings.TrimSpace(directive) + " 'self'"
	}

	if foundStyleSource {
		return strings.Join(directives, ";")
	}

	trimmed := strings.TrimSpace(policy)
	if trimmed == "" {
		return "style-src 'self'"
	}
	if !strings.HasSuffix(trimmed, ";") {
		trimmed += ";"
	}
	return trimmed + " style-src 'self'"
}

func rewriteRootPaths(input, prefix string) string {
	// Trump vs. Shakespeare builds its WebSocket and share URLs from template
	// literals rather than simple quoted root paths.
	input = strings.ReplaceAll(
		input,
		"${protocol}//${location.host}/",
		"${protocol}//${location.host}"+prefix+"/",
	)
	input = strings.ReplaceAll(
		input,
		"${location.origin}/",
		"${location.origin}"+prefix+"/",
	)

	input = prefixQuotedRootPaths(input, prefix)
	input = strings.ReplaceAll(input, "url(/", "url("+prefix+"/")
	input = strings.ReplaceAll(input, "sourceMappingURL=/", "sourceMappingURL="+prefix+"/")
	return input
}

func prefixQuotedRootPaths(input, prefix string) string {
	var output strings.Builder
	output.Grow(len(input) + 128)

	for index := 0; index < len(input); index++ {
		current := input[index]
		output.WriteByte(current)
		if current != '\'' && current != '"' && current != '`' {
			continue
		}
		if index+1 >= len(input) || input[index+1] != '/' {
			continue
		}
		if index+2 < len(input) && input[index+2] == '/' {
			continue // Protocol-relative URL.
		}
		remainder := input[index+1:]
		if remainder == prefix || strings.HasPrefix(remainder, prefix+"/") {
			continue
		}
		output.WriteString(prefix)
	}
	return output.String()
}

func rewriteLocation(header http.Header, prefix string, upstream *url.URL) {
	location := header.Get("Location")
	if location == "" {
		return
	}

	parsed, err := url.Parse(location)
	if err != nil {
		return
	}

	if parsed.IsAbs() {
		if !strings.EqualFold(parsed.Host, upstream.Host) {
			return
		}
		parsed.Scheme = ""
		parsed.Host = ""
	}

	if strings.HasPrefix(parsed.Path, "/") && !strings.HasPrefix(parsed.Path, prefix+"/") && parsed.Path != prefix {
		parsed.Path = prefix + parsed.Path
		header.Set("Location", parsed.String())
	}
}

func rewriteCookies(header http.Header, prefix string) {
	cookies := header.Values("Set-Cookie")
	if len(cookies) == 0 {
		return
	}
	header.Del("Set-Cookie")
	for _, cookie := range cookies {
		segments := strings.Split(cookie, ";")
		for index, segment := range segments {
			trimmed := strings.TrimSpace(segment)
			if !strings.HasPrefix(strings.ToLower(trimmed), "path=") {
				continue
			}
			path := strings.TrimSpace(trimmed[len("path="):])
			if path == "" {
				path = "/"
			}
			if path != prefix && !strings.HasPrefix(path, prefix+"/") {
				path = prefix + "/" + strings.TrimPrefix(path, "/")
			}
			segments[index] = " Path=" + path
		}
		header.Add("Set-Cookie", strings.Join(segments, ";"))
	}
}
