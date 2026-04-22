package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/spf13/cobra"

	"mihoctl/internal/app"
	"mihoctl/internal/mihomo"
)

func newProxyCommand(application *app.App) *cobra.Command {
	proxyCmd := &cobra.Command{
		Use:   "proxy",
		Short: application.T("cmd.proxy.short"),
	}

	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   application.T("cmd.proxy.ls.short"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureProxyCommandReady(cmd, application); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			groups, err := application.MihomoClient().ListProxyGroups(ctx)
			if err != nil {
				return err
			}
			for i, group := range groups {
				renderProxyGroup(cmd, application, i, group)
			}
			return nil
		},
	}

	proxyCmd.AddCommand(
		listCmd,
		&cobra.Command{
			Use:   "use <proxy|index> | use <group|index> <proxy|index>",
			Short: application.T("cmd.proxy.use.short"),
			Args:  cobra.RangeArgs(1, 2),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := ensureProxyCommandReady(cmd, application); err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				groups, err := application.MihomoClient().ListProxyGroups(ctx)
				if err != nil {
					return err
				}
				groupValue := ""
				proxyValue := args[0]
				if len(args) == 2 {
					groupValue = args[0]
					proxyValue = args[1]
				}
				group, proxy, err := resolveProxySelection(groups, groupValue, proxyValue)
				if err != nil {
					return err
				}
				if err := application.MihomoClient().UseProxy(ctx, group, proxy); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.proxy.use.success", map[string]any{
					"group": group,
					"proxy": proxy,
				}))
				return nil
			},
		},
		&cobra.Command{
			Use:   "check",
			Short: application.T("cmd.proxy.check.short"),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := ensureProxyCommandReady(cmd, application); err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				client := application.MihomoClient()
				groups, err := client.ListProxyGroups(ctx)
				if err != nil {
					return err
				}
				totalNodes := 0
				for _, group := range groups {
					totalNodes += len(group.All)
				}
				status := newProxyCheckStatus(cmd.ErrOrStderr(), application, len(groups), totalNodes)
				status.Start()
				defer status.Finish()

				delayResults := measureGroupDelaysFast(ctx, client, groups, application.Config.HealthCheck.URL, application.Config.HealthCheck.TimeoutMS, status)
				for i, group := range groups {
					result := delayResults[group.Name]
					if result == nil || (len(result.Delays) == 0 && result.Failed == len(group.All)) {
						renderProxyCheckFailure(cmd, application, i, group)
						continue
					}
					renderProxyDelayGroup(cmd, application, i, group, orderedDelayNames(group.All, result.Delays), result.Delays)
				}
				return nil
			},
		},
	)

	return proxyCmd
}

func ensureProxyCommandReady(cmd *cobra.Command, application *app.App) error {
	return ensureMihomoRuntimeReady(cmd, application)
}

func resolveProxySelection(groups []mihomo.ProxyGroup, groupValue, proxyValue string) (string, string, error) {
	group, err := selectProxyGroup(groups, groupValue)
	if err != nil {
		return "", "", err
	}
	proxy, err := selectProxyNode(group.All, proxyValue)
	if err != nil {
		return "", "", err
	}
	return group.Name, proxy, nil
}

func selectProxyGroup(groups []mihomo.ProxyGroup, value string) (mihomo.ProxyGroup, error) {
	if strings.TrimSpace(value) == "" {
		return selectDefaultProxyGroup(groups)
	}
	if index, ok := parseProxyIndex(value); ok {
		if index >= 0 && index < len(groups) {
			return groups[index], nil
		}
		return mihomo.ProxyGroup{}, fmt.Errorf("proxy group index out of range: %s", value)
	}
	for _, group := range groups {
		if group.Name == strings.TrimSpace(value) {
			return group, nil
		}
	}
	return mihomo.ProxyGroup{}, fmt.Errorf("proxy group not found: %s", value)
}

func selectDefaultProxyGroup(groups []mihomo.ProxyGroup) (mihomo.ProxyGroup, error) {
	if len(groups) == 0 {
		return mihomo.ProxyGroup{}, fmt.Errorf("proxy group not found")
	}
	for _, group := range groups {
		if strings.EqualFold(strings.TrimSpace(group.Name), "GLOBAL") {
			continue
		}
		return group, nil
	}
	return groups[0], nil
}

func selectProxyNode(nodes []string, value string) (string, error) {
	if index, ok := parseProxyIndex(value); ok {
		if index >= 0 && index < len(nodes) {
			return nodes[index], nil
		}
		return "", fmt.Errorf("proxy node index out of range: %s", value)
	}
	for _, node := range nodes {
		if node == strings.TrimSpace(value) {
			return node, nil
		}
	}
	return "", fmt.Errorf("proxy node not found: %s", value)
}

