package cmd

import (
	"path/filepath"
	"testing"

	"mihoctl/internal/app"
	"mihoctl/internal/config"
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

	application := newTestApplication(t)
	group, proxy, err := resolveProxySelection(application, groups, "2", "1")
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

	application := newTestApplication(t)
	group, proxy, err := resolveProxySelection(application, groups, "", "2")
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

func TestSelectableNodeLabelUsesListIndex(t *testing.T) {
	t.Parallel()

	nodes := []string{"DIRECT", "REJECT", "JP"}

	label := selectableNodeLabel(nodes, "REJECT", "无")
	if label != "[2] REJECT" {
		t.Fatalf("label = %q, want %q", label, "[2] REJECT")
	}
}

func TestOrderedDelayNamesKeepsListOrder(t *testing.T) {
	t.Parallel()

	nodes := []string{"B", "A", "C"}
	delays := map[string]int{
		"A": 10,
		"B": 20,
		"C": 30,
	}

	ordered := orderedDelayNames(nodes, delays)
	want := []string{"B", "A", "C"}
	if len(ordered) != len(want) {
		t.Fatalf("len(ordered) = %d, want %d", len(ordered), len(want))
	}
	for i := range want {
		if ordered[i] != want[i] {
			t.Fatalf("ordered[%d] = %q, want %q", i, ordered[i], want[i])
		}
	}
}

func newTestApplication(t *testing.T) *app.App {
	t.Helper()

	application, err := app.New(config.BootstrapOptions{
		Lang:       "en-US",
		ConfigPath: filepath.Join(t.TempDir(), "config.json"),
	})
	if err != nil {
		t.Fatalf("app.New returned error: %v", err)
	}
	return application
}
