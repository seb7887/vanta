package hotreload

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	Operation string // create, write, remove, rename
	Timestamp time.Time
}

// FileWatcher watches files and directories for changes
type FileWatcher struct {
	watcher     *fsnotify.Watcher
	logger      *zap.Logger
	debouncer   *Debouncer
	watchedPaths map[string]bool
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(logger *zap.Logger, debounceDelay time.Duration) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &FileWatcher{
		watcher:      watcher,
		logger:       logger,
		debouncer:    NewDebouncer(debounceDelay),
		watchedPaths: make(map[string]bool),
		ctx:          ctx,
		cancel:       cancel,
	}, nil
}

// AddPath adds a file or directory to watch
func (fw *FileWatcher) AddPath(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Check if already watching
	if fw.watchedPaths[absPath] {
		return nil
	}

	// Add to fsnotify watcher
	if err := fw.watcher.Add(absPath); err != nil {
		return err
	}

	fw.watchedPaths[absPath] = true
	fw.logger.Debug("Added path to watcher", zap.String("path", absPath))
	
	return nil
}

// RemovePath removes a path from watching
func (fw *FileWatcher) RemovePath(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if !fw.watchedPaths[absPath] {
		return nil
	}

	if err := fw.watcher.Remove(absPath); err != nil {
		return err
	}

	delete(fw.watchedPaths, absPath)
	fw.logger.Debug("Removed path from watcher", zap.String("path", absPath))
	
	return nil
}

// Start starts the file watcher with a callback function
func (fw *FileWatcher) Start(callback func(FileEvent)) error {
	go fw.watchLoop(callback)
	fw.logger.Info("File watcher started", zap.Int("watched_paths", len(fw.watchedPaths)))
	return nil
}

// Stop stops the file watcher
func (fw *FileWatcher) Stop() error {
	fw.cancel()
	
	if err := fw.watcher.Close(); err != nil {
		fw.logger.Error("Error closing file watcher", zap.Error(err))
		return err
	}

	fw.debouncer.Stop()
	fw.logger.Info("File watcher stopped")
	return nil
}

// watchLoop runs the main event processing loop
func (fw *FileWatcher) watchLoop(callback func(FileEvent)) {
	for {
		select {
		case <-fw.ctx.Done():
			return
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event, callback)
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.logger.Error("File watcher error", zap.Error(err))
		}
	}
}

// handleEvent processes a single file system event
func (fw *FileWatcher) handleEvent(event fsnotify.Event, callback func(FileEvent)) {
	// Filter out irrelevant files
	if !fw.isRelevantFile(event.Name) {
		return
	}

	// Convert fsnotify operation to our operation string
	operation := fw.convertOperation(event.Op)
	
	fileEvent := FileEvent{
		Path:      event.Name,
		Operation: operation,
		Timestamp: time.Now(),
	}

	fw.logger.Debug("File event detected",
		zap.String("path", event.Name),
		zap.String("operation", operation),
	)

	// Use debouncer to avoid rapid successive events
	fw.debouncer.Debounce(event.Name, func() {
		callback(fileEvent)
	})
}

// isRelevantFile checks if the file is relevant for hot reload
func (fw *FileWatcher) isRelevantFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

// convertOperation converts fsnotify.Op to string
func (fw *FileWatcher) convertOperation(op fsnotify.Op) string {
	switch {
	case op&fsnotify.Create == fsnotify.Create:
		return "create"
	case op&fsnotify.Write == fsnotify.Write:
		return "write"
	case op&fsnotify.Remove == fsnotify.Remove:
		return "remove"
	case op&fsnotify.Rename == fsnotify.Rename:
		return "rename"
	default:
		return "unknown"
	}
}

// Debouncer helps prevent rapid successive events
type Debouncer struct {
	delay    time.Duration
	timers   map[string]*time.Timer
	mu       sync.Mutex
	stopped  bool
	stopChan chan struct{}
}

// NewDebouncer creates a new debouncer
func NewDebouncer(delay time.Duration) *Debouncer {
	return &Debouncer{
		delay:    delay,
		timers:   make(map[string]*time.Timer),
		stopChan: make(chan struct{}),
	}
}

// Debounce debounces a function call for a specific key
func (d *Debouncer) Debounce(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	// Cancel existing timer if any
	if timer, exists := d.timers[key]; exists {
		timer.Stop()
	}

	// Create new timer
	d.timers[key] = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		delete(d.timers, key)
		d.mu.Unlock()
		
		if !d.stopped {
			fn()
		}
	})
}

// Stop stops the debouncer and cancels all pending timers
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stopped = true
	
	// Cancel all existing timers
	for key, timer := range d.timers {
		timer.Stop()
		delete(d.timers, key)
	}
	
	close(d.stopChan)
}