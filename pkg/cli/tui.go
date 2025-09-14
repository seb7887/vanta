package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.uber.org/zap"
	"github.com/seb7887/vanta/pkg/api"
	"github.com/seb7887/vanta/pkg/config"
)

const (
	TabMetrics = iota
	TabLogs
	TabConfig
	MaxTabs = 3
)

type TUIModel struct {
	config       *config.Config
	server       *api.Server
	logger       *zap.Logger

	// UI state
	activeTab    int
	width        int
	height       int
	quit         bool

	// Metrics data
	metrics      *MetricsData
	metricsLock  sync.RWMutex

	// Logs data
	logs         []LogEntry
	logsLock     sync.RWMutex
	logFilter    LogFilter
	scrollOffset int
	maxLogs      int

	// Config editor state
	configEditor *ConfigEditor

	// Update context
	ctx    context.Context
	cancel context.CancelFunc
}

type MetricsData struct {
	RPS             float64                `json:"rps"`
	ErrorRate       float64                `json:"error_rate"`
	ActiveConns     int64                  `json:"active_conns"`
	MemoryUsage     float64                `json:"memory_mb"`
	Uptime          time.Duration          `json:"uptime"`
	ChaosActive     bool                   `json:"chaos_active"`
	Latency         LatencyStats           `json:"latency"`
	RequestHistory  []float64              `json:"request_history"`
	TopEndpoints    []EndpointStats        `json:"top_endpoints"`
	LastUpdated     time.Time              `json:"last_updated"`
}

type LatencyStats struct {
	P50 time.Duration `json:"p50"`
	P90 time.Duration `json:"p90"`
	P95 time.Duration `json:"p95"`
	P99 time.Duration `json:"p99"`
}

type EndpointStats struct {
	Method    string  `json:"method"`
	Path      string  `json:"path"`
	RPS       float64 `json:"rps"`
	ErrorRate float64 `json:"error_rate"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Component string    `json:"component"`
	Message   string    `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

type LogFilter struct {
	Level     string // ALL, ERROR, WARN, INFO, DEBUG
	Component string // ALL or specific component
}

type ConfigEditor struct {
	fields       []ConfigField
	activeField  int
	editing      bool
	modified     bool
	originalConfig *config.Config
}

type ConfigField struct {
	Name        string
	Value       string
	Valid       bool
	Type        string // string, int, bool, duration
	Path        string // dot notation path to field
	Description string
}

// Styles for the TUI
var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Background(lipgloss.Color("57")).
		Padding(0, 1)

	tabStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	activeTabStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Background(lipgloss.Color("57")).
		Padding(0, 1)

	contentStyle = lipgloss.NewStyle().
		Padding(1, 2)

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("46"))

	warningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("226"))

	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	debugStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	boxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)
)

// NewTUIModel creates a new TUI model
func NewTUIModel(cfg *config.Config, server *api.Server, logger *zap.Logger) *TUIModel {
	ctx, cancel := context.WithCancel(context.Background())

	model := &TUIModel{
		config:       cfg,
		server:       server,
		logger:       logger,
		activeTab:    TabMetrics,
		maxLogs:      1000,
		metrics:      &MetricsData{},
		logs:         make([]LogEntry, 0, 1000),
		logFilter:    LogFilter{Level: "ALL", Component: "ALL"},
		ctx:          ctx,
		cancel:       cancel,
		configEditor: newConfigEditor(cfg),
	}

	return model
}

func newConfigEditor(cfg *config.Config) *ConfigEditor {
	editor := &ConfigEditor{
		originalConfig: cfg,
		fields:        make([]ConfigField, 0),
	}

	// Add editable fields
	editor.fields = []ConfigField{
		{Name: "Server Port", Value: strconv.Itoa(cfg.Server.Port), Valid: true, Type: "int", Path: "server.port", Description: "HTTP server port"},
		{Name: "Server Host", Value: cfg.Server.Host, Valid: true, Type: "string", Path: "server.host", Description: "HTTP server host"},
		{Name: "Read Timeout", Value: cfg.Server.ReadTimeout.String(), Valid: true, Type: "duration", Path: "server.read_timeout", Description: "Server read timeout"},
		{Name: "Write Timeout", Value: cfg.Server.WriteTimeout.String(), Valid: true, Type: "duration", Path: "server.write_timeout", Description: "Server write timeout"},
		{Name: "Chaos Enabled", Value: strconv.FormatBool(cfg.Chaos.Enabled), Valid: true, Type: "bool", Path: "chaos.enabled", Description: "Enable chaos testing"},
		{Name: "Recording Enabled", Value: strconv.FormatBool(cfg.Recording.Enabled), Valid: true, Type: "bool", Path: "recording.enabled", Description: "Enable request recording"},
		{Name: "Metrics Enabled", Value: strconv.FormatBool(cfg.Metrics.Enabled), Valid: true, Type: "bool", Path: "metrics.enabled", Description: "Enable metrics collection"},
	}

	return editor
}

