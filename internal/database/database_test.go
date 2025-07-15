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

package database

import (
	"testing"
	"time"

	"github.com/rsvihladremio/ddd/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDB creates a test database with clean schema
func testDB(t *testing.T) *DB {
	t.Helper()

	cfg := testutil.TestConfig(t)
	db, err := Initialize(cfg.DBPath)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Error closing database: %v", err)
		}
	})

	return db
}

func TestDatabase_Initialize(t *testing.T) {
	cfg := testutil.TestConfig(t)

	db, err := Initialize(cfg.DBPath)
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("Error closing database: %v", err)
		}
	}()

	// Test that tables were created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('files', 'reports', 'worker_status')").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count, "All tables should be created")
}

func TestDatabase_FileOperations(t *testing.T) {
	db := testDB(t)

	t.Run("InsertFile", func(t *testing.T) {
		file := &File{
			Hash:         "test-hash-123",
			OriginalName: "test.txt",
			FileType:     "ttop",
			FileSize:     1024,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/test-hash-123",
		}

		err := db.InsertFile(file)
		require.NoError(t, err)
		assert.NotZero(t, file.ID, "File ID should be set after insert")
	})

	t.Run("GetFileByHash", func(t *testing.T) {
		// Insert a test file
		originalFile := &File{
			Hash:         "test-hash-456",
			OriginalName: "test2.txt",
			FileType:     "iostat",
			FileSize:     2048,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/test-hash-456",
		}

		err := db.InsertFile(originalFile)
		require.NoError(t, err)

		// Retrieve the file
		retrievedFile, err := db.GetFileByHash("test-hash-456")
		require.NoError(t, err)

		assert.Equal(t, originalFile.Hash, retrievedFile.Hash)
		assert.Equal(t, originalFile.OriginalName, retrievedFile.OriginalName)
		assert.Equal(t, originalFile.FileType, retrievedFile.FileType)
		assert.Equal(t, originalFile.FileSize, retrievedFile.FileSize)
		assert.Equal(t, originalFile.FilePath, retrievedFile.FilePath)
	})

	t.Run("GetFileByHash_NotFound", func(t *testing.T) {
		_, err := db.GetFileByHash("non-existent-hash")
		require.Error(t, err)
	})

	t.Run("GetFiles", func(t *testing.T) {
		// Insert multiple test files
		files := []*File{
			{
				Hash:         "hash-1",
				OriginalName: "file1.txt",
				FileType:     "ttop",
				FileSize:     100,
				UploadTime:   time.Now().Add(-2 * time.Hour),
				FilePath:     "/uploads/hash-1",
			},
			{
				Hash:         "hash-2",
				OriginalName: "file2.txt",
				FileType:     "iostat",
				FileSize:     200,
				UploadTime:   time.Now().Add(-1 * time.Hour),
				FilePath:     "/uploads/hash-2",
			},
		}

		for _, file := range files {
			err := db.InsertFile(file)
			require.NoError(t, err)
		}

		// Test pagination
		retrievedFiles, err := db.GetFiles(10, 0, false)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(retrievedFiles), 2, "Should retrieve at least 2 files")

		// Files should be ordered by upload_time DESC
		if len(retrievedFiles) >= 2 {
			assert.True(t, retrievedFiles[0].UploadTime.After(retrievedFiles[1].UploadTime) ||
				retrievedFiles[0].UploadTime.Equal(retrievedFiles[1].UploadTime))
		}
	})

	t.Run("MarkFileDeleted", func(t *testing.T) {
		// Insert a test file
		file := &File{
			Hash:         "delete-test-hash",
			OriginalName: "delete-test.txt",
			FileType:     "ttop",
			FileSize:     512,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/delete-test-hash",
		}

		err := db.InsertFile(file)
		require.NoError(t, err)

		// Mark as deleted
		err = db.MarkFileDeleted(file.ID)
		require.NoError(t, err)

		// Verify it's marked as deleted
		retrievedFile, err := db.GetFileByHash("delete-test-hash")
		require.NoError(t, err)
		assert.True(t, retrievedFile.Deleted)
		assert.NotNil(t, retrievedFile.DeletedTime)
	})

	t.Run("GetFilesOlderThan", func(t *testing.T) {
		cutoffTime := time.Now().Add(-1 * time.Hour)

		// Insert an old file
		oldFile := &File{
			Hash:         "old-file-hash",
			OriginalName: "old-file.txt",
			FileType:     "ttop",
			FileSize:     256,
			UploadTime:   time.Now().Add(-2 * time.Hour),
			FilePath:     "/uploads/old-file-hash",
		}

		err := db.InsertFile(oldFile)
		require.NoError(t, err)

		// Insert a new file
		newFile := &File{
			Hash:         "new-file-hash",
			OriginalName: "new-file.txt",
			FileType:     "ttop",
			FileSize:     256,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/new-file-hash",
		}

		err = db.InsertFile(newFile)
		require.NoError(t, err)

		// Get files older than cutoff
		oldFiles, err := db.GetFilesOlderThan(cutoffTime)
		require.NoError(t, err)

		// Should contain the old file but not the new one
		found := false
		for _, file := range oldFiles {
			if file.Hash == "old-file-hash" {
				found = true
			}
			assert.True(t, file.UploadTime.Before(cutoffTime))
		}
		assert.True(t, found, "Should find the old file")
	})
}

