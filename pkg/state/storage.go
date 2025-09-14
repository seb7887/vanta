package state

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type StateStorage interface {
	Load() (map[string]*StateValue, error)
	Save(data map[string]*StateValue) error
	Close() error
}

func NewStorage(config StorageConfig) (StateStorage, error) {
	switch config.Type {
	case "memory":
		return NewMemoryStorage(), nil
	case "file":
		return NewFileStorage(config.FilePath)
	case "":
		return NewMemoryStorage(), nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Type)
	}
}

type MemoryStorage struct {
	data map[string]*StateValue
	mu   sync.RWMutex
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		data: make(map[string]*StateValue),
	}
}

func (s *MemoryStorage) Load() (map[string]*StateValue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*StateValue)
	for key, value := range s.data {
		result[key] = value
	}
	return result, nil
}

func (s *MemoryStorage) Save(data map[string]*StateValue) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]*StateValue)
	for key, value := range data {
		s.data[key] = value
	}
	return nil
}

func (s *MemoryStorage) Close() error {
	return nil
}

type FileStorage struct {
	filePath string
	mu       sync.RWMutex
}

func NewFileStorage(filePath string) (*FileStorage, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	// Ensure the directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return &FileStorage{
		filePath: filePath,
	}, nil
}

func (s *FileStorage) Load() (map[string]*StateValue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if file exists
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return make(map[string]*StateValue), nil
	}

	data, err := ioutil.ReadFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var result map[string]*StateValue
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state data: %w", err)
	}

	if result == nil {
		result = make(map[string]*StateValue)
	}

	return result, nil
}

func (s *FileStorage) Save(data map[string]*StateValue) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state data: %w", err)
	}

	// Write to temporary file first, then rename for atomic operation
	tempFile := s.filePath + ".tmp"
	if err := ioutil.WriteFile(tempFile, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}

	if err := os.Rename(tempFile, s.filePath); err != nil {
		os.Remove(tempFile) // Clean up temp file
		return fmt.Errorf("failed to rename temporary state file: %w", err)
	}

	return nil
}

func (s *FileStorage) Close() error {
	return nil
}

// BackupStorage wraps another storage and creates backups
type BackupStorage struct {
	primary    StateStorage
	backupDir  string
	maxBackups int
	mu         sync.RWMutex
}

func NewBackupStorage(primary StateStorage, backupDir string, maxBackups int) (*BackupStorage, error) {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &BackupStorage{
		primary:    primary,
		backupDir:  backupDir,
		maxBackups: maxBackups,
	}, nil
}

func (s *BackupStorage) Load() (map[string]*StateValue, error) {
	return s.primary.Load()
}

func (s *BackupStorage) Save(data map[string]*StateValue) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Save to primary storage first
	if err := s.primary.Save(data); err != nil {
		return err
	}

	// Create backup
	return s.createBackup(data)
}

func (s *BackupStorage) Close() error {
	return s.primary.Close()
}

func (s *BackupStorage) createBackup(data map[string]*StateValue) error {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFile := filepath.Join(s.backupDir, fmt.Sprintf("state_backup_%s.json", timestamp))

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup data: %w", err)
	}

	if err := ioutil.WriteFile(backupFile, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	// Clean up old backups
	return s.cleanupOldBackups()
}

func (s *BackupStorage) cleanupOldBackups() error {
	if s.maxBackups <= 0 {
		return nil
	}

	files, err := ioutil.ReadDir(s.backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	// Filter backup files and sort by modification time
	var backupFiles []os.FileInfo
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			backupFiles = append(backupFiles, file)
		}
	}

	if len(backupFiles) <= s.maxBackups {
		return nil
	}

	// Sort by modification time (oldest first)
	for i := 0; i < len(backupFiles)-1; i++ {
		for j := i + 1; j < len(backupFiles); j++ {
			if backupFiles[i].ModTime().After(backupFiles[j].ModTime()) {
				backupFiles[i], backupFiles[j] = backupFiles[j], backupFiles[i]
			}
		}
	}

	// Remove oldest backups
	toRemove := len(backupFiles) - s.maxBackups
	for i := 0; i < toRemove; i++ {
		filePath := filepath.Join(s.backupDir, backupFiles[i].Name())
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove old backup %s: %w", filePath, err)
		}
	}

	return nil
}

// RestoreFromBackup restores state from a backup file
func (s *BackupStorage) RestoreFromBackup(backupFile string) (map[string]*StateValue, error) {
	data, err := ioutil.ReadFile(backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file: %w", err)
	}

	var result map[string]*StateValue
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup data: %w", err)
	}

	return result, nil
}

// ListBackups returns a list of available backup files
func (s *BackupStorage) ListBackups() ([]string, error) {
	files, err := ioutil.ReadDir(s.backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backupFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			backupFiles = append(backupFiles, filepath.Join(s.backupDir, file.Name()))
		}
	}

	return backupFiles, nil
}