// Init implements tea.Model
func (m *TUIModel) Init() tea.Cmd {
	return tea.Batch(
		m.startMetricsUpdater(),
		m.startLogCollector(),
		tea.EnterAltScreen,
	)
}

func (m *TUIModel) startMetricsUpdater() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return MetricsUpdateMsg{time: t}
	})
}

func (m *TUIModel) startLogCollector() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return LogUpdateMsg{time: t}
	})
}

// Messages
type MetricsUpdateMsg struct{ time time.Time }
type LogUpdateMsg struct{ time time.Time }
type ConfigSaveMsg struct{}

// Update implements tea.Model
func (m *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case MetricsUpdateMsg:
		m.updateMetrics()
		return m, m.startMetricsUpdater()
	case LogUpdateMsg:
		m.updateLogs()
		return m, m.startLogCollector()
	case ConfigSaveMsg:
		return m.saveConfig()
	}
	return m, nil
}

func (m *TUIModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.quit {
		return m, tea.Quit
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.quit = true
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	case "tab":
		m.activeTab = (m.activeTab + 1) % MaxTabs
		m.scrollOffset = 0 // Reset scroll when switching tabs
		return m, nil
	case "shift+tab":
		m.activeTab = (m.activeTab - 1 + MaxTabs) % MaxTabs
		m.scrollOffset = 0
		return m, nil
	case "r":
		// Manual refresh
		m.updateMetrics()
		m.updateLogs()
		return m, nil
	}

	// Tab-specific key handling
	switch m.activeTab {
	case TabLogs:
		return m.handleLogsKeyMsg(msg)
	case TabConfig:
		return m.handleConfigKeyMsg(msg)
	}

	return m, nil
}

func (m *TUIModel) handleLogsKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	case "down", "j":
		maxScroll := len(m.logs) - (m.height - 10) // Account for UI chrome
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.scrollOffset < maxScroll {
			m.scrollOffset++
		}
	case "f":
		// Toggle filter
		switch m.logFilter.Level {
		case "ALL":
			m.logFilter.Level = "ERROR"
		case "ERROR":
			m.logFilter.Level = "WARN"
		case "WARN":
			m.logFilter.Level = "INFO"
		case "INFO":
			m.logFilter.Level = "DEBUG"
		case "DEBUG":
			m.logFilter.Level = "ALL"
		}
		m.scrollOffset = 0
	case "c":
		// Clear logs
		m.logsLock.Lock()
		m.logs = make([]LogEntry, 0, m.maxLogs)
		m.logsLock.Unlock()
		m.scrollOffset = 0
	}
	return m, nil
}

func (m *TUIModel) handleConfigKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	editor := m.configEditor

	switch msg.String() {
	case "up", "k":
		if editor.activeField > 0 {
			editor.activeField--
		}
		editor.editing = false
	case "down", "j":
		if editor.activeField < len(editor.fields)-1 {
			editor.activeField++
		}
		editor.editing = false
	case "enter":
		if !editor.editing {
			editor.editing = true
		} else {
			editor.editing = false
			m.validateField(editor.activeField)
		}
	case "esc":
		editor.editing = false
	case "ctrl+s":
		if editor.modified {
			return m, func() tea.Msg { return ConfigSaveMsg{} }
		}
	case "ctrl+r":
		// Reset to original values
		m.configEditor = newConfigEditor(m.config)
	default:
		if editor.editing && len(msg.String()) == 1 {
			// Simple character input for editing
			field := &editor.fields[editor.activeField]
			if msg.String() == " " {
				field.Value += " "
			} else if msg.Type == tea.KeyBackspace {
				if len(field.Value) > 0 {
					field.Value = field.Value[:len(field.Value)-1]
				}
			} else {
				field.Value += msg.String()
			}
			editor.modified = true
			m.validateField(editor.activeField)
		}
	}

	return m, nil
}

