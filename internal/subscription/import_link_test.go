package subscription

import "testing"

func TestResolveImportLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		raw       string
		wantURL   string
		wantName  string
		wantUA    string
		wrapped   bool
		shouldErr bool
	}{
		{
			name:     "direct http url",
			raw:      "https://example.com/sub.yaml?token=abc",
			wantURL:  "https://example.com/sub.yaml?token=abc",
			wrapped:  false,
			wantName: "",
		},
		{
			name:     "clash import query url",
			raw:      "clash://install-config?url=https%3A%2F%2Fexample.com%2Fsub.yaml%3Ftoken%3Dabc&name=MyNode&ua=FlClash",
			wantURL:  "https://example.com/sub.yaml?token=abc",
			wantName: "MyNode",
			wantUA:   "FlClash",
			wrapped:  true,
		},
		{
			name:     "mihomo import with plain query value",
			raw:      "mihomo://install-config?subscription=https://example.com/api/v1/client/subscribe?token=abc",
			wantURL:  "https://example.com/api/v1/client/subscribe?token=abc",
			wantName: "",
			wrapped:  true,
		},
		{
			name:     "base64 wrapped url",
			raw:      "clash://aHR0cHM6Ly9leGFtcGxlLmNvbS9zdWIueWFtbD90b2tlbj1hYmM=",
			wantURL:  "https://example.com/sub.yaml?token=abc",
			wantName: "",
			wrapped:  true,
		},
		{
			name:      "invalid import link",
			raw:       "clash://install-config?name=NoURL",
			shouldErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := ResolveImportLink(tc.raw)
			if tc.shouldErr {
				if err == nil {
					t.Fatalf("expected an error for %q", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveImportLink(%q) error = %v", tc.raw, err)
			}
			if result.URL != tc.wantURL {
				t.Fatalf("ResolveImportLink(%q) URL = %q, want %q", tc.raw, result.URL, tc.wantURL)
			}
			if result.Name != tc.wantName {
				t.Fatalf("ResolveImportLink(%q) Name = %q, want %q", tc.raw, result.Name, tc.wantName)
			}
			if result.UserAgent != tc.wantUA {
				t.Fatalf("ResolveImportLink(%q) UserAgent = %q, want %q", tc.raw, result.UserAgent, tc.wantUA)
			}
			if result.Wrapped != tc.wrapped {
				t.Fatalf("ResolveImportLink(%q) Wrapped = %v, want %v", tc.raw, result.Wrapped, tc.wrapped)
			}
		})
	}
}
