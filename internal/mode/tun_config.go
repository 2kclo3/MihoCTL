package mode

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

const (
	tunManagedStart = "# >>> mihoctl managed tun >>>"
	tunManagedEnd   = "# <<< mihoctl managed tun <<<"
)

func upsertTunConfigFile(path string, enabled bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := upsertTunConfigContent(string(data), enabled, runtime.GOOS)
	if updated == string(data) {
		return nil
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}

func readTunEnabledFromConfigFile(path string) (*bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return readTunEnabledFromContent(string(data)), nil
}

// 只管理顶层 tun 段，避免为了一个小开关引入完整 YAML 解析依赖。
func upsertTunConfigContent(content string, enabled bool, goos string) string {
	block := renderTunManagedBlock(enabled, goos)

	if updated, ok := replaceManagedBlock(content, block); ok {
		content = updated
	} else if updated, ok := replaceTopLevelKeyBlock(content, "tun", block); ok {
		content = updated
	} else {
		content = appendManagedBlock(content, block)
	}

	// TUN 启用后需要 Mihomo 自身 DNS 参与解析/劫持；若订阅把 dns.enable 设为 false，
	// 常见现象就是 TUN 已开启但域名请求卡住。
	if enabled {
		content = ensureDNSEnabled(content)
	}
	return content
}

func readTunEnabledFromContent(content string) *bool {
	lines := splitConfigLines(content)
	start, end, ok := findTopLevelKeyRange(lines, "tun")
	if !ok {
		return nil
	}

	for i := start + 1; i < end; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") || !strings.HasPrefix(line, "enable:") {
			continue
		}
		value := normalizeScalarValue(strings.TrimPrefix(line, "enable:"))
		switch {
		case strings.EqualFold(value, "true"):
			enabled := true
			return &enabled
		case strings.EqualFold(value, "false"):
			enabled := false
			return &enabled
		}
	}
	return nil
}

func renderTunManagedBlock(enabled bool, goos string) string {
	lines := []string{
		tunManagedStart,
		"tun:",
		fmt.Sprintf("  enable: %t", enabled),
		"  stack: system",
		"  dns-hijack:",
		"    - any:53",
		"    - tcp://any:53",
		"  auto-route: true",
		"  auto-detect-interface: true",
		"  strict-route: true",
	}
	if goos == "linux" {
		lines = append(lines, "  auto-redirect: true")
	}
	lines = append(lines, tunManagedEnd)
	return strings.Join(lines, "\n") + "\n"
}

func ensureDNSEnabled(content string) string {
	lines := splitConfigLines(content)
	if len(lines) == 0 {
		return strings.Join([]string{
			"dns:",
			"  enable: true",
			"",
		}, "\n")
	}

	start, end, ok := findTopLevelKeyRange(lines, "dns")
	if !ok {
		return joinConfigSections(lines, []string{"", "dns:", "  enable: true"})
	}

	for i := start + 1; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, "enable:") {
			continue
		}
		indent := leadingIndent(lines[i])
		lines[i] = strings.Repeat(" ", indent) + "enable: true"
		return joinConfigSections(lines)
	}

	dnsLines := append([]string{}, lines[start:end]...)
	insertAt := 1
	if len(dnsLines) > 1 && strings.TrimSpace(dnsLines[1]) == "" {
		insertAt = 2
	}
	dnsLines = insertLines(dnsLines, insertAt, []string{"  enable: true"})
	return joinConfigSections(lines[:start], dnsLines, lines[end:])
}

func replaceManagedBlock(content, block string) (string, bool) {
	lines := splitConfigLines(content)
	start, end, ok := findManagedBlockRange(lines)
	if !ok {
		return "", false
	}
	return joinConfigSections(lines[:start], splitConfigLines(block), lines[end:]), true
}

func replaceTopLevelKeyBlock(content, key, block string) (string, bool) {
	lines := splitConfigLines(content)
	start, end, ok := findTopLevelKeyRange(lines, key)
	if !ok {
		return "", false
	}
	return joinConfigSections(lines[:start], splitConfigLines(block), lines[end:]), true
}

func appendManagedBlock(content, block string) string {
	lines := splitConfigLines(content)
	if len(lines) == 0 {
		return block
	}
	return joinConfigSections(lines, []string{""}, splitConfigLines(block))
}

func splitConfigLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func findManagedBlockRange(lines []string) (int, int, bool) {
	start := -1
	end := -1
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case tunManagedStart:
			start = i
		case tunManagedEnd:
			if start >= 0 {
				end = i + 1
				return start, end, true
			}
		}
	}
	return 0, 0, false
}

func findTopLevelKeyRange(lines []string, key string) (int, int, bool) {
	start := -1
	for i, line := range lines {
		if leadingIndent(line) != 0 {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), key+":") {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, 0, false
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if leadingIndent(lines[i]) == 0 {
			end = i
			break
		}
	}
	return start, end, true
}

func joinConfigSections(sections ...[]string) string {
	merged := make([]string, 0)
	for _, section := range sections {
		for _, line := range section {
			merged = append(merged, line)
		}
	}

	for len(merged) > 0 && merged[0] == "" {
		merged = merged[1:]
	}
	for len(merged) > 0 && merged[len(merged)-1] == "" {
		merged = merged[:len(merged)-1]
	}
	return strings.Join(merged, "\n") + "\n"
}

func insertLines(lines []string, index int, extra []string) []string {
	if index < 0 {
		index = 0
	}
	if index > len(lines) {
		index = len(lines)
	}
	result := make([]string, 0, len(lines)+len(extra))
	result = append(result, lines[:index]...)
	result = append(result, extra...)
	result = append(result, lines[index:]...)
	return result
}

func leadingIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func normalizeScalarValue(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.Index(value, "#"); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	value = strings.Trim(value, `"'`)
	if fields := strings.Fields(value); len(fields) > 0 {
		return fields[0]
	}
	return value
}
