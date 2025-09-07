package hotreload

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"vanta/pkg/config"
	"vanta/pkg/openapi"
)

// ServerInterface defines the interface that a server must implement for hot reload
type ServerInterface interface {
	Stop() error
	Start() error
	Restart(newConfig *config.Config, newSpec *openapi.Specification) error
}

// ReloadResult represents the result of a reload operation
type ReloadResult struct {
	Success     bool
	Duration    time.Duration
	Error       error
	Timestamp   time.Time
	ReloadType  string // "config", "spec", "full"
}

// HotReloader manages hot reloading of configuration and OpenAPI specification
type HotReloader struct {
	server       ServerInterface
	configPath   string
	specPath     string
	watcher      *FileWatcher
	logger       *zap.Logger
	reloadChan   chan FileEvent
	ctx          context.Context
	cancel       context.CancelFunc
	reloading    atomic.Bool
	
	// Current state
	currentConfig *config.Config
	currentSpec   *openapi.Specification
	
	// Metrics
	totalReloads    atomic.Int64
	successReloads  atomic.Int64
	failedReloads   atomic.Int64
	lastReloadTime  atomic.Value // time.Time
	
	// Configuration
	config *config.HotReloadConfig
}

// NewHotReloader creates a new hot reloader instance
func NewHotReloader(
	server ServerInterface,
	configPath, specPath string,
	currentConfig *config.Config,
	currentSpec *openapi.Specification,
	logger *zap.Logger,
) (*HotReloader, error) {
	if server == nil {
		return nil, fmt.Errorf("server cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if currentConfig == nil {
		return nil, fmt.Errorf("current config cannot be nil")
	}
	if currentSpec == nil {
		return nil, fmt.Errorf("current spec cannot be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize file watcher with debounce delay
	debounceDelay := currentConfig.HotReload.DebounceDelay
	if debounceDelay == 0 {
		debounceDelay = 500 * time.Millisecond // Default debounce delay
	}

	watcher, err := NewFileWatcher(logger.With(zap.String("component", "file_watcher")), debounceDelay)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	hr := &HotReloader{
		server:        server,
		configPath:    configPath,
		specPath:      specPath,
		watcher:       watcher,
		logger:        logger.With(zap.String("component", "hot_reloader")),
		reloadChan:    make(chan FileEvent, 10), // Buffered channel
		ctx:           ctx,
		cancel:        cancel,
		currentConfig: currentConfig,
		currentSpec:   currentSpec,
		config:        &currentConfig.HotReload,
	}

	hr.lastReloadTime.Store(time.Now())

	return hr, nil
}

// Start starts the hot reload system
func (hr *HotReloader) Start() error {
	if !hr.config.Enabled {
		hr.logger.Info("Hot reload is disabled")
		return nil
	}

	hr.logger.Info("Starting hot reload system",
		zap.String("config_path", hr.configPath),
		zap.String("spec_path", hr.specPath),
		zap.Bool("watch_config", hr.config.WatchConfig),
		zap.Bool("watch_spec", hr.config.WatchSpec),
		zap.Duration("debounce_delay", hr.config.DebounceDelay),
	)

	// Add paths to watcher
	if hr.config.WatchConfig && hr.configPath != "" {
		if err := hr.watcher.AddPath(hr.configPath); err != nil {
			return fmt.Errorf("failed to watch config file: %w", err)
		}
	}

	if hr.config.WatchSpec && hr.specPath != "" {
		if err := hr.watcher.AddPath(hr.specPath); err != nil {
			return fmt.Errorf("failed to watch spec file: %w", err)
		}
	}

	// Start file watcher
	if err := hr.watcher.Start(hr.onFileEvent); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Start reload processing goroutine
	go hr.processReloads()

	hr.logger.Info("Hot reload system started successfully")
	return nil
}

// Stop stops the hot reload system
func (hr *HotReloader) Stop() error {
	hr.logger.Info("Stopping hot reload system...")
	
	hr.cancel()
	
	if err := hr.watcher.Stop(); err != nil {
		hr.logger.Error("Error stopping file watcher", zap.Error(err))
		return err
	}

	close(hr.reloadChan)
	hr.logger.Info("Hot reload system stopped")
	return nil
}

// onFileEvent handles file system events
func (hr *HotReloader) onFileEvent(event FileEvent) {
	select {
	case hr.reloadChan <- event:
	case <-hr.ctx.Done():
	default:
		hr.logger.Warn("Reload channel full, dropping event", 
			zap.String("path", event.Path),
			zap.String("operation", event.Operation),
		)
	}
}

// processReloads processes reload events
func (hr *HotReloader) processReloads() {
	for {
		select {
		case <-hr.ctx.Done():
			return
		case event, ok := <-hr.reloadChan:
			if !ok {
				return
			}
			hr.handleReloadEvent(event)
		}
	}
}

// handleReloadEvent handles a single reload event
func (hr *HotReloader) handleReloadEvent(event FileEvent) {
	if hr.reloading.Load() {
		hr.logger.Debug("Reload already in progress, skipping event",
			zap.String("path", event.Path),
		)
		return
	}

	hr.logger.Info("Processing file change event",
		zap.String("path", event.Path),
		zap.String("operation", event.Operation),
		zap.Time("timestamp", event.Timestamp),
	)

	// Determine reload type based on file path
	var result ReloadResult
	switch {
	case hr.isConfigFile(event.Path):
		result = hr.reloadConfig()
	case hr.isSpecFile(event.Path):
		result = hr.reloadSpec()
	default:
		hr.logger.Warn("Unknown file changed", zap.String("path", event.Path))
		return
	}

	// Log result
	hr.logReloadResult(result)
	
	// Update metrics
	hr.updateMetrics(result)
}

// reloadConfig reloads only the configuration
func (hr *HotReloader) reloadConfig() ReloadResult {
	start := time.Now()
	result := ReloadResult{
		ReloadType: "config",
		Timestamp:  start,
	}

	hr.reloading.Store(true)
	defer hr.reloading.Store(false)

	hr.logger.Info("Reloading configuration...")

	// Load new configuration
	newConfig, err := hr.loadConfig()
	if err != nil {
		result.Error = fmt.Errorf("failed to load new config: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Validate configuration
	if err := hr.validateConfig(newConfig); err != nil {
		result.Error = fmt.Errorf("invalid new config: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Restart server with new configuration
	if err := hr.server.Restart(newConfig, hr.currentSpec); err != nil {
		result.Error = fmt.Errorf("failed to restart server with new config: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Update current configuration
	hr.currentConfig = newConfig
	hr.config = &newConfig.HotReload

	result.Success = true
	result.Duration = time.Since(start)
	return result
}

// reloadSpec reloads only the OpenAPI specification
func (hr *HotReloader) reloadSpec() ReloadResult {
	start := time.Now()
	result := ReloadResult{
		ReloadType: "spec",
		Timestamp:  start,
	}

	hr.reloading.Store(true)
	defer hr.reloading.Store(false)

	hr.logger.Info("Reloading OpenAPI specification...")

	// Load new specification
	newSpec, err := hr.loadSpec()
	if err != nil {
		result.Error = fmt.Errorf("failed to load new spec: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Validate specification
	if err := hr.validateSpec(newSpec); err != nil {
		result.Error = fmt.Errorf("invalid new spec: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Restart server with new specification
	if err := hr.server.Restart(hr.currentConfig, newSpec); err != nil {
		result.Error = fmt.Errorf("failed to restart server with new spec: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Update current specification
	hr.currentSpec = newSpec

	result.Success = true
	result.Duration = time.Since(start)
	return result
}

// fullReload performs a complete reload of both config and spec
func (hr *HotReloader) fullReload() ReloadResult {
	start := time.Now()
	result := ReloadResult{
		ReloadType: "full",
		Timestamp:  start,
	}

	hr.reloading.Store(true)
	defer hr.reloading.Store(false)

	hr.logger.Info("Performing full reload...")

	// Load new configuration
	newConfig, err := hr.loadConfig()
	if err != nil {
		result.Error = fmt.Errorf("failed to load new config: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Load new specification
	newSpec, err := hr.loadSpec()
	if err != nil {
		result.Error = fmt.Errorf("failed to load new spec: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Validate both
	if err := hr.validateConfig(newConfig); err != nil {
		result.Error = fmt.Errorf("invalid new config: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	if err := hr.validateSpec(newSpec); err != nil {
		result.Error = fmt.Errorf("invalid new spec: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Restart server with both new config and spec
	if err := hr.server.Restart(newConfig, newSpec); err != nil {
		result.Error = fmt.Errorf("failed to restart server: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Update current state
	hr.currentConfig = newConfig
	hr.currentSpec = newSpec
	hr.config = &newConfig.HotReload

	result.Success = true
	result.Duration = time.Since(start)
	return result
}

// Helper functions

func (hr *HotReloader) isConfigFile(path string) bool {
	return path == hr.configPath
}

func (hr *HotReloader) isSpecFile(path string) bool {
	return path == hr.specPath
}

func (hr *HotReloader) loadConfig() (*config.Config, error) {
	return config.LoadConfig(hr.configPath)
}

func (hr *HotReloader) loadSpec() (*openapi.Specification, error) {
	return openapi.LoadSpecification(hr.specPath)
}

func (hr *HotReloader) validateConfig(cfg *config.Config) error {
	return config.ValidateConfig(cfg)
}

func (hr *HotReloader) validateSpec(spec *openapi.Specification) error {
	return openapi.ValidateSpecification(spec)
}

func (hr *HotReloader) logReloadResult(result ReloadResult) {
	fields := []zap.Field{
		zap.String("type", result.ReloadType),
		zap.Duration("duration", result.Duration),
		zap.Time("timestamp", result.Timestamp),
		zap.Bool("success", result.Success),
	}

	if result.Error != nil {
		fields = append(fields, zap.Error(result.Error))
		hr.logger.Error("Reload failed", fields...)
	} else {
		hr.logger.Info("Reload completed successfully", fields...)
	}
}

func (hr *HotReloader) updateMetrics(result ReloadResult) {
	hr.totalReloads.Add(1)
	if result.Success {
		hr.successReloads.Add(1)
	} else {
		hr.failedReloads.Add(1)
	}
	hr.lastReloadTime.Store(result.Timestamp)
}

// Public methods for manual reload

// ReloadConfig manually triggers a configuration reload
func (hr *HotReloader) ReloadConfig() error {
	result := hr.reloadConfig()
	hr.logReloadResult(result)
	hr.updateMetrics(result)
	
	if result.Error != nil {
		return result.Error
	}
	return nil
}

// ReloadSpec manually triggers a specification reload  
func (hr *HotReloader) ReloadSpec() error {
	result := hr.reloadSpec()
	hr.logReloadResult(result)
	hr.updateMetrics(result)
	
	if result.Error != nil {
		return result.Error
	}
	return nil
}

// FullReload manually triggers a full reload
func (hr *HotReloader) FullReload() error {
	result := hr.fullReload()
	hr.logReloadResult(result)
	hr.updateMetrics(result)
	
	if result.Error != nil {
		return result.Error
	}
	return nil
}

// GetMetrics returns hot reload metrics
func (hr *HotReloader) GetMetrics() map[string]interface{} {
	var lastReload time.Time
	if val := hr.lastReloadTime.Load(); val != nil {
		lastReload = val.(time.Time)
	}

	return map[string]interface{}{
		"enabled":        hr.config.Enabled,
		"total_reloads":  hr.totalReloads.Load(),
		"success_reloads": hr.successReloads.Load(),
		"failed_reloads": hr.failedReloads.Load(),
		"last_reload":    lastReload,
		"reloading":      hr.reloading.Load(),
	}
}

// IsReloading returns true if a reload is currently in progress
func (hr *HotReloader) IsReloading() bool {
	return hr.reloading.Load()
}