func parseProxyIndex(value string) (int, bool) {
	index, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || index <= 0 {
		return 0, false
	}
	return index - 1, true
}

func renderProxyGroup(cmd *cobra.Command, application *app.App, index int, group mihomo.ProxyGroup) {
	width := terminalWidth(cmd.OutOrStdout())
	renderProxyBlockHeader(cmd, width, application.Tf("msg.proxy.list.group_title", map[string]any{
		"index": index + 1,
		"name":  group.Name,
	}))
	fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.proxy.list.current", map[string]any{
		"node": selectableNodeLabel(group.All, group.Now, application.T("label.none")),
	}))
	fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.proxy.list.count", map[string]any{
		"count": len(group.All),
	}))
	renderProxyBlockDivider(cmd, width)
	if len(group.All) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", application.T("label.none"))
		renderProxyBlockFooter(cmd, width)
		return
	}
	for _, line := range formatProxyNodes(group.All, width) {
		fmt.Fprintln(cmd.OutOrStdout(), line)
	}
	renderProxyBlockFooter(cmd, width)
}

func fallbackProxyNow(value string, application *app.App) string {
	if strings.TrimSpace(value) == "" {
		return application.T("label.none")
	}
	return value
}

func renderProxyCheckFailure(cmd *cobra.Command, application *app.App, index int, group mihomo.ProxyGroup) {
	width := terminalWidth(cmd.OutOrStdout())
	renderProxyBlockHeader(cmd, width, application.Tf("msg.proxy.check.group_title", map[string]any{
		"index": index + 1,
		"group": group.Name,
	}))
	fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.proxy.check.current", map[string]any{
		"node": selectableNodeLabel(group.All, group.Now, application.T("label.none")),
	}))
	fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.proxy.check.current_delay", map[string]any{
		"delay": application.T("label.none"),
	}))
	renderProxyBlockDivider(cmd, width)
	fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", application.Tf("msg.proxy.check.fail", map[string]any{
		"group": group.Name,
	}))
	renderProxyBlockFooter(cmd, width)
}

func renderProxyDelayGroup(cmd *cobra.Command, application *app.App, index int, group mihomo.ProxyGroup, names []string, delays map[string]int) {
	width := terminalWidth(cmd.OutOrStdout())
	renderProxyBlockHeader(cmd, width, application.Tf("msg.proxy.check.group_title", map[string]any{
		"index": index + 1,
		"group": group.Name,
	}))
	fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.proxy.check.current", map[string]any{
		"node": selectableNodeLabel(group.All, group.Now, application.T("label.none")),
	}))
	fmt.Fprintln(cmd.OutOrStdout(), application.Tf("msg.proxy.check.current_delay", map[string]any{
		"delay": currentDelayLabel(group.Now, delays, application),
	}))
	renderProxyBlockDivider(cmd, width)
	items := make([]delayCell, 0, len(names))
	for _, name := range names {
		items = append(items, delayCell{
			label: selectableNodeLabel(group.All, name, application.T("label.none")),
			right: formatDelayMS(delays[name]),
		})
	}
	for _, line := range formatDelayCells(items, width) {
		fmt.Fprintln(cmd.OutOrStdout(), line)
	}
	renderProxyBlockFooter(cmd, width)
}

func formatProxyNodes(nodes []string, width int) []string {
	items := make([]string, 0, len(nodes))
	for i, node := range nodes {
		items = append(items, fmt.Sprintf("[%d] %s", i+1, node))
	}
	return formatCells(items, width)
}

func formatCells(items []string, width int) []string {
	if len(items) == 0 {
		return nil
	}

	// 按终端宽度动态估算列数，优先保证块状可读性，再尽量多放几列。
	contentWidth := maxInt(width-4, 48)
	longest := 0
	for _, item := range items {
		longest = maxInt(longest, displayWidth(item))
	}
	cellWidth := clampInt(longest+2, 18, 36)
	columns := maxInt(1, contentWidth/cellWidth)
	if columns > len(items) {
		columns = len(items)
	}
	if columns <= 0 {
		columns = 1
	}
	actualCellWidth := maxInt(12, (contentWidth-(columns-1)*2)/columns)

	lines := make([]string, 0, (len(items)+columns-1)/columns)
	row := make([]string, 0, columns)
	for _, item := range items {
		row = append(row, padCell(item, actualCellWidth))
		if len(row) == columns {
			lines = append(lines, "  "+strings.TrimRight(strings.Join(row, " | "), " "))
			row = row[:0]
		}
	}
	if len(row) > 0 {
		lines = append(lines, "  "+strings.TrimRight(strings.Join(row, " | "), " "))
	}
	return lines
}

type delayCell struct {
	label string
	right string
}

