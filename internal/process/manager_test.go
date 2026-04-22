package process

import (
	"strings"
	"testing"
)

func TestEnsurePATHEntriesAddsMissingSbinPaths(t *testing.T) {
	env := []string{
		"HOME=/root",
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}

	updated := ensurePATHEntries(env, []string{"/usr/local/sbin", "/usr/sbin", "/sbin"})

	pathValue := ""
	for _, item := range updated {
		if strings.HasPrefix(item, "PATH=") {
			pathValue = strings.TrimPrefix(item, "PATH=")
			break
		}
	}
	if pathValue == "" {
		t.Fatalf("expected PATH to be present")
	}
	for _, expected := range []string{"/usr/local/bin", "/usr/bin", "/bin", "/usr/local/sbin", "/usr/sbin", "/sbin"} {
		if !strings.Contains(pathValue, expected) {
			t.Fatalf("expected PATH to contain %s, got %s", expected, pathValue)
		}
	}
}

func TestEnsurePATHEntriesDoesNotDuplicateValues(t *testing.T) {
	env := []string{
		"PATH=/usr/local/bin:/usr/sbin:/usr/bin:/sbin",
	}

	updated := ensurePATHEntries(env, []string{"/usr/sbin", "/sbin"})

	pathValue := strings.TrimPrefix(updated[0], "PATH=")
	counts := map[string]int{}
	for _, entry := range strings.Split(pathValue, ":") {
		counts[entry]++
	}
	if counts["/usr/sbin"] != 1 {
		t.Fatalf("expected /usr/sbin only once, got %s", pathValue)
	}
	if counts["/sbin"] != 1 {
		t.Fatalf("expected /sbin only once, got %s", pathValue)
	}
}