func (m *TUIModel) validateField(fieldIndex int) {
	if fieldIndex < 0 || fieldIndex >= len(m.configEditor.fields) {
		return
	}

	field := &m.configEditor.fields[fieldIndex]

	switch field.Type {
	case "int":
		if _, err := strconv.Atoi(field.Value); err != nil {
			field.Valid = false
		} else {
			field.Valid = true
		}
	case "bool":
		if _, err := strconv.ParseBool(field.Value); err != nil {
			field.Valid = false
		} else {
			field.Valid = true
		}
	case "duration":
		if _, err := time.ParseDuration(field.Value); err != nil {
			field.Valid = false
		} else {
			field.Valid = true
		}
	default:
		field.Valid = len(strings.TrimSpace(field.Value)) > 0
	}
}

func (m *TUIModel) saveConfig() (tea.Model, tea.Cmd) {
	// For now, just mark as saved
	// In a full implementation, this would update the actual config and trigger hot reload
	m.configEditor.modified = false
	m.logger.Info("Configuration changes saved (simulated)")
	return m, nil
}

func (m *TUIModel) updateMetrics() {
	if m.server == nil {
		return
	}

	stats := m.server.GetStats()

	m.metricsLock.Lock()
	defer m.metricsLock.Unlock()

	// Update metrics data
	if stats.Metrics != nil {
		if rps, ok := stats.Metrics["rps"].(float64); ok {
			m.metrics.RPS = rps
		}
		if errorRate, ok := stats.Metrics["error_rate"].(float64); ok {
			m.metrics.ErrorRate = errorRate
		}
		if activeConns, ok := stats.Metrics["active_connections"].(int64); ok {
			m.metrics.ActiveConns = activeConns
		}
	}

	// Calculate uptime
	if stats.IsRunning {
		m.metrics.Uptime = time.Since(stats.StartTime)
	}

	// Update request history (sliding window)
	m.metrics.RequestHistory = append(m.metrics.RequestHistory, m.metrics.RPS)
	if len(m.metrics.RequestHistory) > 60 {
		m.metrics.RequestHistory = m.metrics.RequestHistory[1:]
	}

	m.metrics.LastUpdated = time.Now()
}

func (m *TUIModel) updateLogs() {
	// In a real implementation, this would collect logs from the logger
	// For now, we'll simulate some log entries
	if len(m.logs) < 10 {
		m.addSampleLogs()
	}
}

func (m *TUIModel) addSampleLogs() {
	sampleLogs := []LogEntry{
		{time.Now(), "INFO", "server", "HTTP server started on :8080", nil},
		{time.Now().Add(-1 * time.Second), "DEBUG", "chaos", "Loaded 3 chaos scenarios", nil},
		{time.Now().Add(-2 * time.Second), "WARN", "recorder", "Storage 80% full", nil},
		{time.Now().Add(-3 * time.Second), "ERROR", "auth", "Invalid JWT token", nil},
		{time.Now().Add(-4 * time.Second), "INFO", "middleware", "Request processed in 15ms", nil},
	}

	m.logsLock.Lock()
	defer m.logsLock.Unlock()

	for _, log := range sampleLogs {
		m.logs = append(m.logs, log)
		if len(m.logs) > m.maxLogs {
			m.logs = m.logs[1:]
		}
	}
}

// View implements tea.Model
func (m *TUIModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Build header with tabs
	header := m.renderHeader()

	// Build content based on active tab
	var content string
	switch m.activeTab {
	case TabMetrics:
		content = m.renderMetricsTab()
	case TabLogs:
		content = m.renderLogsTab()
	case TabConfig:
		content = m.renderConfigTab()
	}

	// Build footer
	footer := m.renderFooter()

	return header + "\n" + content + "\n" + footer
}

