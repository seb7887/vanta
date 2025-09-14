package cli

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
	"go.uber.org/zap/zaptest"
	"github.com/seb7887/vanta/pkg/api"
	"github.com/seb7887/vanta/pkg/config"
	"github.com/seb7887/vanta/pkg/openapi"
)

func TestNewTUIModel(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:         8080,
			Host:         "localhost",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Chaos:     config.ChaosConfig{},
		Recording: config.RecordingConfig{},
		Metrics:   config.MetricsConfig{},
	}

	spec := &openapi.Specification{
		Version: "3.0.0",
		Info: openapi.InfoObject{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: make(map[string]openapi.PathItem),
	}

	server, err := api.NewServer(cfg, spec, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	model := NewTUIModel(cfg, server, logger)

	if model == nil {
		t.Fatal("Expected model to be created")
	}

	if model.activeTab != TabMetrics {
		t.Errorf("Expected initial tab to be TabMetrics (%d), got %d", TabMetrics, model.activeTab)
	}

	if model.maxLogs != 1000 {
		t.Errorf("Expected maxLogs to be 1000, got %d", model.maxLogs)
	}

	if model.logFilter.Level != "ALL" {
		t.Errorf("Expected log filter level to be 'ALL', got '%s'", model.logFilter.Level)
	}

	if len(model.configEditor.fields) == 0 {
		t.Error("Expected config editor to have fields")
	}
}

func TestTUIModelTabSwitching(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{}

	model := &TUIModel{
		config:       cfg,
		logger:       logger,
		activeTab:    TabMetrics,
		configEditor: newConfigEditor(cfg),
	}

	// Test tab switching forward
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	tuiModel := newModel.(*TUIModel)

	if tuiModel.activeTab != TabLogs {
		t.Errorf("Expected tab to be TabLogs (%d), got %d", TabLogs, tuiModel.activeTab)
	}

	// Test tab switching backward from first tab
	model.activeTab = TabMetrics
	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	tuiModel = newModel.(*TUIModel)

	if tuiModel.activeTab != TabConfig {
		t.Errorf("Expected tab to be TabConfig (%d), got %d", TabConfig, tuiModel.activeTab)
	}
}

func TestTUIModelQuit(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{}

	model := &TUIModel{
		config:       cfg,
		logger:       logger,
		quit:         false,
		configEditor: newConfigEditor(cfg),
	}

	// Test quit with 'q'
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tuiModel := newModel.(*TUIModel)

	if !tuiModel.quit {
		t.Error("Expected quit to be true after pressing 'q'")
	}

	if cmd == nil {
		t.Error("Expected quit command to be returned")
	}
}

func TestMetricsData(t *testing.T) {
	metrics := &MetricsData{
		RPS:         150.5,
		ErrorRate:   2.3,
		ActiveConns: 45,
		MemoryUsage: 67.8,
	}

	if metrics.RPS != 150.5 {
		t.Errorf("Expected RPS to be 150.5, got %f", metrics.RPS)
	}

	if metrics.ErrorRate != 2.3 {
		t.Errorf("Expected ErrorRate to be 2.3, got %f", metrics.ErrorRate)
	}
}

func TestLogEntry(t *testing.T) {
	now := time.Now()
	entry := LogEntry{
		Timestamp: now,
		Level:     "INFO",
		Component: "server",
		Message:   "Test message",
		Fields:    map[string]interface{}{"key": "value"},
	}

	if entry.Timestamp != now {
		t.Error("Expected timestamp to match")
	}

	if entry.Level != "INFO" {
		t.Errorf("Expected level to be 'INFO', got '%s'", entry.Level)
	}

	if entry.Component != "server" {
		t.Errorf("Expected component to be 'server', got '%s'", entry.Component)
	}

	if entry.Fields["key"] != "value" {
		t.Error("Expected fields to contain key-value pair")
	}
}

func TestConfigEditor(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:         8080,
			Host:         "localhost",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Chaos:     config.ChaosConfig{Enabled: false},
		Recording: config.RecordingConfig{Enabled: false},
		Metrics:   config.MetricsConfig{Enabled: true},
	}

	editor := newConfigEditor(cfg)

	if len(editor.fields) == 0 {
		t.Error("Expected editor to have fields")
	}

	if editor.activeField != 0 {
		t.Errorf("Expected active field to be 0, got %d", editor.activeField)
	}

	if editor.editing {
		t.Error("Expected editor to not be in editing mode initially")
	}

	if editor.modified {
		t.Error("Expected editor to not be modified initially")
	}

	// Check that server port field exists and has correct value
	found := false
	for _, field := range editor.fields {
		if field.Name == "Server Port" {
			if field.Value != "8080" {
				t.Errorf("Expected server port field value to be '8080', got '%s'", field.Value)
			}
			if field.Type != "int" {
				t.Errorf("Expected server port field type to be 'int', got '%s'", field.Type)
			}
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find 'Server Port' field")
	}
}

func TestConfigFieldValidation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{}

	model := &TUIModel{
		logger:       logger,
		configEditor: newConfigEditor(cfg),
	}

	// Test int validation
	model.configEditor.fields = []ConfigField{
		{Name: "Test Int", Value: "123", Type: "int"},
		{Name: "Invalid Int", Value: "abc", Type: "int"},
	}

	model.validateField(0)
	model.validateField(1)

	if !model.configEditor.fields[0].Valid {
		t.Error("Expected valid int field to be valid")
	}

	if model.configEditor.fields[1].Valid {
		t.Error("Expected invalid int field to be invalid")
	}

	// Test bool validation
	model.configEditor.fields = []ConfigField{
		{Name: "Test Bool", Value: "true", Type: "bool"},
		{Name: "Invalid Bool", Value: "maybe", Type: "bool"},
	}

	model.validateField(0)
	model.validateField(1)

	if !model.configEditor.fields[0].Valid {
		t.Error("Expected valid bool field to be valid")
	}

	if model.configEditor.fields[1].Valid {
		t.Error("Expected invalid bool field to be invalid")
	}

	// Test duration validation
	model.configEditor.fields = []ConfigField{
		{Name: "Test Duration", Value: "30s", Type: "duration"},
		{Name: "Invalid Duration", Value: "30", Type: "duration"},
	}

	model.validateField(0)
	model.validateField(1)

	if !model.configEditor.fields[0].Valid {
		t.Error("Expected valid duration field to be valid")
	}

	if model.configEditor.fields[1].Valid {
		t.Error("Expected invalid duration field to be invalid")
	}
}

