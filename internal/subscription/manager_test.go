package subscription

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"mihoctl/internal/config"
	"mihoctl/internal/mihomo"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFetchSubscriptionRetriesWithClientUserAgent(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Subscriptions: []config.Subscription{
			{
				Name: "demo",
				URL:  "https://example.com/sub.yaml",
			},
		},
	}
	manager := NewManager(cfg, config.Paths{}, mihomo.NewClient("http://127.0.0.1:9090", ""), io.Discard, "")
	wantUA := buildClashVergeCompatibleUserAgent()
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.UserAgent() != wantUA {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(strings.NewReader("Unauthorized")),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("mixed-port: 7890\n")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, cfg.Subscriptions[0].URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext error = %v", err)
	}

	result, err := manager.fetchSubscription(context.Background(), client, req, cfg.Subscriptions[0])
	if err != nil {
		t.Fatalf("fetchSubscription returned error: %v", err)
	}
	defer result.response.Body.Close()

	if result.userAgent != wantUA {
		t.Fatalf("result.userAgent = %q, want %q", result.userAgent, wantUA)
	}

	cfg.Subscriptions[0].UserAgent = result.userAgent
	manager.updateSubscriptionMeta(cfg.Subscriptions[0])
	if cfg.Subscriptions[0].UserAgent != wantUA {
		t.Fatalf("stored user agent = %q, want %q", cfg.Subscriptions[0].UserAgent, wantUA)
	}
}

func TestApplySubscriptionHeadersUsesBasicAuthFromURL(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest(http.MethodGet, "https://demo:secret@example.com/sub.yaml", nil)
	if err != nil {
		t.Fatalf("NewRequest error = %v", err)
	}

	applySubscriptionHeaders(req, buildFlClashCompatibleUserAgent())

	if got := req.Header.Get("Authorization"); got != "Basic ZGVtbzpzZWNyZXQ=" {
		t.Fatalf("Authorization = %q, want %q", got, "Basic ZGVtbzpzZWNyZXQ=")
	}
	if got := req.Header.Get("User-Agent"); got != buildFlClashCompatibleUserAgent() {
		t.Fatalf("User-Agent = %q, want %q", got, buildFlClashCompatibleUserAgent())
	}
}

func TestSetUserAgent(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Subscriptions: []config.Subscription{
			{
				Name: "demo",
				URL:  "https://example.com/sub.yaml",
			},
		},
	}
	manager := NewManager(cfg, config.Paths{}, mihomo.NewClient("http://127.0.0.1:9090", ""), io.Discard, "")

	entry, err := manager.SetUserAgent("demo", "custom-agent/1.0")
	if err != nil {
		t.Fatalf("SetUserAgent returned error: %v", err)
	}
	if entry.UserAgent != "custom-agent/1.0" {
		t.Fatalf("entry.UserAgent = %q, want %q", entry.UserAgent, "custom-agent/1.0")
	}
	if cfg.Subscriptions[0].UserAgent != "custom-agent/1.0" {
		t.Fatalf("stored user agent = %q, want %q", cfg.Subscriptions[0].UserAgent, "custom-agent/1.0")
	}
}

func TestFindSubscriptionByIndex(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Subscriptions: []config.Subscription{
			{Name: "alpha", URL: "https://example.com/a.yaml"},
			{Name: "beta", URL: "https://example.com/b.yaml"},
		},
	}
	manager := NewManager(cfg, config.Paths{SubDir: t.TempDir()}, mihomo.NewClient("http://127.0.0.1:9090", ""), io.Discard, "")

	entry, err := manager.findSubscription("2")
	if err != nil {
		t.Fatalf("findSubscription returned error: %v", err)
	}
	if entry.Name != "beta" {
		t.Fatalf("entry.Name = %q, want %q", entry.Name, "beta")
	}
}