func formatDelayCells(items []delayCell, width int) []string {
	if len(items) == 0 {
		return nil
	}

	contentWidth := maxInt(width-4, 48)
	longestLabel := 0
	longestRight := 0
	for _, item := range items {
		longestLabel = maxInt(longestLabel, displayWidth(item.label))
		longestRight = maxInt(longestRight, displayWidth(item.right))
	}

	cellWidth := clampInt(longestLabel+longestRight+3, 22, 44)
	columns := maxInt(1, contentWidth/cellWidth)
	if columns > len(items) {
		columns = len(items)
	}
	actualCellWidth := maxInt(16, (contentWidth-(columns-1)*2)/columns)
	labelWidth := maxInt(8, actualCellWidth-longestRight-1)

	lines := make([]string, 0, (len(items)+columns-1)/columns)
	row := make([]string, 0, columns)
	for _, item := range items {
		cell := padCell(item.label, labelWidth) + " " + leftPadCell(item.right, longestRight)
		row = append(row, padCell(cell, actualCellWidth))
		if len(row) == columns {
			lines = append(lines, "  "+strings.TrimRight(strings.Join(row, " | "), " "))
			row = row[:0]
		}
	}
	if len(row) > 0 {
		lines = append(lines, "  "+strings.TrimRight(strings.Join(row, " | "), " "))
	}
	return lines
}

func renderProxyBlockHeader(cmd *cobra.Command, width int, title string) {
	lineWidth := normalizedBlockWidth(width)
	border := strings.Repeat("=", lineWidth)
	fmt.Fprintln(cmd.OutOrStdout(), border)
	fmt.Fprintln(cmd.OutOrStdout(), title)
}

func renderProxyBlockDivider(cmd *cobra.Command, width int) {
	lineWidth := normalizedBlockWidth(width)
	fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", lineWidth))
}

func renderProxyBlockFooter(cmd *cobra.Command, width int) {
	lineWidth := normalizedBlockWidth(width)
	fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", lineWidth))
}

func terminalWidth(out any) int {
	if width := realTerminalWidth(out); width >= 40 {
		return width
	}
	if value := strings.TrimSpace(os.Getenv("COLUMNS")); value != "" {
		if width, err := strconv.Atoi(value); err == nil && width >= 40 {
			return width
		}
	}
	return 120
}

func padCell(value string, width int) string {
	text := truncateCell(value, width)
	padding := width - displayWidth(text)
	if padding <= 0 {
		return text
	}
	return text + strings.Repeat(" ", padding)
}

func leftPadCell(value string, width int) string {
	padding := width - displayWidth(value)
	if padding <= 0 {
		return value
	}
	return strings.Repeat(" ", padding) + value
}

func truncateCell(value string, width int) string {
	if width <= 0 || displayWidth(value) <= width {
		return value
	}
	const ellipsis = "..."
	if width <= len(ellipsis) {
		return ellipsis[:width]
	}
	limit := width - len(ellipsis)
	current := 0
	var builder strings.Builder
	for _, r := range value {
		w := runeDisplayWidth(r)
		if current+w > limit {
			break
		}
		builder.WriteRune(r)
		current += w
	}
	builder.WriteString(ellipsis)
	return builder.String()
}

func displayWidth(value string) int {
	total := 0
	for _, r := range value {
		total += runeDisplayWidth(r)
	}
	return total
}

func runeDisplayWidth(r rune) int {
	if r == 0 {
		return 0
	}
	if r == '\u200d' || r == '\ufe0f' {
		return 0
	}
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
		return 0
	}
	if r <= 0x7f {
		return 1
	}
	// 区旗区域指示符在很多终端里并不会按双宽 emoji 渲染，
	// 按单宽处理更接近实际显示效果，能避免整列错位。
	if r >= 0x1f1e6 && r <= 0x1f1ff {
		return 1
	}
	if isWideRune(r) {
		return 2
	}
	return 1
}

func isWideRune(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115f:
		return true
	case r >= 0x2329 && r <= 0x232a:
		return true
	case r >= 0x2e80 && r <= 0xa4cf:
		return true
	case r >= 0xac00 && r <= 0xd7a3:
		return true
	case r >= 0xf900 && r <= 0xfaff:
		return true
	case r >= 0xfe10 && r <= 0xfe19:
		return true
	case r >= 0xfe30 && r <= 0xfe6f:
		return true
	case r >= 0xff01 && r <= 0xff60:
		return true
	case r >= 0xffe0 && r <= 0xffe6:
		return true
	case r >= 0x1f300 && r <= 0x1f64f:
		return true
	case r >= 0x1f900 && r <= 0x1f9ff:
		return true
	case r >= 0x20000 && r <= 0x3fffd:
		return true
	default:
		return false
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(value, minValue, maxValue int) int {
	return minInt(maxInt(value, minValue), maxValue)
}

func normalizedBlockWidth(width int) int {
	if width <= 0 {
		return 72
	}
	return maxInt(48, width-2)
}

func proxyNodeIndex(nodes []string, value string) (int, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}
	for i, node := range nodes {
		if node == trimmed {
			return i + 1, true
		}
	}
	return 0, false
}