func TestLogFiltering(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{}

	model := &TUIModel{
		logger:       logger,
		configEditor: newConfigEditor(cfg),
		logs: []LogEntry{
			{Level: "ERROR", Component: "server", Message: "Error message"},
			{Level: "WARN", Component: "auth", Message: "Warning message"},
			{Level: "INFO", Component: "server", Message: "Info message"},
			{Level: "DEBUG", Component: "chaos", Message: "Debug message"},
		},
		logFilter: LogFilter{Level: "ERROR", Component: "ALL"},
	}

	// In a real implementation, you would test the filtering logic
	// For now, we just verify the log structure is correct
	if len(model.logs) != 4 {
		t.Errorf("Expected 4 log entries, got %d", len(model.logs))
	}

	if model.logFilter.Level != "ERROR" {
		t.Errorf("Expected log filter level to be 'ERROR', got '%s'", model.logFilter.Level)
	}
}

func TestHelperFunctions(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{}

	model := &TUIModel{
		logger:       logger,
		configEditor: newConfigEditor(cfg),
	}

	// Test formatNumber
	tests := []struct {
		input    float64
		expected string
	}{
		{123, "123"},
		{1234, "1.2k"},
		{999, "999"},
		{1000, "1.0k"},
	}

	for _, test := range tests {
		result := model.formatNumber(test.input)
		if result != test.expected {
			t.Errorf("formatNumber(%f): expected '%s', got '%s'", test.input, test.expected, result)
		}
	}

	// Test formatDuration
	durTests := []struct {
		input    time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{2 * time.Minute, "2m"},
		{90 * time.Second, "1m"},
		{2*time.Hour + 15*time.Minute, "2h 15m"},
	}

	for _, test := range durTests {
		result := model.formatDuration(test.input)
		if result != test.expected {
			t.Errorf("formatDuration(%v): expected '%s', got '%s'", test.input, test.expected, result)
		}
	}

	// Test formatBool
	if result := model.formatBool(true, "YES", "NO"); result != "YES" {
		t.Errorf("formatBool(true): expected 'YES', got '%s'", result)
	}

	if result := model.formatBool(false, "YES", "NO"); result != "NO" {
		t.Errorf("formatBool(false): expected 'NO', got '%s'", result)
	}

	// Test truncateString
	if result := model.truncateString("hello world", 20); result != "hello world" {
		t.Errorf("truncateString should not truncate short strings, got '%s'", result)
	}

	if result := model.truncateString("this is a very long string", 10); result != "this is..." {
		t.Errorf("truncateString should truncate long strings, got '%s'", result)
	}
}