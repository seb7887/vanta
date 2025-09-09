package recorder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"vanta/pkg/config"
)

// Storage defines the interface for recording storage backends
type Storage interface {
	Save(recording *Recording) error
	Load(id string) (*Recording, error)
	List(filter ListFilter) ([]*Recording, error)
	Delete(id string) error
	DeleteAll() error
	GetStats() StorageStats
	Close() error
}

// FileStorage implements file-based storage for recordings
type FileStorage struct {
	directory   string
	format      string
	maxFiles    int
	mu          sync.RWMutex
	logger      *zap.Logger
	index       map[string]*RecordingIndex // In-memory index for performance
	indexFile   string
}

// NewFileStorage creates a new file-based storage instance
func NewFileStorage(config *config.StorageConfig, logger *zap.Logger) (*FileStorage, error) {
	if config.Directory == "" {
		return nil, fmt.Errorf("storage directory cannot be empty")
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(config.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	format := config.Format
	if format == "" {
		format = "jsonlines"
	}

	storage := &FileStorage{
		directory: config.Directory,
		format:    format,
		maxFiles:  1000, // Default max files
		logger:    logger,
		index:     make(map[string]*RecordingIndex),
		indexFile: filepath.Join(config.Directory, "index.json"),
	}

	// Load existing index
	if err := storage.loadIndex(); err != nil {
		logger.Warn("Failed to load storage index", zap.Error(err))
	}

	return storage, nil
}

// Save stores a recording to the filesystem
func (fs *FileStorage) Save(recording *Recording) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if recording == nil {
		return fmt.Errorf("recording cannot be nil")
	}

	if recording.ID == "" {
		return fmt.Errorf("recording ID cannot be empty")
	}

	// Generate filename
	filename := fs.generateFilename(recording)
	filepath := filepath.Join(fs.directory, filename)

	// Save recording to file
	if err := fs.saveToFile(recording, filepath); err != nil {
		return fmt.Errorf("failed to save recording to file: %w", err)
	}

	// Update index
	fs.index[recording.ID] = &RecordingIndex{
		ID:        recording.ID,
		Timestamp: recording.Timestamp,
		Method:    recording.Request.Method,
		URI:       recording.Request.URI,
		Status:    recording.Response.StatusCode,
		Filename:  filename,
	}

	// Save index
	if err := fs.saveIndex(); err != nil {
		fs.logger.Warn("Failed to save storage index", zap.Error(err))
	}

	// Clean up old files if necessary
	fs.cleanupOldFiles()

	return nil
}

// Load retrieves a recording by ID
func (fs *FileStorage) Load(id string) (*Recording, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if id == "" {
		return nil, fmt.Errorf("recording ID cannot be empty")
	}

	// Find in index
	index, exists := fs.index[id]
	if !exists {
		return nil, fmt.Errorf("recording not found: %s", id)
	}

	// Load from file
	filepath := filepath.Join(fs.directory, index.Filename)
	return fs.loadFromFile(filepath)
}

// List returns recordings matching the given filter
func (fs *FileStorage) List(filter ListFilter) ([]*Recording, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var recordings []*Recording
	var indices []*RecordingIndex

	// Filter indices
	for _, idx := range fs.index {
		if fs.matchesFilter(idx, filter) {
			indices = append(indices, idx)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(indices, func(i, j int) bool {
		return indices[i].Timestamp.After(indices[j].Timestamp)
	})

	// Apply offset and limit
	start := filter.Offset
	if start >= len(indices) {
		return recordings, nil
	}

	end := len(indices)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}

	indices = indices[start:end]

	// Load recordings
	for _, idx := range indices {
		filepath := filepath.Join(fs.directory, idx.Filename)
		recording, err := fs.loadFromFile(filepath)
		if err != nil {
			fs.logger.Warn("Failed to load recording",
				zap.String("id", idx.ID),
				zap.Error(err))
			continue
		}
		recordings = append(recordings, recording)
	}

	return recordings, nil
}

// Delete removes a recording by ID
func (fs *FileStorage) Delete(id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if id == "" {
		return fmt.Errorf("recording ID cannot be empty")
	}

	// Find in index
	index, exists := fs.index[id]
	if !exists {
		return fmt.Errorf("recording not found: %s", id)
	}

	// Delete file
	filepath := filepath.Join(fs.directory, index.Filename)
	if err := os.Remove(filepath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete recording file: %w", err)
	}

	// Remove from index
	delete(fs.index, id)

	// Save index
	if err := fs.saveIndex(); err != nil {
		fs.logger.Warn("Failed to save storage index", zap.Error(err))
	}

	return nil
}

// DeleteAll removes all recordings
func (fs *FileStorage) DeleteAll() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Delete all recording files
	for _, index := range fs.index {
		filepath := filepath.Join(fs.directory, index.Filename)
		if err := os.Remove(filepath); err != nil && !os.IsNotExist(err) {
			fs.logger.Warn("Failed to delete recording file",
				zap.String("file", filepath),
				zap.Error(err))
		}
	}

	// Clear index
	fs.index = make(map[string]*RecordingIndex)

	// Save empty index
	if err := fs.saveIndex(); err != nil {
		fs.logger.Warn("Failed to save storage index", zap.Error(err))
	}

	return nil
}

// GetStats returns storage statistics
func (fs *FileStorage) GetStats() StorageStats {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	stats := StorageStats{
		TotalRecordings: int64(len(fs.index)),
	}

	if len(fs.index) == 0 {
		return stats
	}

	// Calculate size and find oldest/newest
	var oldest, newest time.Time
	first := true

	for _, index := range fs.index {
		if first {
			oldest = index.Timestamp
			newest = index.Timestamp
			first = false
		} else {
			if index.Timestamp.Before(oldest) {
				oldest = index.Timestamp
			}
			if index.Timestamp.After(newest) {
				newest = index.Timestamp
			}
		}

		// Get file size
		filepath := filepath.Join(fs.directory, index.Filename)
		if info, err := os.Stat(filepath); err == nil {
			stats.TotalSize += info.Size()
		}
	}

	stats.OldestRecording = oldest
	stats.NewestRecording = newest

	return stats
}

// Close closes the storage
func (fs *FileStorage) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Save index one final time
	return fs.saveIndex()
}

