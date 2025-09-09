package recorder

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"vanta/pkg/config"
)

func TestMemoryStorage(t *testing.T) {
	storage := NewMemoryStorage()
	testStorageInterface(t, storage)
}

func TestFileStorage(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "recorder_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	logger := zaptest.NewLogger(t)
	config := &config.StorageConfig{
		Type:      "file",
		Directory: tempDir,
		Format:    "json",
	}

	storage, err := NewFileStorage(config, logger)
	require.NoError(t, err)
	defer storage.Close()

	testStorageInterface(t, storage)
}

func testStorageInterface(t *testing.T, storage Storage) {
	// Test Save and Load
	recording := &Recording{
		ID:        "test-recording-1",
		Timestamp: time.Now(),
		Request: RecordedRequest{
			Method: "GET",
			URI:    "http://example.com/api/test",
			Headers: map[string]string{
				"User-Agent": "test-agent",
			},
			Body:        []byte(`{"test": true}`),
			QueryParams: map[string]string{"q": "search"},
			ContentType: "application/json",
		},
		Response: RecordedResponse{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body:        []byte(`{"result": "success"}`),
			ContentType: "application/json",
		},
		Metadata: RecordingMetadata{
			Source:    "live",
			ClientIP:  "127.0.0.1",
			UserAgent: "test-agent",
			RequestID: "req-123",
		},
		Duration: 150 * time.Millisecond,
	}

	// Save recording
	err := storage.Save(recording)
	require.NoError(t, err)

	// Load recording
	loaded, err := storage.Load("test-recording-1")
	require.NoError(t, err)
	assert.Equal(t, recording.ID, loaded.ID)
	assert.Equal(t, recording.Request.Method, loaded.Request.Method)
	assert.Equal(t, recording.Request.URI, loaded.Request.URI)
	assert.Equal(t, recording.Response.StatusCode, loaded.Response.StatusCode)

	// Test List
	recordings, err := storage.List(ListFilter{})
	require.NoError(t, err)
	assert.Len(t, recordings, 1)
	assert.Equal(t, "test-recording-1", recordings[0].ID)

	// Test List with limit
	recordings, err = storage.List(ListFilter{Limit: 1})
	require.NoError(t, err)
	assert.Len(t, recordings, 1)

	// Save another recording
	recording2 := &Recording{
		ID:        "test-recording-2",
		Timestamp: time.Now().Add(1 * time.Minute),
		Request: RecordedRequest{
			Method: "POST",
			URI:    "http://example.com/api/create",
		},
		Response: RecordedResponse{
			StatusCode: 201,
		},
		Duration: 200 * time.Millisecond,
	}

	err = storage.Save(recording2)
	require.NoError(t, err)

	// Test List with filters
	recordings, err = storage.List(ListFilter{
		Methods: []string{"GET"},
	})
	require.NoError(t, err)
	assert.Len(t, recordings, 1)
	assert.Equal(t, "test-recording-1", recordings[0].ID)

	recordings, err = storage.List(ListFilter{
		StatusCodes: []int{201},
	})
	require.NoError(t, err)
	assert.Len(t, recordings, 1)
	assert.Equal(t, "test-recording-2", recordings[0].ID)

	// Test GetStats
	stats := storage.GetStats()
	assert.Equal(t, int64(2), stats.TotalRecordings)

	// Test Delete
	err = storage.Delete("test-recording-1")
	require.NoError(t, err)

	_, err = storage.Load("test-recording-1")
	assert.Error(t, err)

	recordings, err = storage.List(ListFilter{})
	require.NoError(t, err)
	assert.Len(t, recordings, 1)

	// Test DeleteAll
	err = storage.DeleteAll()
	require.NoError(t, err)

	recordings, err = storage.List(ListFilter{})
	require.NoError(t, err)
	assert.Len(t, recordings, 0)
}

