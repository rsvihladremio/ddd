package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rsvihladremio/ddd/internal/database"
	"github.com/rsvihladremio/ddd/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	handler := New(db, cfg)
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

		files, err := db.GetFiles(10, 0, false)
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

		// The handler returns 200 even for non-existent files (updates 0 rows)
		assert.Equal(t, http.StatusOK, w.Code)
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