func (m *TUIModel) renderHeader() string {
	tabs := []string{}

	for i, name := range []string{"📊 METRICS", "📝 LOGS", "⚙️  CONFIG"} {
		style := tabStyle
		if i == m.activeTab {
			style = activeTabStyle
		}
		tabs = append(tabs, style.Render(fmt.Sprintf(" %s [%d/%d] ", name, i+1, MaxTabs)))
	}

	title := titleStyle.Render(" OpenAPI Mocker TUI ")
	tabsStr := strings.Join(tabs, " ")

	// Add status indicator
	status := "⚫ DISCONNECTED"
	if m.server != nil && m.server.IsRunning() {
		status = "🟢 RUNNING"
	}

	headerWidth := m.width
	padding := headerWidth - lipgloss.Width(title) - lipgloss.Width(tabsStr) - lipgloss.Width(status) - 2
	if padding < 0 {
		padding = 0
	}

	return title + tabsStr + strings.Repeat(" ", padding) + status
}

func (m *TUIModel) renderMetricsTab() string {
	m.metricsLock.RLock()
	metrics := *m.metrics
	m.metricsLock.RUnlock()

	// Top row with key metrics
	rpsBox := boxStyle.Width(18).Render(fmt.Sprintf("RPS: %s\nErrors: %.1f%%\nActive: %d conn",
		m.formatNumber(metrics.RPS),
		metrics.ErrorRate,
		metrics.ActiveConns))

	latencyBox := boxStyle.Width(18).Render(fmt.Sprintf("P50: %s\nP90: %s\nP99: %s",
		metrics.Latency.P50.Truncate(time.Millisecond),
		metrics.Latency.P90.Truncate(time.Millisecond),
		metrics.Latency.P99.Truncate(time.Millisecond)))

	systemBox := boxStyle.Width(18).Render(fmt.Sprintf("Memory: %.1fMB\nUptime: %s\nChaos: %s",
		metrics.MemoryUsage,
		m.formatDuration(metrics.Uptime),
		m.formatBool(metrics.ChaosActive, "✅ ACTIVE", "⚫ INACTIVE")))

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, rpsBox, "  ", latencyBox, "  ", systemBox)

	// Request history chart
	historyChart := m.renderRequestHistory(metrics.RequestHistory)
	historyBox := boxStyle.Width(m.width-4).Render("Request History (last 60s):\n" + historyChart)

	// Top endpoints table
	endpointsTable := m.renderTopEndpoints(metrics.TopEndpoints)
	endpointsBox := boxStyle.Width(m.width-4).Render("Top Endpoints:\n" + endpointsTable)

	return contentStyle.Render(
		topRow + "\n\n" +
		historyBox + "\n\n" +
		endpointsBox,
	)
}

func (m *TUIModel) renderLogsTab() string {
	m.logsLock.RLock()
	logs := make([]LogEntry, len(m.logs))
	copy(logs, m.logs)
	m.logsLock.RUnlock()

	// Filter logs
	filteredLogs := []LogEntry{}
	for _, log := range logs {
		if m.logFilter.Level != "ALL" && log.Level != m.logFilter.Level {
			continue
		}
		if m.logFilter.Component != "ALL" && log.Component != m.logFilter.Component {
			continue
		}
		filteredLogs = append(filteredLogs, log)
	}

	// Sort by timestamp (newest first)
	sort.Slice(filteredLogs, func(i, j int) bool {
		return filteredLogs[i].Timestamp.After(filteredLogs[j].Timestamp)
	})

	// Calculate visible range
	maxVisible := m.height - 10 // Account for UI chrome
	startIdx := m.scrollOffset
	endIdx := startIdx + maxVisible

	if endIdx > len(filteredLogs) {
		endIdx = len(filteredLogs)
	}
	if startIdx > len(filteredLogs) {
		startIdx = len(filteredLogs)
	}

	// Render visible logs
	logLines := []string{}
	for i := startIdx; i < endIdx; i++ {
		log := filteredLogs[i]
		timestamp := log.Timestamp.Format("15:04:05")

		var levelStyle lipgloss.Style
		switch log.Level {
		case "ERROR":
			levelStyle = errorStyle
		case "WARN":
			levelStyle = warningStyle
		case "INFO":
			levelStyle = infoStyle
		case "DEBUG":
			levelStyle = debugStyle
		default:
			levelStyle = lipgloss.NewStyle()
		}

		level := levelStyle.Render(fmt.Sprintf("[%-5s]", log.Level))
		component := fmt.Sprintf("%-10s", log.Component)

		line := fmt.Sprintf("%s %s %s │ %s", timestamp, level, component, log.Message)
		if len(line) > m.width-6 {
			line = line[:m.width-9] + "..."
		}
		logLines = append(logLines, line)
	}

	if len(logLines) == 0 {
		logLines = append(logLines, "No logs to display")
	}

	// Header with filter info
	filterInfo := fmt.Sprintf("Filter: %s", m.logFilter.Level)
	scrollInfo := fmt.Sprintf("Showing %d-%d of %d", startIdx+1, endIdx, len(filteredLogs))
	header := fmt.Sprintf("%s%s%s",
		filterInfo,
		strings.Repeat(" ", m.width-len(filterInfo)-len(scrollInfo)-8),
		scrollInfo)

	content := strings.Join(logLines, "\n")
	controls := "Controls: ↑/↓ scroll │ f:filter │ c:clear │ q:quit"

	return contentStyle.Render(
		boxStyle.Width(m.width-4).Render(header + "\n" + content + "\n" + controls),
	)
}