func TestFileStorage_FileOperations(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "recorder_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	logger := zaptest.NewLogger(t)
	config := &config.StorageConfig{
		Type:      "file",
		Directory: tempDir,
		Format:    "json",
	}

	storage, err := NewFileStorage(config, logger)
	require.NoError(t, err)
	defer storage.Close()

	recording := &Recording{
		ID:        "test-file-recording",
		Timestamp: time.Now(),
		Request: RecordedRequest{
			Method: "GET",
			URI:    "http://example.com/test",
		},
		Response: RecordedResponse{
			StatusCode: 200,
		},
		Duration: 100 * time.Millisecond,
	}

	// Save recording
	err = storage.Save(recording)
	require.NoError(t, err)

	// Check that file was created
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	
	// Should have recording file and index file
	assert.GreaterOrEqual(t, len(files), 2)

	// Check index file exists
	indexPath := filepath.Join(tempDir, "index.json")
	_, err = os.Stat(indexPath)
	assert.NoError(t, err)

	// Check recording file exists
	var recordingFile string
	for _, file := range files {
		if file.Name() != "index.json" {
			recordingFile = file.Name()
			break
		}
	}
	assert.NotEmpty(t, recordingFile)
	assert.Contains(t, recordingFile, "test-file-recording")
	assert.True(t, len(recordingFile) > 5 && recordingFile[len(recordingFile)-5:] == ".json")
}

func TestStorageConfig_Validation(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Test empty directory
	_, err := NewFileStorage(&config.StorageConfig{}, logger)
	assert.Error(t, err)

	// Test non-existent directory that can't be created
	_, err = NewFileStorage(&config.StorageConfig{
		Directory: "/invalid/path/that/does/not/exist",
	}, logger)
	assert.Error(t, err)
}

func TestListFilter_TimeRange(t *testing.T) {
	storage := NewMemoryStorage()

	now := time.Now()
	
	// Create recordings with different timestamps
	recordings := []*Recording{
		{
			ID:        "old-recording",
			Timestamp: now.Add(-2 * time.Hour),
			Request:   RecordedRequest{Method: "GET"},
			Response:  RecordedResponse{StatusCode: 200},
		},
		{
			ID:        "recent-recording",
			Timestamp: now.Add(-30 * time.Minute),
			Request:   RecordedRequest{Method: "POST"},
			Response:  RecordedResponse{StatusCode: 201},
		},
		{
			ID:        "newest-recording",
			Timestamp: now,
			Request:   RecordedRequest{Method: "PUT"},
			Response:  RecordedResponse{StatusCode: 200},
		},
	}

	// Save all recordings
	for _, recording := range recordings {
		err := storage.Save(recording)
		require.NoError(t, err)
	}

	// Test StartTime filter
	results, err := storage.List(ListFilter{
		StartTime: now.Add(-1 * time.Hour),
	})
	require.NoError(t, err)
	assert.Len(t, results, 2) // recent and newest

	// Test EndTime filter
	results, err = storage.List(ListFilter{
		EndTime: now.Add(-1 * time.Hour),
	})
	require.NoError(t, err)
	assert.Len(t, results, 1) // only old

	// Test time range
	results, err = storage.List(ListFilter{
		StartTime: now.Add(-90 * time.Minute),
		EndTime:   now.Add(-15 * time.Minute),
	})
	require.NoError(t, err)
	assert.Len(t, results, 1) // only recent
}

func TestListFilter_Pagination(t *testing.T) {
	storage := NewMemoryStorage()

	// Create multiple recordings
	for i := 0; i < 10; i++ {
		recording := &Recording{
			ID:        fmt.Sprintf("recording-%d", i),
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Request:   RecordedRequest{Method: "GET"},
			Response:  RecordedResponse{StatusCode: 200},
		}
		err := storage.Save(recording)
		require.NoError(t, err)
	}

	// Test limit
	results, err := storage.List(ListFilter{Limit: 5})
	require.NoError(t, err)
	assert.Len(t, results, 5)

	// Test offset
	results, err = storage.List(ListFilter{Offset: 5, Limit: 5})
	require.NoError(t, err)
	assert.Len(t, results, 5)

	// Test offset beyond available records
	results, err = storage.List(ListFilter{Offset: 15})
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestMemoryStorage_EdgeCases(t *testing.T) {
	storage := NewMemoryStorage()

	// Test nil recording
	err := storage.Save(nil)
	assert.Error(t, err)

	// Test empty ID
	err = storage.Save(&Recording{ID: ""})
	assert.Error(t, err)

	// Test load non-existent
	_, err = storage.Load("non-existent")
	assert.Error(t, err)

	// Test delete non-existent
	err = storage.Delete("non-existent")
	assert.Error(t, err)

	// Test empty ID operations
	_, err = storage.Load("")
	assert.Error(t, err)

	err = storage.Delete("")
	assert.Error(t, err)
}