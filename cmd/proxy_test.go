package cmd

import (
	"testing"

	"mihoctl/internal/mihomo"
)

func TestResolveProxySelectionByIndex(t *testing.T) {
	t.Parallel()

	groups := []mihomo.ProxyGroup{
		{
			Name: "Auto",
			All:  []string{"HK", "JP"},
		},
		{
			Name: "Fallback",
			All:  []string{"SG", "US"},
		},
	}

	group, proxy, err := resolveProxySelection(groups, "2", "1")
	if err != nil {
		t.Fatalf("resolveProxySelection returned error: %v", err)
	}
	if group != "Fallback" {
		t.Fatalf("group = %q, want %q", group, "Fallback")
	}
	if proxy != "SG" {
		t.Fatalf("proxy = %q, want %q", proxy, "SG")
	}
}

func TestResolveProxySelectionUsesFirstNonGlobalGroupByDefault(t *testing.T) {
	t.Parallel()

	groups := []mihomo.ProxyGroup{
		{
			Name: "GLOBAL",
			All:  []string{"DIRECT", "Proxy"},
		},
		{
			Name: "Auto",
			All:  []string{"HK", "JP"},
		},
		{
			Name: "Fallback",
			All:  []string{"SG", "US"},
		},
	}

	group, proxy, err := resolveProxySelection(groups, "", "2")
	if err != nil {
		t.Fatalf("resolveProxySelection returned error: %v", err)
	}
	if group != "Auto" {
		t.Fatalf("group = %q, want %q", group, "Auto")
	}
	if proxy != "JP" {
		t.Fatalf("proxy = %q, want %q", proxy, "JP")
	}
}