func TestDatabase_ReportOperations(t *testing.T) {
	db := testDB(t)

	// First create a file to associate reports with
	file := &File{
		Hash:         "report-test-hash",
		OriginalName: "report-test.txt",
		FileType:     "ttop",
		FileSize:     1024,
		UploadTime:   time.Now(),
		FilePath:     "/uploads/report-test-hash",
	}

	err := db.InsertFile(file)
	require.NoError(t, err)

	t.Run("InsertReport", func(t *testing.T) {
		report := &Report{
			FileID:      file.ID,
			ReportType:  "ttop",
			Status:      "pending",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}

		err := db.InsertReport(report)
		require.NoError(t, err)
		assert.NotZero(t, report.ID, "Report ID should be set after insert")
	})

	t.Run("GetPendingReports", func(t *testing.T) {
		// Insert a pending report
		pendingReport := &Report{
			FileID:      file.ID,
			ReportType:  "ttop",
			Status:      "pending",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}

		err := db.InsertReport(pendingReport)
		require.NoError(t, err)

		// Get pending reports
		reports, err := db.GetPendingReports()
		require.NoError(t, err)

		found := false
		for _, report := range reports {
			if report.ID == pendingReport.ID {
				found = true
				assert.Equal(t, "pending", report.Status)
			}
		}
		assert.True(t, found, "Should find the pending report")
	})

	t.Run("UpdateReportStatus", func(t *testing.T) {
		// Insert a report
		report := &Report{
			FileID:      file.ID,
			ReportType:  "ttop",
			Status:      "pending",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}

		err := db.InsertReport(report)
		require.NoError(t, err)

		// Update status to running
		err = db.UpdateReportStatus(report.ID, "running")
		require.NoError(t, err)

		// Verify status was updated
		updatedReport, err := db.GetReportByID(report.ID)
		require.NoError(t, err)
		assert.Equal(t, "running", updatedReport.Status)
	})

	t.Run("CompleteReport", func(t *testing.T) {
		// Insert a report
		report := &Report{
			FileID:      file.ID,
			ReportType:  "ttop",
			Status:      "running",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}

		err := db.InsertReport(report)
		require.NoError(t, err)

		reportData := `{"type": "ttop", "summary": "Test report"}`

		// Complete the report
		err = db.CompleteReport(report.ID, reportData)
		require.NoError(t, err)

		// Verify report was completed
		completedReport, err := db.GetReportByID(report.ID)
		require.NoError(t, err)
		assert.Equal(t, "completed", completedReport.Status)
		assert.Equal(t, reportData, completedReport.ReportData)
		assert.NotNil(t, completedReport.CompletedTime)
	})

	t.Run("FailReport", func(t *testing.T) {
		// Insert a report
		report := &Report{
			FileID:      file.ID,
			ReportType:  "ttop",
			Status:      "running",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}

		err := db.InsertReport(report)
		require.NoError(t, err)

		errorMessage := "Test error message"

		// Fail the report
		err = db.FailReport(report.ID, errorMessage)
		require.NoError(t, err)

		// Verify report was failed
		failedReport, err := db.GetReportByID(report.ID)
		require.NoError(t, err)
		assert.Equal(t, "failed", failedReport.Status)
		assert.Equal(t, errorMessage, failedReport.ErrorMessage)
		assert.NotNil(t, failedReport.CompletedTime)
	})

	t.Run("GetReportsByFileID", func(t *testing.T) {
		// Insert multiple reports for the same file
		reports := []*Report{
			{
				FileID:      file.ID,
				ReportType:  "ttop",
				Status:      "completed",
				CreatedTime: time.Now().Add(-1 * time.Hour),
				DDDVersion:  "1.0.0",
			},
			{
				FileID:      file.ID,
				ReportType:  "iostat",
				Status:      "pending",
				CreatedTime: time.Now(),
				DDDVersion:  "1.0.0",
			},
		}

		for _, report := range reports {
			err := db.InsertReport(report)
			require.NoError(t, err)
		}

		// Get reports for the file
		fileReports, err := db.GetReportsByFileID(file.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(fileReports), 2, "Should have at least 2 reports")

		// Verify all reports belong to the correct file
		for _, report := range fileReports {
			assert.Equal(t, file.ID, report.FileID)
		}
	})
}