// generateFilename creates a filename for a recording
func (fs *FileStorage) generateFilename(recording *Recording) string {
	timestamp := recording.Timestamp.Format("20060102-150405")
	switch fs.format {
	case "json":
		return fmt.Sprintf("%s-%s.json", timestamp, recording.ID)
	default: // jsonlines
		return fmt.Sprintf("%s-%s.jsonl", timestamp, recording.ID)
	}
}

// saveToFile saves a recording to a file
func (fs *FileStorage) saveToFile(recording *Recording, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(recording)
}

// loadFromFile loads a recording from a file
func (fs *FileStorage) loadFromFile(filepath string) (*Recording, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var recording Recording
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&recording); err != nil {
		return nil, err
	}

	return &recording, nil
}

// loadIndex loads the recording index from disk
func (fs *FileStorage) loadIndex() error {
	file, err := os.Open(fs.indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Index file doesn't exist yet, that's fine
			return nil
		}
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(&fs.index)
}

// saveIndex saves the recording index to disk
func (fs *FileStorage) saveIndex() error {
	file, err := os.Create(fs.indexFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(fs.index)
}

// matchesFilter checks if a recording index matches the given filter
func (fs *FileStorage) matchesFilter(index *RecordingIndex, filter ListFilter) bool {
	// Time range filter
	if !filter.StartTime.IsZero() && index.Timestamp.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && index.Timestamp.After(filter.EndTime) {
		return false
	}

	// Method filter
	if len(filter.Methods) > 0 {
		found := false
		for _, method := range filter.Methods {
			if strings.EqualFold(index.Method, method) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Endpoint filter (simple substring matching)
	if len(filter.Endpoints) > 0 {
		found := false
		for _, endpoint := range filter.Endpoints {
			if strings.Contains(index.URI, endpoint) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Status code filter
	if len(filter.StatusCodes) > 0 {
		found := false
		for _, status := range filter.StatusCodes {
			if index.Status == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// cleanupOldFiles removes old recordings if we exceed the maximum file count
func (fs *FileStorage) cleanupOldFiles() {
	if len(fs.index) <= fs.maxFiles {
		return
	}

	// Sort indices by timestamp (oldest first)
	var indices []*RecordingIndex
	for _, idx := range fs.index {
		indices = append(indices, idx)
	}

	sort.Slice(indices, func(i, j int) bool {
		return indices[i].Timestamp.Before(indices[j].Timestamp)
	})

	// Remove oldest files
	toRemove := len(indices) - fs.maxFiles
	for i := 0; i < toRemove; i++ {
		idx := indices[i]
		filepath := filepath.Join(fs.directory, idx.Filename)
		if err := os.Remove(filepath); err != nil && !os.IsNotExist(err) {
			fs.logger.Warn("Failed to cleanup old recording",
				zap.String("file", filepath),
				zap.Error(err))
		}
		delete(fs.index, idx.ID)
	}

	fs.logger.Info("Cleaned up old recordings",
		zap.Int("removed", toRemove),
		zap.Int("remaining", len(fs.index)))
}

// MemoryStorage implements in-memory storage for recordings (for testing)
type MemoryStorage struct {
	recordings map[string]*Recording
	mu         sync.RWMutex
}

// NewMemoryStorage creates a new in-memory storage instance
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		recordings: make(map[string]*Recording),
	}
}

// Save stores a recording in memory
func (ms *MemoryStorage) Save(recording *Recording) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if recording == nil {
		return fmt.Errorf("recording cannot be nil")
	}

	if recording.ID == "" {
		return fmt.Errorf("recording ID cannot be empty")
	}

	ms.recordings[recording.ID] = recording
	return nil
}

// Load retrieves a recording by ID from memory
func (ms *MemoryStorage) Load(id string) (*Recording, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if id == "" {
		return nil, fmt.Errorf("recording ID cannot be empty")
	}

	recording, exists := ms.recordings[id]
	if !exists {
		return nil, fmt.Errorf("recording not found: %s", id)
	}

	return recording, nil
}

// List returns recordings matching the given filter from memory
func (ms *MemoryStorage) List(filter ListFilter) ([]*Recording, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var recordings []*Recording

	for _, recording := range ms.recordings {
		if ms.matchesFilter(recording, filter) {
			recordings = append(recordings, recording)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(recordings, func(i, j int) bool {
		return recordings[i].Timestamp.After(recordings[j].Timestamp)
	})

	// Apply offset and limit
	start := filter.Offset
	if start >= len(recordings) {
		return []*Recording{}, nil
	}

	end := len(recordings)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}

	return recordings[start:end], nil
}

// Delete removes a recording by ID from memory
func (ms *MemoryStorage) Delete(id string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if id == "" {
		return fmt.Errorf("recording ID cannot be empty")
	}

	if _, exists := ms.recordings[id]; !exists {
		return fmt.Errorf("recording not found: %s", id)
	}

	delete(ms.recordings, id)
	return nil
}

// DeleteAll removes all recordings from memory
func (ms *MemoryStorage) DeleteAll() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.recordings = make(map[string]*Recording)
	return nil
}

// GetStats returns storage statistics from memory
func (ms *MemoryStorage) GetStats() StorageStats {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	stats := StorageStats{
		TotalRecordings: int64(len(ms.recordings)),
	}

	if len(ms.recordings) == 0 {
		return stats
	}

	var oldest, newest time.Time
	first := true

	for _, recording := range ms.recordings {
		if first {
			oldest = recording.Timestamp
			newest = recording.Timestamp
			first = false
		} else {
			if recording.Timestamp.Before(oldest) {
				oldest = recording.Timestamp
			}
			if recording.Timestamp.After(newest) {
				newest = recording.Timestamp
			}
		}
	}

	stats.OldestRecording = oldest
	stats.NewestRecording = newest

	return stats
}

// Close closes the memory storage (no-op)
func (ms *MemoryStorage) Close() error {
	return nil
}

// matchesFilter checks if a recording matches the given filter
func (ms *MemoryStorage) matchesFilter(recording *Recording, filter ListFilter) bool {
	// Time range filter
	if !filter.StartTime.IsZero() && recording.Timestamp.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && recording.Timestamp.After(filter.EndTime) {
		return false
	}

	// Method filter
	if len(filter.Methods) > 0 {
		found := false
		for _, method := range filter.Methods {
			if strings.EqualFold(recording.Request.Method, method) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Endpoint filter (simple substring matching)
	if len(filter.Endpoints) > 0 {
		found := false
		for _, endpoint := range filter.Endpoints {
			if strings.Contains(recording.Request.URI, endpoint) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Status code filter
	if len(filter.StatusCodes) > 0 {
		found := false
		for _, status := range filter.StatusCodes {
			if recording.Response.StatusCode == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}