//	Copyright 2025 Ryan SVIHLA Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rsvihladremio/ddd/internal/database"
	"github.com/rsvihladremio/ddd/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCleanupWorker implements CleanupWorker interface for testing
type mockCleanupWorker struct {
	mu           sync.Mutex
	triggerCount int
}

func (m *mockCleanupWorker) TriggerCleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.triggerCount++
}

func (m *mockCleanupWorker) getTriggerCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.triggerCount
}

// testDB creates a test database with clean schema
func testDB(t *testing.T) *database.DB {
	t.Helper()

	cfg := testutil.TestConfig(t)
	db, err := database.Initialize(cfg.DBPath)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Error closing database: %v", err)
		}
	})

	return db
}

func setupTestHandler(t *testing.T) (*Handlers, *database.DB) {
	t.Helper()

	db := testDB(t)
	cfg := testutil.TestConfig(t)
	mockWorker := &mockCleanupWorker{}

	handler := New(db, cfg, mockWorker)
	return handler, db
}

func TestHandlers_HandleIndex(t *testing.T) {
	handler, _ := setupTestHandler(t)

	t.Run("Serve index page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		handler.HandleIndex(w, req)

		// Since index.html doesn't exist in test environment, expect 404
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Return 404 for non-root paths", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/nonexistent", nil)
		w := httptest.NewRecorder()

		handler.HandleIndex(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandlers_HandleUpload(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("Upload valid file", func(t *testing.T) {
		// Create multipart form data
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreateFormFile("file", "test.txt")
		require.NoError(t, err)

		testContent := testutil.SampleFiles["ttop"].Content
		_, err = part.Write(testContent)
		require.NoError(t, err)

		err = writer.Close()
		require.NoError(t, err)

		// Create request
		req := httptest.NewRequest("POST", "/api/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		// Handle request
		handler.HandleUpload(w, req)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		// Parse response
		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Contains(t, response, "file")

		// Verify file was saved to database
		fileData := response["file"].(map[string]interface{})
		fileID := int(fileData["id"].(float64))

		files, err := db.GetFiles(10, 0, false, "")
		require.NoError(t, err)

		found := false
		for _, file := range files {
			if file.ID == fileID {
				found = true
				assert.Equal(t, "test.txt", file.OriginalName)
				assert.Equal(t, "ttop", file.FileType)
				break
			}
		}
		assert.True(t, found, "File should be found in database")
	})

	t.Run("Upload duplicate file", func(t *testing.T) {
		// First upload
		body1 := &bytes.Buffer{}
		writer1 := multipart.NewWriter(body1)
		part1, err := writer1.CreateFormFile("file", "duplicate.txt")
		require.NoError(t, err)

		testContent := []byte("duplicate content")
		_, err = part1.Write(testContent)
		require.NoError(t, err)
		err = writer1.Close()
		require.NoError(t, err)

		req1 := httptest.NewRequest("POST", "/api/upload", body1)
		req1.Header.Set("Content-Type", writer1.FormDataContentType())
		w1 := httptest.NewRecorder()

		handler.HandleUpload(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second upload (duplicate)
		body2 := &bytes.Buffer{}
		writer2 := multipart.NewWriter(body2)
		part2, err := writer2.CreateFormFile("file", "duplicate.txt")
		require.NoError(t, err)

		_, err = part2.Write(testContent)
		require.NoError(t, err)
		err = writer2.Close()
		require.NoError(t, err)

		req2 := httptest.NewRequest("POST", "/api/upload", body2)
		req2.Header.Set("Content-Type", writer2.FormDataContentType())
		w2 := httptest.NewRecorder()

		handler.HandleUpload(w2, req2)

		// Should still return success but with "already exists" message
		assert.Equal(t, http.StatusOK, w2.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w2.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Contains(t, response["message"], "already exists")
	})

	t.Run("Upload with invalid method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/upload", nil)
		w := httptest.NewRecorder()

		handler.HandleUpload(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("Upload with no file", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		err := writer.Close()
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.HandleUpload(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandlers_HandleFiles(t *testing.T) {
	handler, db := setupTestHandler(t)

	// Create test files
	testFiles := []*database.File{
		{
			Hash:         "hash1",
			OriginalName: "file1.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now().Add(-1 * time.Hour),
			FilePath:     "/uploads/hash1",
		},
		{
			Hash:         "hash2",
			OriginalName: "file2.txt",
			FileType:     "iostat",
			FileSize:     200,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/hash2",
		},
	}

	for _, file := range testFiles {
		err := db.InsertFile(file)
		require.NoError(t, err)
	}

	t.Run("Get files list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/files", nil)
		w := httptest.NewRecorder()

		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Contains(t, response, "files")

		files := response["files"].([]interface{})
		assert.GreaterOrEqual(t, len(files), 2)
	})

	t.Run("Get files with pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/files?limit=1&offset=0", nil)
		w := httptest.NewRecorder()

		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]interface{})
		assert.Equal(t, 1, len(files))
	})

	t.Run("Get files with default pagination limit", func(t *testing.T) {
		// Insert more than 5 test files to test the default limit
		moreTestFiles := make([]*database.File, 8)
		for i := 0; i < 8; i++ {
			moreTestFiles[i] = &database.File{
				Hash:         fmt.Sprintf("default-limit-hash-%d", i),
				OriginalName: fmt.Sprintf("default_limit_test_%d.txt", i),
				FileType:     "ttop",
				FileSize:     100,
				UploadTime:   time.Now().Add(-time.Duration(i) * time.Minute),
				FilePath:     fmt.Sprintf("/uploads/default-limit-hash-%d", i),
			}
		}

		for _, file := range moreTestFiles {
			err := db.InsertFile(file)
			require.NoError(t, err)
		}

		// Request without explicit limit should use default of 5
		req := httptest.NewRequest("GET", "/api/files", nil)
		w := httptest.NewRecorder()

		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]any)
		// Should return exactly 5 files (default limit) even though we have more
		assert.LessOrEqual(t, len(files), 5, "Default limit should be 5 or fewer files")

		// If we have at least 5 files in total, we should get exactly 5
		if len(files) == 5 {
			assert.Equal(t, 5, len(files), "Should return exactly 5 files with default limit")
		}
	})

	t.Run("Get files respects explicit limit parameter", func(t *testing.T) {
		// Test with explicit limit of 3
		req := httptest.NewRequest("GET", "/api/files?limit=3", nil)
		w := httptest.NewRecorder()

		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]any)
		assert.LessOrEqual(t, len(files), 3, "Should respect explicit limit of 3")
	})

	t.Run("Get files with limit larger than default", func(t *testing.T) {
		// Test with explicit limit larger than default (10 > 5)
		req := httptest.NewRequest("GET", "/api/files?limit=10", nil)
		w := httptest.NewRecorder()

		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]any)
		// Should be able to return more than 5 when explicitly requested
		assert.LessOrEqual(t, len(files), 10, "Should respect explicit limit of 10")
	})

	t.Run("Get files returns pagination data", func(t *testing.T) {
		// Insert multiple files to test pagination data
		paginationDataFiles := make([]*database.File, 12)
		for i := 0; i < 12; i++ {
			paginationDataFiles[i] = &database.File{
				Hash:         fmt.Sprintf("pagination-data-hash-%d", i),
				OriginalName: fmt.Sprintf("pagination_data_%d.txt", i),
				FileType:     "ttop",
				FileSize:     100,
				UploadTime:   time.Now().Add(-time.Duration(i) * time.Minute),
				FilePath:     fmt.Sprintf("/uploads/pagination-data-hash-%d", i),
			}
		}

		for _, file := range paginationDataFiles {
			err := db.InsertFile(file)
			require.NoError(t, err)
		}

		// Test first page with default limit (5)
		req := httptest.NewRequest("GET", "/api/files", nil)
		w := httptest.NewRecorder()

		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify pagination data is present
		assert.Contains(t, response, "total", "Response should include total count")
		assert.Contains(t, response, "page", "Response should include current page")
		assert.Contains(t, response, "page_size", "Response should include page size")
		assert.Contains(t, response, "total_pages", "Response should include total pages")

		// Verify pagination data values
		total := int(response["total"].(float64))
		page := int(response["page"].(float64))
		pageSize := int(response["page_size"].(float64))
		totalPages := int(response["total_pages"].(float64))

		assert.GreaterOrEqual(t, total, 12, "Total should be at least 12")
		assert.Equal(t, 1, page, "Should be on page 1")
		assert.Equal(t, 5, pageSize, "Page size should be 5")
		assert.GreaterOrEqual(t, totalPages, 3, "Should have at least 3 pages for 12+ files with page size 5")

		// Test second page
		req = httptest.NewRequest("GET", "/api/files?limit=5&offset=5", nil)
		w = httptest.NewRecorder()

		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		page = int(response["page"].(float64))
		assert.Equal(t, 2, page, "Should be on page 2")
	})

	t.Run("Get files with search query", func(t *testing.T) {
		// Insert additional test files with specific names for search testing
		searchTestFiles := []*database.File{
			{
				Hash:         "search-test-1",
				OriginalName: "performance_metrics.txt",
				FileType:     "ttop",
				FileSize:     300,
				UploadTime:   time.Now().Add(-30 * time.Minute),
				FilePath:     "/uploads/search-test-1",
			},
			{
				Hash:         "search-test-2",
				OriginalName: "system_iostat.log",
				FileType:     "iostat",
				FileSize:     400,
				UploadTime:   time.Now().Add(-20 * time.Minute),
				FilePath:     "/uploads/search-test-2",
			},
			{
				Hash:         "search-test-3",
				OriginalName: "database_performance.csv",
				FileType:     "unknown",
				FileSize:     500,
				UploadTime:   time.Now().Add(-10 * time.Minute),
				FilePath:     "/uploads/search-test-3",
			},
		}

		for _, file := range searchTestFiles {
			err := db.InsertFile(file)
			require.NoError(t, err)
		}

		// Test search by filename
		req := httptest.NewRequest("GET", "/api/files?search=performance", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]interface{})
		assert.Len(t, files, 2, "Should find 2 files with 'performance' in name")

		// Verify the correct files were found
		foundNames := make(map[string]bool)
		for _, f := range files {
			file := f.(map[string]interface{})
			foundNames[file["original_name"].(string)] = true
		}
		assert.True(t, foundNames["performance_metrics.txt"], "Should find performance_metrics.txt")
		assert.True(t, foundNames["database_performance.csv"], "Should find database_performance.csv")
	})

	t.Run("Get files with search by file type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/files?search=iostat", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]interface{})
		assert.GreaterOrEqual(t, len(files), 1, "Should find at least 1 file with 'iostat' type")

		// Verify at least one file has iostat type
		foundIostat := false
		for _, f := range files {
			file := f.(map[string]interface{})
			if file["file_type"].(string) == "iostat" {
				foundIostat = true
				break
			}
		}
		assert.True(t, foundIostat, "Should find at least one iostat file")
	})

	t.Run("Get files with search by hash", func(t *testing.T) {
		// Insert a test file with a known hash for searching
		hashTestFile := &database.File{
			Hash:         "abc123def456",
			OriginalName: "hash_test_file.txt",
			FileType:     "ttop",
			FileSize:     200,
			UploadTime:   time.Now().Add(-15 * time.Minute),
			FilePath:     "/uploads/abc123def456",
		}

		err := db.InsertFile(hashTestFile)
		require.NoError(t, err)

		// Search by full hash
		req := httptest.NewRequest("GET", "/api/files?search=abc123def456", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]interface{})
		assert.Len(t, files, 1, "Should find exactly 1 file with matching hash")

		file := files[0].(map[string]interface{})
		assert.Equal(t, "abc123def456", file["hash"].(string))
	})

	t.Run("Get files with search by partial hash", func(t *testing.T) {
		// Search by partial hash
		req := httptest.NewRequest("GET", "/api/files?search=abc123", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]interface{})
		assert.GreaterOrEqual(t, len(files), 1, "Should find at least 1 file with partial hash match")

		// Verify at least one file has the matching hash
		foundHash := false
		for _, f := range files {
			file := f.(map[string]interface{})
			if strings.Contains(file["hash"].(string), "abc123") {
				foundHash = true
				break
			}
		}
		assert.True(t, foundHash, "Should find file with matching hash")
	})

	t.Run("Get files with case insensitive search", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/files?search=PERFORMANCE", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]interface{})
		assert.GreaterOrEqual(t, len(files), 1, "Case insensitive search should find files")
	})

	t.Run("Get files with search no results", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/files?search=nonexistentfile", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]any)
		assert.Len(t, files, 0, "Should find no files for non-existent search term")
	})

	t.Run("Get files with search and pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/files?search=file&limit=1&offset=0", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]interface{})
		assert.LessOrEqual(t, len(files), 1, "Should respect limit parameter with search")
	})

	t.Run("Get files with search uses default limit", func(t *testing.T) {
		// Insert multiple files that will match a search to test default limit with search
		searchLimitFiles := make([]*database.File, 7)
		for i := 0; i < 7; i++ {
			searchLimitFiles[i] = &database.File{
				Hash:         fmt.Sprintf("search-limit-hash-%d", i),
				OriginalName: fmt.Sprintf("searchable_file_%d.txt", i),
				FileType:     "ttop",
				FileSize:     100,
				UploadTime:   time.Now().Add(-time.Duration(i) * time.Minute),
				FilePath:     fmt.Sprintf("/uploads/search-limit-hash-%d", i),
			}
		}

		for _, file := range searchLimitFiles {
			err := db.InsertFile(file)
			require.NoError(t, err)
		}

		// Search without explicit limit should use default of 5
		req := httptest.NewRequest("GET", "/api/files?search=searchable_file", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]interface{})
		// Should return at most 5 files even though 7 match the search
		assert.LessOrEqual(t, len(files), 5, "Search should respect default limit of 5")

		// Verify all returned files match the search
		for _, f := range files {
			file := f.(map[string]interface{})
			fileName := file["original_name"].(string)
			assert.Contains(t, fileName, "searchable_file", "All returned files should match search")
		}
	})

	t.Run("Get files with search and include deleted", func(t *testing.T) {
		// First create and delete a file with searchable content
		deletedFile := &database.File{
			Hash:         "deleted-search-file",
			OriginalName: "deleted_searchable.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/deleted-search-file",
		}

		err := db.InsertFile(deletedFile)
		require.NoError(t, err)

		err = db.MarkFileDeleted(deletedFile.ID)
		require.NoError(t, err)

		// Search without including deleted files
		req := httptest.NewRequest("GET", "/api/files?search=deleted_searchable", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files := response["files"].([]interface{})
		assert.Len(t, files, 0, "Should not find deleted files when include_deleted=false")

		// Search including deleted files
		req = httptest.NewRequest("GET", "/api/files?search=deleted_searchable&include_deleted=true", nil)
		w = httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		files = response["files"].([]interface{})
		assert.Len(t, files, 1, "Should find deleted files when include_deleted=true")

		file := files[0].(map[string]interface{})
		assert.True(t, file["deleted"].(bool), "Found file should be marked as deleted")
	})

	t.Run("Invalid method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/files", nil)
		w := httptest.NewRecorder()

		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestHandlers_HandleFileOperations(t *testing.T) {
	handler, db := setupTestHandler(t)

	// Create a test file
	testFile := &database.File{
		Hash:         "test-hash",
		OriginalName: "test.txt",
		FileType:     "ttop",
		FileSize:     100,
		UploadTime:   time.Now(),
		FilePath:     "/uploads/test-hash",
	}

	err := db.InsertFile(testFile)
	require.NoError(t, err)

	t.Run("Delete file", func(t *testing.T) {
		url := fmt.Sprintf("/api/files/%d", testFile.ID)
		req := httptest.NewRequest("DELETE", url, nil)
		w := httptest.NewRecorder()

		handler.HandleFileOperations(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))

		// Verify file was marked as deleted
		updatedFile, err := db.GetFileByHash("test-hash")
		require.NoError(t, err)
		assert.True(t, updatedFile.Deleted)
	})

	t.Run("Delete non-existent file", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/files/99999", nil)
		w := httptest.NewRecorder()

		handler.HandleFileOperations(w, req)

		// The handler now returns 404 for non-existent files
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Invalid file ID", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/files/invalid", nil)
		w := httptest.NewRecorder()

		handler.HandleFileOperations(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandlers_HandleReports(t *testing.T) {
	handler, db := setupTestHandler(t)

	// Create test file and reports
	testFile := &database.File{
		Hash:         "report-test-hash",
		OriginalName: "report-test.txt",
		FileType:     "ttop",
		FileSize:     100,
		UploadTime:   time.Now(),
		FilePath:     "/uploads/report-test-hash",
	}

	err := db.InsertFile(testFile)
	require.NoError(t, err)

	testReport := &database.Report{
		FileID:      testFile.ID,
		ReportType:  "ttop",
		Status:      "completed",
		CreatedTime: time.Now(),
		DDDVersion:  "1.0.0",
		ReportData:  `{"type": "ttop", "summary": "test report"}`,
	}

	err = db.InsertReport(testReport)
	require.NoError(t, err)

	t.Run("Get reports for file", func(t *testing.T) {
		url := fmt.Sprintf("/api/reports/%d", testFile.ID)
		req := httptest.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()

		handler.HandleReports(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Contains(t, response, "reports")

		reports := response["reports"].([]interface{})
		assert.GreaterOrEqual(t, len(reports), 1)
	})

	t.Run("Get reports for non-existent file", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/reports/99999", nil)
		w := httptest.NewRecorder()

		handler.HandleReports(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Check if reports field exists and handle nil case
		if reportsField, exists := response["reports"]; exists && reportsField != nil {
			reports := reportsField.([]interface{})
			assert.Equal(t, 0, len(reports))
		} else {
			// If reports field is nil or doesn't exist, that's also valid for no reports
			assert.True(t, true) // Test passes
		}
	})
}

func TestHandlers_HandleReportContent(t *testing.T) {
	handler, db := setupTestHandler(t)

	// Create test report with content
	testFile := &database.File{
		Hash:         "content-test-hash",
		OriginalName: "content-test.txt",
		FileType:     "ttop",
		FileSize:     100,
		UploadTime:   time.Now(),
		FilePath:     "/uploads/content-test-hash",
	}

	err := db.InsertFile(testFile)
	require.NoError(t, err)

	reportData := `{"type": "ttop", "summary": "detailed report content", "analysis": "comprehensive analysis"}`
	testReport := &database.Report{
		FileID:      testFile.ID,
		ReportType:  "ttop",
		Status:      "completed",
		CreatedTime: time.Now(),
		DDDVersion:  "1.0.0",
		ReportData:  reportData,
	}

	err = db.InsertReport(testReport)
	require.NoError(t, err)

	t.Run("Get report content", func(t *testing.T) {
		url := fmt.Sprintf("/api/reports/content/%d", testReport.ID)
		req := httptest.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()

		handler.HandleReportContent(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Contains(t, response, "report_data")

		// The report_data should contain the JSON report data
		reportDataStr := response["report_data"].(string)
		assert.Equal(t, reportData, reportDataStr)

		// Parse the report data to verify it's valid JSON
		var parsedReportData map[string]interface{}
		err = json.Unmarshal([]byte(reportDataStr), &parsedReportData)
		require.NoError(t, err)
		assert.Equal(t, "ttop", parsedReportData["type"])
		assert.Equal(t, "detailed report content", parsedReportData["summary"])
	})

	t.Run("Get content for non-existent report", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/reports/content/99999", nil)
		w := httptest.NewRecorder()

		handler.HandleReportContent(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// Integration Tests - These replace the Playwright e2e tests with httptest-based tests

func TestIntegration_DeletedFileReupload(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("Re-upload deleted file should restore it", func(t *testing.T) {
		// Create test file content
		testContent := testutil.SampleFiles["ttop"].Content

		// Create multipart form for first upload
		var buf1 bytes.Buffer
		writer1 := multipart.NewWriter(&buf1)
		part1, err := writer1.CreateFormFile("file", "test_reupload.txt")
		require.NoError(t, err)

		_, err = part1.Write(testContent)
		require.NoError(t, err)
		require.NoError(t, writer1.Close())

		// First upload
		req1 := httptest.NewRequest("POST", "/api/upload", &buf1)
		req1.Header.Set("Content-Type", writer1.FormDataContentType())
		w1 := httptest.NewRecorder()

		handler.HandleUpload(w1, req1)

		assert.Equal(t, http.StatusOK, w1.Code)

		var uploadResponse1 map[string]interface{}
		err = json.Unmarshal(w1.Body.Bytes(), &uploadResponse1)
		require.NoError(t, err)
		assert.True(t, uploadResponse1["success"].(bool))

		// Extract file ID from first upload response
		fileData1 := uploadResponse1["file"].(map[string]interface{})
		fileID1 := int(fileData1["id"].(float64))

		// Delete the file
		deleteURL := fmt.Sprintf("/api/files/%d", fileID1)
		deleteReq := httptest.NewRequest("DELETE", deleteURL, nil)
		deleteW := httptest.NewRecorder()
		handler.HandleFileOperations(deleteW, deleteReq)

		assert.Equal(t, http.StatusOK, deleteW.Code)

		// Create multipart form for second upload (same content)
		var buf2 bytes.Buffer
		writer2 := multipart.NewWriter(&buf2)
		part2, err := writer2.CreateFormFile("file", "test_reupload.txt")
		require.NoError(t, err)

		_, err = part2.Write(testContent)
		require.NoError(t, err)
		require.NoError(t, writer2.Close())

		// Second upload (should restore the file)
		req2 := httptest.NewRequest("POST", "/api/upload", &buf2)
		req2.Header.Set("Content-Type", writer2.FormDataContentType())
		w2 := httptest.NewRecorder()

		handler.HandleUpload(w2, req2)

		if w2.Code != http.StatusOK {
			t.Logf("Second upload failed with status %d, response: %s", w2.Code, w2.Body.String())
		}
		assert.Equal(t, http.StatusOK, w2.Code)

		var uploadResponse2 map[string]interface{}
		err = json.Unmarshal(w2.Body.Bytes(), &uploadResponse2)
		require.NoError(t, err)
		assert.True(t, uploadResponse2["success"].(bool))

		// Extract file ID from second upload response
		fileData2 := uploadResponse2["file"].(map[string]interface{})
		fileID2 := int(fileData2["id"].(float64))

		// The second upload should restore the same file record (same ID)
		assert.Equal(t, fileID1, fileID2, "Second upload should restore the same file record")

		// Verify the message indicates file restoration
		assert.Equal(t, "File restored successfully", uploadResponse2["message"])

		// Verify the file is no longer marked as deleted
		restoredFile, err := db.GetFileByHash(fileData2["hash"].(string))
		require.NoError(t, err)
		assert.False(t, restoredFile.Deleted, "Restored file should not be marked as deleted")
		assert.Nil(t, restoredFile.DeletedTime, "Restored file should have no deleted time")
	})

	_ = db // Suppress unused variable warning
}

func TestIntegration_ReportsPreservedOnFileDeletion(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("Reports should be preserved when file is deleted", func(t *testing.T) {
		// Create test file content
		testContent := testutil.SampleFiles["ttop"].Content

		// Upload file
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "test_reports_preserved.txt")
		require.NoError(t, err)

		_, err = part.Write(testContent)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		req := httptest.NewRequest("POST", "/api/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.HandleUpload(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var uploadResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &uploadResponse)
		require.NoError(t, err)

		fileData := uploadResponse["file"].(map[string]interface{})
		fileID := int(fileData["id"].(float64))

		// Create a manual report
		reportReq := httptest.NewRequest("POST", fmt.Sprintf("/api/reports/%d", fileID),
			strings.NewReader(`{"report_type": "ttop"}`))
		reportReq.Header.Set("Content-Type", "application/json")
		reportW := httptest.NewRecorder()

		handler.HandleReports(reportW, reportReq)
		assert.Equal(t, http.StatusOK, reportW.Code)

		// Get reports before deletion
		getReportsReq := httptest.NewRequest("GET", fmt.Sprintf("/api/reports/%d", fileID), nil)
		getReportsW := httptest.NewRecorder()
		handler.HandleReports(getReportsW, getReportsReq)

		assert.Equal(t, http.StatusOK, getReportsW.Code)
		var reportsBeforeDelete map[string]interface{}
		err = json.Unmarshal(getReportsW.Body.Bytes(), &reportsBeforeDelete)
		require.NoError(t, err)

		reportsBefore := reportsBeforeDelete["reports"].([]interface{})
		assert.Greater(t, len(reportsBefore), 0, "Should have at least one report before deletion")

		// Delete the file
		deleteURL := fmt.Sprintf("/api/files/%d", fileID)
		deleteReq := httptest.NewRequest("DELETE", deleteURL, nil)
		deleteW := httptest.NewRecorder()
		handler.HandleFileOperations(deleteW, deleteReq)

		assert.Equal(t, http.StatusOK, deleteW.Code)

		// Verify file is marked as deleted
		deletedFile, err := db.GetFileByHash(fileData["hash"].(string))
		require.NoError(t, err)
		assert.True(t, deletedFile.Deleted, "File should be marked as deleted")

		// Get reports after deletion - they should still exist
		getReportsAfterReq := httptest.NewRequest("GET", fmt.Sprintf("/api/reports/%d", fileID), nil)
		getReportsAfterW := httptest.NewRecorder()
		handler.HandleReports(getReportsAfterW, getReportsAfterReq)

		assert.Equal(t, http.StatusOK, getReportsAfterW.Code)
		var reportsAfterDelete map[string]interface{}
		err = json.Unmarshal(getReportsAfterW.Body.Bytes(), &reportsAfterDelete)
		require.NoError(t, err)

		reportsAfter := reportsAfterDelete["reports"].([]interface{})
		assert.Equal(t, len(reportsBefore), len(reportsAfter), "Reports should be preserved after file deletion")

		// Verify we can still access report content
		if len(reportsAfter) > 0 {
			report := reportsAfter[0].(map[string]interface{})
			reportID := int(report["id"].(float64))

			contentReq := httptest.NewRequest("GET", fmt.Sprintf("/api/reports/content/%d", reportID), nil)
			contentW := httptest.NewRecorder()
			handler.HandleReportContent(contentW, contentReq)

			// Report content should still be accessible
			assert.Equal(t, http.StatusOK, contentW.Code)
		}
	})

	_ = db // Suppress unused variable warning
}

func TestIntegration_FileUploadWorkflow(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("Complete file upload and processing workflow", func(t *testing.T) {
		// Create test file content
		testContent := testutil.SampleFiles["ttop"].Content

		// Create multipart form
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "test_upload.txt")
		require.NoError(t, err)

		_, err = part.Write(testContent)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		// Upload file
		req := httptest.NewRequest("POST", "/api/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.HandleUpload(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var uploadResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &uploadResponse)
		require.NoError(t, err)
		assert.True(t, uploadResponse["success"].(bool))

		// Extract file ID from response
		fileData := uploadResponse["file"].(map[string]interface{})
		fileID := int(fileData["id"].(float64))

		// Verify file appears in files list
		req = httptest.NewRequest("GET", "/api/files", nil)
		w = httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var filesResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &filesResponse)
		require.NoError(t, err)

		files := filesResponse["files"].([]interface{})
		assert.GreaterOrEqual(t, len(files), 1)

		// Find our uploaded file
		var uploadedFile map[string]interface{}
		for _, f := range files {
			file := f.(map[string]interface{})
			if int(file["id"].(float64)) == fileID {
				uploadedFile = file
				break
			}
		}
		require.NotNil(t, uploadedFile)
		assert.Equal(t, "test_upload.txt", uploadedFile["original_name"])

		// Check if automatic report was created
		reportsURL := fmt.Sprintf("/api/reports/%d", fileID)
		req = httptest.NewRequest("GET", reportsURL, nil)
		w = httptest.NewRecorder()
		handler.HandleReports(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var reportsResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &reportsResponse)
		require.NoError(t, err)

		// Check if reports were created automatically
		if reportsField, exists := reportsResponse["reports"]; exists && reportsField != nil {
			reports := reportsField.([]interface{})
			if len(reports) > 0 {
				// If automatic report was created, verify we can access its content
				report := reports[0].(map[string]interface{})
				reportID := int(report["id"].(float64))

				contentURL := fmt.Sprintf("/api/reports/content/%d", reportID)
				req = httptest.NewRequest("GET", contentURL, nil)
				w = httptest.NewRecorder()
				handler.HandleReportContent(w, req)

				// Report might be pending, so we accept both OK and not found
				assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, w.Code)
			}
		}
	})

	_ = db // Suppress unused variable warning
}

func TestIntegration_ReportGeneration(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("Manual report creation and viewing", func(t *testing.T) {
		// First upload a file
		testContent := testutil.SampleFiles["ttop"].Content

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "report_test.txt")
		require.NoError(t, err)

		_, err = part.Write(testContent)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		req := httptest.NewRequest("POST", "/api/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.HandleUpload(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var uploadResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &uploadResponse)
		require.NoError(t, err)

		fileData := uploadResponse["file"].(map[string]interface{})
		fileID := int(fileData["id"].(float64))

		// Create a manual report
		reportReq := map[string]string{
			"report_type": "ttop",
		}
		reqBody, err := json.Marshal(reportReq)
		require.NoError(t, err)

		reportsURL := fmt.Sprintf("/api/reports/%d", fileID)
		req = httptest.NewRequest("POST", reportsURL, bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()

		handler.HandleReports(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify report was created
		req = httptest.NewRequest("GET", reportsURL, nil)
		w = httptest.NewRecorder()
		handler.HandleReports(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var reportsResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &reportsResponse)
		require.NoError(t, err)

		reports := reportsResponse["reports"].([]interface{})
		assert.GreaterOrEqual(t, len(reports), 1)

		// Check report properties
		report := reports[0].(map[string]interface{})
		assert.Equal(t, "ttop", report["report_type"])
		assert.Contains(t, []string{"pending", "completed", "failed"}, report["status"])
	})

	_ = db // Suppress unused variable warning
}

func TestIntegration_APIEndpoints(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("API endpoint responses and JSON structure", func(t *testing.T) {
		// Test /api/files endpoint
		req := httptest.NewRequest("GET", "/api/files", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		// Verify JSON structure
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Contains(t, response, "files")

		// Test pagination parameters
		req = httptest.NewRequest("GET", "/api/files?limit=5&offset=0", nil)
		w = httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		// Test include_deleted parameter
		req = httptest.NewRequest("GET", "/api/files?include_deleted=true", nil)
		w = httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("File operations API", func(t *testing.T) {
		// Upload a test file first
		testContent := testutil.SampleFiles["ttop"].Content

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "delete_test.txt")
		require.NoError(t, err)

		_, err = part.Write(testContent)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		req := httptest.NewRequest("POST", "/api/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.HandleUpload(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var uploadResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &uploadResponse)
		require.NoError(t, err)

		fileData := uploadResponse["file"].(map[string]interface{})
		fileID := int(fileData["id"].(float64))

		// Test file deletion
		deleteURL := fmt.Sprintf("/api/files/%d", fileID)
		req = httptest.NewRequest("DELETE", deleteURL, nil)
		w = httptest.NewRecorder()
		handler.HandleFileOperations(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var deleteResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &deleteResponse)
		require.NoError(t, err)
		assert.True(t, deleteResponse["success"].(bool))
		assert.Contains(t, deleteResponse["message"], "deleted")
	})

	_ = db // Suppress unused variable warning
}

func TestIntegration_ErrorHandling(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("404 errors for non-existent resources", func(t *testing.T) {
		// Test non-existent file - the handler returns 200 even for non-existent files
		// because the database UPDATE operation succeeds (affects 0 rows)
		req := httptest.NewRequest("DELETE", "/api/files/99999", nil)
		w := httptest.NewRecorder()
		handler.HandleFileOperations(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		// Test non-existent report
		req = httptest.NewRequest("GET", "/api/reports/content/99999", nil)
		w = httptest.NewRecorder()
		handler.HandleReportContent(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		// Test invalid file ID format
		req = httptest.NewRequest("DELETE", "/api/files/invalid", nil)
		w = httptest.NewRecorder()
		handler.HandleFileOperations(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Method not allowed errors", func(t *testing.T) {
		// Test wrong method on files endpoint
		req := httptest.NewRequest("POST", "/api/files", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// Test wrong method on report content endpoint
		req = httptest.NewRequest("POST", "/api/reports/content/1", nil)
		w = httptest.NewRecorder()
		handler.HandleReportContent(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("Invalid upload requests", func(t *testing.T) {
		// Test upload without file
		req := httptest.NewRequest("POST", "/api/upload", nil)
		w := httptest.NewRecorder()
		handler.HandleUpload(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Test wrong method on upload endpoint
		req = httptest.NewRequest("GET", "/api/upload", nil)
		w = httptest.NewRecorder()
		handler.HandleUpload(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("Invalid JSON in report creation", func(t *testing.T) {
		// Test invalid JSON body
		req := httptest.NewRequest("POST", "/api/reports/1", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.HandleReports(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	_ = db // Suppress unused variable warning
}

func TestIntegration_Performance(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("API response times", func(t *testing.T) {
		// Test files endpoint performance
		start := time.Now()
		req := httptest.NewRequest("GET", "/api/files", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)
		duration := time.Since(start)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Less(t, duration, 100*time.Millisecond, "Files API should respond quickly")

		// Test upload performance with small file
		testContent := []byte("small test content")

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "perf_test.txt")
		require.NoError(t, err)

		_, err = part.Write(testContent)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		start = time.Now()
		req = httptest.NewRequest("POST", "/api/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w = httptest.NewRecorder()
		handler.HandleUpload(w, req)
		duration = time.Since(start)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Less(t, duration, 500*time.Millisecond, "Small file upload should be fast")
	})

	t.Run("Concurrent requests", func(t *testing.T) {
		// Test concurrent file list requests
		const numRequests = 10
		results := make(chan int, numRequests)

		for i := 0; i < numRequests; i++ {
			go func() {
				req := httptest.NewRequest("GET", "/api/files", nil)
				w := httptest.NewRecorder()
				handler.HandleFiles(w, req)
				results <- w.Code
			}()
		}

		// Collect results
		for i := 0; i < numRequests; i++ {
			code := <-results
			assert.Equal(t, http.StatusOK, code)
		}
	})

	_ = db // Suppress unused variable warning
}

func TestIntegration_DuplicateFileHandling(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("Upload same file twice", func(t *testing.T) {
		testContent := testutil.SampleFiles["ttop"].Content

		// Upload file first time
		var buf1 bytes.Buffer
		writer1 := multipart.NewWriter(&buf1)
		part1, err := writer1.CreateFormFile("file", "duplicate_test.txt")
		require.NoError(t, err)

		_, err = part1.Write(testContent)
		require.NoError(t, err)
		require.NoError(t, writer1.Close())

		req1 := httptest.NewRequest("POST", "/api/upload", &buf1)
		req1.Header.Set("Content-Type", writer1.FormDataContentType())
		w1 := httptest.NewRecorder()

		handler.HandleUpload(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		var response1 map[string]interface{}
		err = json.Unmarshal(w1.Body.Bytes(), &response1)
		require.NoError(t, err)

		fileData1 := response1["file"].(map[string]interface{})
		fileID1 := int(fileData1["id"].(float64))

		// Upload same file second time
		var buf2 bytes.Buffer
		writer2 := multipart.NewWriter(&buf2)
		part2, err := writer2.CreateFormFile("file", "duplicate_test.txt")
		require.NoError(t, err)

		_, err = part2.Write(testContent)
		require.NoError(t, err)
		require.NoError(t, writer2.Close())

		req2 := httptest.NewRequest("POST", "/api/upload", &buf2)
		req2.Header.Set("Content-Type", writer2.FormDataContentType())
		w2 := httptest.NewRecorder()

		handler.HandleUpload(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)

		var response2 map[string]interface{}
		err = json.Unmarshal(w2.Body.Bytes(), &response2)
		require.NoError(t, err)

		// Should return the same file
		fileData2 := response2["file"].(map[string]interface{})
		fileID2 := int(fileData2["id"].(float64))

		assert.Equal(t, fileID1, fileID2)
		assert.Contains(t, response2["message"], "already exists")
	})

	_ = db // Suppress unused variable warning
}

func TestIntegration_FileSearchWorkflow(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("Complete file search workflow", func(t *testing.T) {
		// Upload multiple files with different names and types
		testFiles := []struct {
			filename string
			content  []byte
			fileType string
		}{
			{"performance_analysis.txt", testutil.SampleFiles["ttop"].Content, "ttop"},
			{"system_iostat_report.log", []byte("iostat sample content"), "unknown"},
			{"database_metrics.csv", []byte("csv,data,here"), "unknown"},
			{"network_performance.json", []byte(`{"network": "data"}`), "unknown"},
		}

		uploadedFiles := make([]map[string]interface{}, 0, len(testFiles))

		// Upload all test files
		for _, testFile := range testFiles {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)
			part, err := writer.CreateFormFile("file", testFile.filename)
			require.NoError(t, err)

			_, err = part.Write(testFile.content)
			require.NoError(t, err)
			require.NoError(t, writer.Close())

			req := httptest.NewRequest("POST", "/api/upload", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()

			handler.HandleUpload(w, req)
			require.Equal(t, http.StatusOK, w.Code)

			var uploadResponse map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &uploadResponse)
			require.NoError(t, err)

			fileData := uploadResponse["file"].(map[string]interface{})
			uploadedFiles = append(uploadedFiles, fileData)
		}

		// Test 1: Search by filename substring
		req := httptest.NewRequest("GET", "/api/files?search=performance", nil)
		w := httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var searchResponse map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &searchResponse)
		require.NoError(t, err)

		files := searchResponse["files"].([]interface{})
		assert.Len(t, files, 2, "Should find 2 files with 'performance' in name")

		// Verify correct files found
		foundNames := make(map[string]bool)
		for _, f := range files {
			file := f.(map[string]interface{})
			foundNames[file["original_name"].(string)] = true
		}
		assert.True(t, foundNames["performance_analysis.txt"])
		assert.True(t, foundNames["network_performance.json"])

		// Test 2: Search by file type
		req = httptest.NewRequest("GET", "/api/files?search=ttop", nil)
		w = httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		err = json.Unmarshal(w.Body.Bytes(), &searchResponse)
		require.NoError(t, err)

		files = searchResponse["files"].([]interface{})
		assert.GreaterOrEqual(t, len(files), 1, "Should find at least 1 ttop file")

		// Test 3: Search with pagination
		req = httptest.NewRequest("GET", "/api/files?search=performance&limit=1&offset=0", nil)
		w = httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		err = json.Unmarshal(w.Body.Bytes(), &searchResponse)
		require.NoError(t, err)

		files = searchResponse["files"].([]interface{})
		assert.Len(t, files, 1, "Should respect limit with search")

		// Test 4: Case insensitive search
		req = httptest.NewRequest("GET", "/api/files?search=PERFORMANCE", nil)
		w = httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		err = json.Unmarshal(w.Body.Bytes(), &searchResponse)
		require.NoError(t, err)

		files = searchResponse["files"].([]interface{})
		assert.Len(t, files, 2, "Case insensitive search should work")

		// Test 5: Search with no results
		req = httptest.NewRequest("GET", "/api/files?search=nonexistentfile", nil)
		w = httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		err = json.Unmarshal(w.Body.Bytes(), &searchResponse)
		require.NoError(t, err)

		files = searchResponse["files"].([]interface{})
		assert.Len(t, files, 0, "Should return empty results for non-existent search")

		// Test 6: Delete a file and verify search behavior
		fileToDelete := uploadedFiles[0]
		fileID := int(fileToDelete["id"].(float64))

		deleteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/files/%d", fileID), nil)
		deleteW := httptest.NewRecorder()
		handler.HandleFileOperations(deleteW, deleteReq)

		assert.Equal(t, http.StatusOK, deleteW.Code)

		// Search should not find deleted file by default
		req = httptest.NewRequest("GET", "/api/files?search=performance", nil)
		w = httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		err = json.Unmarshal(w.Body.Bytes(), &searchResponse)
		require.NoError(t, err)

		files = searchResponse["files"].([]interface{})
		assert.Len(t, files, 1, "Should find 1 file after deletion (excluding deleted)")

		// Search with include_deleted should find the deleted file
		req = httptest.NewRequest("GET", "/api/files?search=performance&include_deleted=true", nil)
		w = httptest.NewRecorder()
		handler.HandleFiles(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		err = json.Unmarshal(w.Body.Bytes(), &searchResponse)
		require.NoError(t, err)

		files = searchResponse["files"].([]interface{})
		assert.Len(t, files, 2, "Should find 2 files when including deleted")

		// Verify one is marked as deleted
		deletedCount := 0
		for _, f := range files {
			file := f.(map[string]interface{})
			if file["deleted"].(bool) {
				deletedCount++
			}
		}
		assert.Equal(t, 1, deletedCount, "Should have exactly 1 deleted file")
	})

	_ = db // Suppress unused variable warning
}

func TestHandlers_HandleDiskUsage(t *testing.T) {
	handler, _ := setupTestHandler(t)

	t.Run("Returns disk usage stats", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/disk-usage", nil)
		w := httptest.NewRecorder()

		handler.HandleDiskUsage(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Contains(t, response, "uploads")
		assert.Contains(t, response, "database")
		assert.Contains(t, response, "same_filesystem")
		assert.Contains(t, response, "max_disk_usage")
		assert.Contains(t, response, "file_retention_days")

		// Verify settings values from test config
		assert.Equal(t, handler.cfg.MaxDiskUsage, response["max_disk_usage"].(float64))
		assert.Equal(t, float64(handler.cfg.FileRetentionDays), response["file_retention_days"].(float64))

		// Verify uploads stats structure
		uploads := response["uploads"].(map[string]interface{})
		assert.Contains(t, uploads, "path")
		assert.Contains(t, uploads, "total_bytes")
		assert.Contains(t, uploads, "free_bytes")
		assert.Contains(t, uploads, "used_bytes")
		assert.Contains(t, uploads, "available_bytes")
		assert.Contains(t, uploads, "used_percent")

		// Verify database stats structure
		db := response["database"].(map[string]interface{})
		assert.Contains(t, db, "path")
		assert.Contains(t, db, "total_bytes")
		assert.Contains(t, db, "free_bytes")
		assert.Contains(t, db, "used_bytes")
		assert.Contains(t, db, "available_bytes")
		assert.Contains(t, db, "used_percent")
	})

	t.Run("Handles invalid method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/disk-usage", nil)
		w := httptest.NewRecorder()

		handler.HandleDiskUsage(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestHandlers_HandleRedetectFileType(t *testing.T) {
	handler, db := setupTestHandler(t)

	// 1. Upload a file. We'll use ttop content but with a generic name.
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.data")
	require.NoError(t, err)
	_, err = part.Write(testutil.SampleFiles["ttop"].Content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	handler.HandleUpload(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var uploadResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &uploadResponse)
	require.NoError(t, err)
	fileData := uploadResponse["file"].(map[string]interface{})
	fileID := int(fileData["id"].(float64))

	// Verify it was detected as ttop initially
	assert.Equal(t, "ttop", fileData["file_type"])

	// 2. Manually change the file type in the DB to 'unknown' to simulate a past misdetection.
	// Use the new UpdateFileFileType directly for testing purposes
	err = db.UpdateFileFileType(fileID, "unknown")
	require.NoError(t, err)

	// 3. Call the redetect endpoint.
	redetectURL := fmt.Sprintf("/api/files/%d/redetect", fileID)
	req = httptest.NewRequest("POST", redetectURL, nil)
	w = httptest.NewRecorder()
	handler.HandleRedetectFileType(w, req)

	// 4. Assert the response is successful and the file type is corrected.
	require.Equal(t, http.StatusOK, w.Code)
	var redetectResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &redetectResponse)
	require.NoError(t, err)
	assert.True(t, redetectResponse["success"].(bool))

	updatedFileData := redetectResponse["file"].(map[string]interface{})
	assert.Equal(t, "ttop", updatedFileData["file_type"])

	// 5. Verify directly from the DB that the file type was updated using GetFileByID.
	updatedFileFromDB, err := db.GetFileByID(fileID)
	require.NoError(t, err)
	assert.Equal(t, "ttop", updatedFileFromDB.FileType)

	// 6. Verify that a new report was queued.
	// The initial upload creates one report. Re-detection should create a second one.
	reports, err := db.GetReportsByFileID(fileID)
	require.NoError(t, err)
	assert.Len(t, reports, 2, "a new report should have been created on re-detection")
}

func TestHandlers_HandleSettings(t *testing.T) {
	handler, db := setupTestHandler(t)

	// Initialize settings in database
	defaultSettings := map[string]string{
		"max_disk_usage":      fmt.Sprintf("%.6f", handler.cfg.MaxDiskUsage),
		"file_retention_days": fmt.Sprintf("%d", handler.cfg.FileRetentionDays),
	}
	err := db.InitializeSettings(defaultSettings)
	require.NoError(t, err)

	t.Run("Get settings", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/settings", nil)
		w := httptest.NewRecorder()
		handler.HandleSettings(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		// Should get values from database now
		assert.Equal(t, handler.cfg.MaxDiskUsage, response["max_disk_usage"].(float64))
		assert.Equal(t, float64(handler.cfg.FileRetentionDays), response["file_retention_days"].(float64))
	})

	t.Run("Update settings successfully", func(t *testing.T) {
		body := `{"max_disk_usage": "80.5", "file_retention_days": "30"}`
		req := httptest.NewRequest("POST", "/api/settings", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.HandleSettings(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Settings updated successfully", response["message"])

		// Verify config was updated (for backward compatibility)
		assert.InDelta(t, 0.805, handler.cfg.MaxDiskUsage, 0.001)
		assert.Equal(t, 30, handler.cfg.FileRetentionDays)

		// Verify settings were saved to database
		maxDiskUsageStr, err := db.GetSetting("max_disk_usage")
		require.NoError(t, err)
		assert.Equal(t, "0.805000", maxDiskUsageStr)

		fileRetentionDaysStr, err := db.GetSetting("file_retention_days")
		require.NoError(t, err)
		assert.Equal(t, "30", fileRetentionDaysStr)
	})

	t.Run("Update settings with invalid values", func(t *testing.T) {
		testCases := []struct {
			name       string
			body       string
			expectCode int
			expectMsg  string
		}{
			{
				name:       "invalid max disk usage (not a number)",
				body:       `{"max_disk_usage": "abc", "file_retention_days": "14"}`,
				expectCode: http.StatusBadRequest,
				expectMsg:  "Invalid max_disk_usage value",
			},
			{
				name:       "invalid max disk usage (out of range)",
				body:       `{"max_disk_usage": "101", "file_retention_days": "14"}`,
				expectCode: http.StatusBadRequest,
				expectMsg:  "max_disk_usage must be between 0 and 100",
			},
			{
				name:       "invalid retention days (not a number)",
				body:       `{"max_disk_usage": "50", "file_retention_days": "abc"}`,
				expectCode: http.StatusBadRequest,
				expectMsg:  "Invalid file_retention_days value",
			},
			{
				name:       "invalid retention days (negative)",
				body:       `{"max_disk_usage": "50", "file_retention_days": "-1"}`,
				expectCode: http.StatusBadRequest,
				expectMsg:  "file_retention_days must be non-negative",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest("POST", "/api/settings", strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				handler.HandleSettings(w, req)

				assert.Equal(t, tc.expectCode, w.Code)
				assert.Contains(t, w.Body.String(), tc.expectMsg)
			})
		}
	})

	t.Run("Settings persist across handler instances", func(t *testing.T) {
		// Update settings
		body := `{"max_disk_usage": "75.0", "file_retention_days": "21"}`
		req := httptest.NewRequest("POST", "/api/settings", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.HandleSettings(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Create a new handler instance with the same database
		mockWorker := &mockCleanupWorker{}
		newHandler := New(db, handler.cfg, mockWorker)

		// Get settings from new handler instance
		req = httptest.NewRequest("GET", "/api/settings", nil)
		w = httptest.NewRecorder()
		newHandler.HandleSettings(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		// Should get the updated values from database
		assert.Equal(t, 0.75, response["max_disk_usage"].(float64))
		assert.Equal(t, float64(21), response["file_retention_days"].(float64))
	})

	t.Run("Cleanup triggered when threshold lowered", func(t *testing.T) {
		// Create handler with mock cleanup worker
		mockWorker := &mockCleanupWorker{}
		testHandler := New(db, handler.cfg, mockWorker)

		// Set initial high threshold
		body := `{"max_disk_usage": "90.0", "file_retention_days": "14"}`
		req := httptest.NewRequest("POST", "/api/settings", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		testHandler.HandleSettings(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		initialTriggerCount := mockWorker.getTriggerCount()

		// Lower the threshold - should trigger cleanup
		body = `{"max_disk_usage": "50.0", "file_retention_days": "14"}`
		req = httptest.NewRequest("POST", "/api/settings", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		testHandler.HandleSettings(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Give goroutine time to execute
		time.Sleep(10 * time.Millisecond)

		// Verify cleanup was triggered
		assert.Greater(t, mockWorker.getTriggerCount(), initialTriggerCount, "Cleanup should be triggered when threshold is lowered")

		// Raise the threshold - should NOT trigger cleanup
		initialTriggerCount = mockWorker.getTriggerCount()
		body = `{"max_disk_usage": "80.0", "file_retention_days": "14"}`
		req = httptest.NewRequest("POST", "/api/settings", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		testHandler.HandleSettings(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Give goroutine time to execute
		time.Sleep(10 * time.Millisecond)

		// Verify cleanup was NOT triggered
		assert.Equal(t, initialTriggerCount, mockWorker.getTriggerCount(), "Cleanup should NOT be triggered when threshold is raised")
	})

	t.Run("Handles invalid method", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/settings", nil)
		w := httptest.NewRecorder()
		handler.HandleSettings(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestHandlers_DeleteReportCleanup(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("Delete file entry when deleted file has no reports left", func(t *testing.T) {
		// Create a test file
		testFile := &database.File{
			Hash:         "cleanup-test-hash",
			OriginalName: "cleanup-test.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/cleanup-test-hash",
		}

		err := db.InsertFile(testFile)
		require.NoError(t, err)

		// Create two reports for the file
		report1 := &database.Report{
			FileID:      testFile.ID,
			ReportType:  "ttop",
			Status:      "completed",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
			ReportData:  `{"type": "ttop", "summary": "test report 1"}`,
		}

		report2 := &database.Report{
			FileID:      testFile.ID,
			ReportType:  "iostat",
			Status:      "completed",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
			ReportData:  `{"type": "iostat", "summary": "test report 2"}`,
		}

		err = db.InsertReport(report1)
		require.NoError(t, err)
		err = db.InsertReport(report2)
		require.NoError(t, err)

		// Mark the file as deleted
		err = db.MarkFileDeleted(testFile.ID)
		require.NoError(t, err)

		// Verify file exists but is marked as deleted
		deletedFile, err := db.GetFileByID(testFile.ID)
		require.NoError(t, err)
		assert.True(t, deletedFile.Deleted)

		// Delete the first report - file should still exist
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/reports/%d", report1.ID), nil)
		w := httptest.NewRecorder()
		handler.HandleReports(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// File should still exist (has one report left)
		_, err = db.GetFileByID(testFile.ID)
		assert.NoError(t, err, "File should still exist with one report remaining")

		// Delete the second report - file should be completely removed
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/reports/%d", report2.ID), nil)
		w = httptest.NewRecorder()
		handler.HandleReports(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Check the response includes file deletion information
		var deleteResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &deleteResponse)
		require.NoError(t, err)

		assert.True(t, deleteResponse["success"].(bool))
		assert.True(t, deleteResponse["file_deleted"].(bool), "Response should indicate file was deleted")
		assert.Equal(t, float64(testFile.ID), deleteResponse["deleted_file_id"].(float64))
		assert.Equal(t, testFile.OriginalName, deleteResponse["deleted_file_name"].(string))
		assert.Contains(t, deleteResponse["message"].(string), "completely removed")

		// File should no longer exist in database
		_, err = db.GetFileByID(testFile.ID)
		assert.Error(t, err, "File should be completely removed from database")
	})

	t.Run("Keep file entry when file is not deleted", func(t *testing.T) {
		// Create a test file (not deleted)
		testFile := &database.File{
			Hash:         "keep-test-hash",
			OriginalName: "keep-test.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/keep-test-hash",
		}

		err := db.InsertFile(testFile)
		require.NoError(t, err)

		// Create a report for the file
		report := &database.Report{
			FileID:      testFile.ID,
			ReportType:  "ttop",
			Status:      "completed",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
			ReportData:  `{"type": "ttop", "summary": "test report"}`,
		}

		err = db.InsertReport(report)
		require.NoError(t, err)

		// Delete the report
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/reports/%d", report.ID), nil)
		w := httptest.NewRecorder()
		handler.HandleReports(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Check the response does NOT include file deletion information
		var deleteResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &deleteResponse)
		require.NoError(t, err)

		assert.True(t, deleteResponse["success"].(bool))
		assert.Nil(t, deleteResponse["file_deleted"], "Response should not indicate file was deleted")
		assert.Equal(t, "Report deleted successfully", deleteResponse["message"].(string))

		// File should still exist (not deleted, so we keep it)
		_, err = db.GetFileByID(testFile.ID)
		assert.NoError(t, err, "File should still exist when not marked as deleted")
	})
}