func selectableNodeLabel(nodes []string, value string, noneLabel string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return noneLabel
	}
	if index, ok := proxyNodeIndex(nodes, trimmed); ok {
		return fmt.Sprintf("[%d] %s", index, trimmed)
	}
	return trimmed
}

func orderedDelayNames(nodes []string, delays map[string]int) []string {
	names := make([]string, 0, len(delays))
	seen := make(map[string]struct{}, len(delays))
	for _, node := range nodes {
		if _, ok := delays[node]; !ok {
			continue
		}
		names = append(names, node)
		seen[node] = struct{}{}
	}

	extras := make([]string, 0)
	for name := range delays {
		if _, ok := seen[name]; ok {
			continue
		}
		extras = append(extras, name)
	}
	sort.Strings(extras)
	return append(names, extras...)
}

func currentDelayLabel(current string, delays map[string]int, application *app.App) string {
	delay, ok := delays[strings.TrimSpace(current)]
	if !ok {
		return application.T("label.none")
	}
	return formatDelayMS(delay)
}

func formatDelayMS(delay int) string {
	if delay <= 0 {
		return "-"
	}
	return fmt.Sprintf("%4d ms", delay)
}

type groupDelayResult struct {
	Delays map[string]int
	Failed int
}

func measureGroupDelaysFast(ctx context.Context, client *mihomo.Client, groups []mihomo.ProxyGroup, testURL string, timeoutMS int, status *proxyCheckStatus) map[string]*groupDelayResult {
	results := make(map[string]*groupDelayResult, len(groups))
	if len(groups) == 0 {
		return results
	}

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	for _, group := range groups {
		group := group
		wg.Add(1)
		go func() {
			defer wg.Done()

			delays, err := client.CheckGroupDelay(ctx, group.Name, testURL, timeoutMS)
			result := &groupDelayResult{
				Delays: map[string]int{},
				Failed: len(group.All),
			}
			if err == nil {
				failed := 0
				for _, node := range group.All {
					delay, ok := delays[node]
					if !ok || delay <= 0 {
						failed++
						continue
					}
					result.Delays[node] = delay
				}
				result.Failed = failed
			}

			mu.Lock()
			results[group.Name] = result
			mu.Unlock()
			if status != nil {
				status.Advance(group.Name, len(group.All))
			}
		}()
	}

	wg.Wait()
	return results
}

type proxyCheckStatus struct {
	out         io.Writer
	application *app.App
	totalGroups int
	startedAt   time.Time
	done        chan struct{}
	once        sync.Once
	doneGroups  atomic.Int64
	lastWidth   atomic.Int64
}

func newProxyCheckStatus(out io.Writer, application *app.App, totalGroups, totalNodes int) *proxyCheckStatus {
	if out == nil {
		out = io.Discard
	}
	return &proxyCheckStatus{
		out:         out,
		application: application,
		totalGroups: maxInt(totalGroups, 1),
		done:        make(chan struct{}),
	}
}

func (s *proxyCheckStatus) Start() {
	s.startedAt = time.Now()
	go s.loop()
}

func (s *proxyCheckStatus) Advance(group string, nodeCount int) {
	s.doneGroups.Add(1)
	s.render()
}

func (s *proxyCheckStatus) Finish() {
	s.once.Do(func() {
		close(s.done)
		s.clear()
	})
}

func (s *proxyCheckStatus) loop() {
	s.render()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.render()
		case <-s.done:
			return
		}
	}
}

func (s *proxyCheckStatus) render() {
	spinnerFrames := []string{"-", "\\", "|", "/"}
	frame := spinnerFrames[int(time.Since(s.startedAt)/(150*time.Millisecond))%len(spinnerFrames)]

	line := s.application.Tf("msg.proxy.check.start", map[string]any{
		"spinner":      frame,
		"groups_total": s.totalGroups,
		"elapsed":      time.Since(s.startedAt).Truncate(time.Second),
	})

	width := normalizedBlockWidth(terminalWidth(s.out))
	if displayWidth(line) > width {
		line = truncateCell(line, width)
	}
	line = padCell(line, width)
	lastWidth := int(s.lastWidth.Load())
	if width > lastWidth {
		s.lastWidth.Store(int64(width))
		lastWidth = width
	}
	fmt.Fprintf(s.out, "\r%s%s", line, strings.Repeat(" ", maxInt(0, lastWidth-displayWidth(line))))
}

func (s *proxyCheckStatus) clear() {
	lastWidth := int(s.lastWidth.Load())
	if lastWidth <= 0 {
		return
	}
	fmt.Fprintf(s.out, "\r%s\r", strings.Repeat(" ", lastWidth))
}