func (m *TUIModel) renderConfigTab() string {
	editor := m.configEditor

	lines := []string{}
	lines = append(lines, "Server Configuration:")

	for i, field := range editor.fields {
		prefix := "  "
		if i == editor.activeField {
			if editor.editing {
				prefix = "> "
			} else {
				prefix = "► "
			}
		}

		status := "✅"
		if !field.Valid {
			status = "❌"
		}

		value := field.Value
		if i == editor.activeField && editor.editing {
			value = value + "█" // Cursor
		}

		line := fmt.Sprintf("%s%-15s [%-15s] %s", prefix, field.Name+":", value, status)
		lines = append(lines, line)
	}

	lines = append(lines, "")

	// Action buttons
	actions := []string{}
	if editor.modified {
		actions = append(actions, "[Ctrl+S] Apply", "[Ctrl+R] Reset", "[Esc] Cancel")
		lines = append(lines, successStyle.Render(fmt.Sprintf("Modified: %d fields", m.countModifiedFields())))
	} else {
		actions = append(actions, "[Enter] Edit", "[↑/↓] Navigate")
	}

	lines = append(lines, strings.Join(actions, " "))

	content := strings.Join(lines, "\n")
	return contentStyle.Render(boxStyle.Width(m.width-4).Render(content))
}

func (m *TUIModel) renderFooter() string {
	left := "Tab: Switch tabs │ R: Refresh │ Q: Quit"
	right := fmt.Sprintf("Terminal: %dx%d", m.width, m.height)

	padding := m.width - len(left) - len(right)
	if padding < 0 {
		padding = 0
	}

	return left + strings.Repeat(" ", padding) + right
}

func (m *TUIModel) renderRequestHistory(history []float64) string {
	if len(history) == 0 {
		return "No data"
	}

	// Simple ASCII chart
	max := 0.0
	for _, v := range history {
		if v > max {
			max = v
		}
	}

	if max == 0 {
		return strings.Repeat("▁", min(len(history), 60))
	}

	chars := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	result := ""

	for i, v := range history {
		if i >= 60 {
			break
		}
		level := int((v / max) * float64(len(chars)-1))
		if level >= len(chars) {
			level = len(chars) - 1
		}
		result += chars[level]
	}

	return result
}

func (m *TUIModel) renderTopEndpoints(endpoints []EndpointStats) string {
	if len(endpoints) == 0 {
		return "No endpoint data available"
	}

	lines := []string{}
	for i, ep := range endpoints {
		if i >= 5 { // Show top 5
			break
		}

		line := fmt.Sprintf("%-4s %-20s │ %s req/s │ %.1f%% errors",
			ep.Method,
			m.truncateString(ep.Path, 20),
			m.formatNumber(ep.RPS),
			ep.ErrorRate)
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return "No endpoints available"
	}

	return strings.Join(lines, "\n")
}

// Helper functions
func (m *TUIModel) formatNumber(n float64) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", n/1000)
	}
	return fmt.Sprintf("%.0f", n)
}

func (m *TUIModel) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func (m *TUIModel) formatBool(b bool, trueVal, falseVal string) string {
	if b {
		return trueVal
	}
	return falseVal
}

func (m *TUIModel) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (m *TUIModel) countModifiedFields() int {
	// In a real implementation, compare with original values
	return 3 // Placeholder
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RunTUI starts the TUI application
func RunTUI(cfg *config.Config, server *api.Server, logger *zap.Logger) error {
	model := NewTUIModel(cfg, server, logger)

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}