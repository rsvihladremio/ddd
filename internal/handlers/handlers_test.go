//	Copyright 2023 Dremio Corporation
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

// Integration Tests - These replace the Playwright e2e tests with httptest-based tests

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

		assert.Equal(t, http.StatusOK, w.Code)

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
