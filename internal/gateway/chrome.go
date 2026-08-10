package gateway

import (
	"net/http"
	"strings"
)

const gamePageBackLink = `<a class="gamepage-back-link" href="/" aria-label="Back to Game Mainframe"><span aria-hidden="true">←</span> Game Mainframe</a>`

const (
	trumpFavicon = `data:image/svg+xml,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20viewBox='0%200%2064%2064'%3E%3Crect%20width='64'%20height='64'%20rx='14'%20fill='%23111620'/%3E%3Crect%20x='5'%20y='5'%20width='54'%20height='54'%20rx='11'%20fill='none'%20stroke='%23313a4b'%20stroke-width='2'/%3E%3Ctext%20x='9'%20y='42'%20font-family='Georgia,serif'%20font-size='29'%20font-weight='700'%20fill='%23f2b943'%3ET%3C/text%3E%3Ctext%20x='28'%20y='39'%20font-family='Arial,sans-serif'%20font-size='14'%20font-weight='700'%20fill='%23f4f1e8'%3Ev%3C/text%3E%3Ctext%20x='40'%20y='42'%20font-family='Georgia,serif'%20font-size='27'%20font-weight='700'%20fill='%23df5d73'%3ES%3C/text%3E%3C/svg%3E`
	golfFavicon  = `data:image/svg+xml,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20viewBox='0%200%2064%2064'%3E%3Crect%20width='64'%20height='64'%20rx='14'%20fill='%23081624'/%3E%3Crect%20x='5'%20y='5'%20width='54'%20height='54'%20rx='11'%20fill='none'%20stroke='%233eec9b'%20stroke-width='2'/%3E%3Cpath%20d='M39%2011v36'%20stroke='%23effff7'%20stroke-width='4'%20stroke-linecap='round'/%3E%3Cpath%20d='M41%2013l16%208-16%208z'%20fill='%23ff4f70'/%3E%3Cellipse%20cx='37'%20cy='50'%20rx='18'%20ry='4'%20fill='%23172d3d'/%3E%3Ccircle%20cx='19'%20cy='45'%20r='8'%20fill='%23f5fff9'/%3E%3Ccircle%20cx='17'%20cy='43'%20r='1.3'%20fill='%2390a4af'/%3E%3Ccircle%20cx='21'%20cy='47'%20r='1.3'%20fill='%2390a4af'/%3E%3C/svg%3E`
	raceFavicon  = `data:image/svg+xml,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20viewBox='0%200%2064%2064'%3E%3Crect%20width='64'%20height='64'%20rx='14'%20fill='%23091122'/%3E%3Crect%20x='5'%20y='5'%20width='54'%20height='54'%20rx='11'%20fill='none'%20stroke='%236bdcff'%20stroke-width='2'/%3E%3Cpath%20d='M16%2010v45'%20stroke='%23f5f7ff'%20stroke-width='4'%20stroke-linecap='round'/%3E%3Cpath%20d='M19%2013h36v28H19z'%20fill='%236bdcff'/%3E%3Cpath%20d='M19%2013h9v7h-9zm18%200h9v7h-9zm9%207h9v7h-9zm-18%200h9v7h-9zm-9%207h9v7h-9zm18%200h9v7h-9zm9%207h9v7h-9zm-18%200h9v7h-9z'%20fill='%23a88cff'/%3E%3C/svg%3E`
)

func injectGamePageChrome(input, prefix string) string {
	if !strings.Contains(input, `href="/assets/gamepage-nav.css"`) {
		input = insertBeforeClosingTag(input, "head", `<link rel="stylesheet" href="/assets/gamepage-nav.css">`)
	}
	if !strings.Contains(input, `data-gamepage-favicon=`) {
		input = insertBeforeClosingTag(input, "head", gameFaviconMarkup(prefix))
	}
	if !strings.Contains(input, `class="gamepage-back-link"`) {
		input = insertAfterOpeningTag(input, "body", gamePageBackLink)
	}
	return input
}

func gameFaviconMarkup(prefix string) string {
	var name, favicon string
	switch prefix {
	case "/play/trump":
		name, favicon = "trump", trumpFavicon
	case "/play/golf":
		name, favicon = "golf", golfFavicon
	case "/play/race":
		name, favicon = "race", raceFavicon
	default:
		name, favicon = "game", golfFavicon
	}
	return `<link rel="icon" type="image/svg+xml" sizes="any" data-gamepage-favicon="` + name + `" href="` + favicon + `">`
}

func allowInjectedAssets(header http.Header) {
	policy := header.Get("Content-Security-Policy")
	if strings.TrimSpace(policy) == "" {
		return
	}
	header.Set(
		"Content-Security-Policy",
		ensureCSPSource(ensureCSPSource(policy, "style-src", "'self'"), "img-src", "data:"),
	)
}

func ensureCSPSource(policy, directive, source string) string {
	parts := strings.Split(policy, ";")
	for index, part := range parts {
		fields := strings.Fields(part)
		if len(fields) == 0 || !strings.EqualFold(fields[0], directive) {
			continue
		}
		for _, existing := range fields[1:] {
			if strings.EqualFold(existing, source) {
				return strings.Join(parts, ";")
			}
		}
		parts[index] = strings.TrimSpace(part) + " " + source
		return strings.Join(parts, ";")
	}

	trimmed := strings.TrimSpace(policy)
	if trimmed != "" && !strings.HasSuffix(trimmed, ";") {
		trimmed += ";"
	}
	return trimmed + " " + directive + " " + source
}

func insertBeforeClosingTag(input, tag, markup string) string {
	if markup == "" {
		return input
	}
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
