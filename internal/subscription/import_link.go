package subscription

import (
	"encoding/base64"
	"net/url"
	"regexp"
	"strings"
)

var embeddedHTTPURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

type ImportLink struct {
	Original  string
	URL       string
	Name      string
	UserAgent string
	Wrapped   bool
}

func ResolveImportLink(raw string) (ImportLink, error) {
	trimmed := strings.TrimSpace(raw)
	if resolved, ok := normalizeHTTPURL(trimmed); ok {
		return ImportLink{
			Original: trimmed,
			URL:      resolved,
		}, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ImportLink{}, err
	}

	candidates := collectImportCandidates(trimmed, parsed)
	for _, candidate := range candidates {
		if resolved, ok := resolveEmbeddedHTTPURL(candidate, 0); ok {
			return ImportLink{
				Original:  trimmed,
				URL:       resolved,
				UserAgent: extractImportUserAgent(parsed),
				Name:      extractImportName(parsed),
				Wrapped:   true,
			}, nil
		}
	}
	return ImportLink{}, url.InvalidHostError(trimmed)
}

func extractImportName(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}

	query := parsed.Query()
	for _, key := range []string{"name", "title", "profile_name", "profile", "tag"} {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func extractImportUserAgent(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}

	query := parsed.Query()
	for _, key := range []string{"user-agent", "user_agent", "ua", "agent"} {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func collectImportCandidates(raw string, parsed *url.URL) []string {
	candidates := make([]string, 0, 16)
	seen := map[string]struct{}{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	query := parsed.Query()
	for _, key := range []string{"url", "target", "subscription", "sub", "config", "profile_url", "endpoint", "remote", "remote_url"} {
		add(query.Get(key))
	}
	for key, values := range query {
		switch key {
		case "name", "title", "profile_name", "profile", "tag":
			continue
		}
		for _, value := range values {
			add(value)
		}
	}

	add(parsed.Opaque)
	add(parsed.Host)
	add(strings.TrimPrefix(parsed.Path, "/"))
	add(strings.TrimPrefix(parsed.Host+parsed.Path, "/"))
	add(parsed.Fragment)
	add(raw)
	return candidates
}

func resolveEmbeddedHTTPURL(raw string, depth int) (string, bool) {
	if depth > 5 {
		return "", false
	}

	candidate := trimCandidate(raw)
	if resolved, ok := normalizeHTTPURL(candidate); ok {
		return resolved, true
	}

	for _, match := range embeddedHTTPURLPattern.FindAllString(candidate, -1) {
		if resolved, ok := normalizeHTTPURL(trimCandidate(match)); ok {
			return resolved, true
		}
	}

	// 一键导入链接经常会把真实订阅地址再包一层 URL Encode 或 Base64。
	for _, next := range transformedCandidates(candidate) {
		if resolved, ok := resolveEmbeddedHTTPURL(next, depth+1); ok {
			return resolved, true
		}
	}

	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", false
	}
	for _, next := range collectImportCandidates(candidate, parsed) {
		if next == candidate {
			continue
		}
		if resolved, ok := resolveEmbeddedHTTPURL(next, depth+1); ok {
			return resolved, true
		}
	}
	return "", false
}

func transformedCandidates(value string) []string {
	items := make([]string, 0, 6)
	seen := map[string]struct{}{}
	add := func(candidate string) {
		candidate = trimCandidate(candidate)
		if candidate == "" || candidate == value {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		items = append(items, candidate)
	}

	if decoded, err := url.QueryUnescape(value); err == nil {
		add(decoded)
	}
	if decoded, err := url.PathUnescape(value); err == nil {
		add(decoded)
	}
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if decoded, err := encoding.DecodeString(value); err == nil {
			add(string(decoded))
		}
	}
	return items
}

func normalizeHTTPURL(value string) (string, bool) {
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return "", false
	}
	if parsed.Host == "" {
		return "", false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.String(), true
	default:
		return "", false
	}
}

func trimCandidate(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	value = strings.Trim(value, "()[]{}<>")
	return strings.TrimSpace(value)
}
