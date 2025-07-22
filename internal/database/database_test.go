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

package database

import (
	"fmt"
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

		// Test pagination without search
		retrievedFiles, err := db.GetFiles(10, 0, false, "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(retrievedFiles), 2, "Should retrieve at least 2 files")

		// Files should be ordered by upload_time DESC
		if len(retrievedFiles) >= 2 {
			assert.True(t, retrievedFiles[0].UploadTime.After(retrievedFiles[1].UploadTime) ||
				retrievedFiles[0].UploadTime.Equal(retrievedFiles[1].UploadTime))
		}
	})

	t.Run("GetFiles_WithSearch", func(t *testing.T) {
		// Insert test files with different names and types for search testing
		searchTestFiles := []*File{
			{
				Hash:         "search-hash-1",
				OriginalName: "performance_data.txt",
				FileType:     "ttop",
				FileSize:     150,
				UploadTime:   time.Now().Add(-3 * time.Hour),
				FilePath:     "/uploads/search-hash-1",
			},
			{
				Hash:         "search-hash-2",
				OriginalName: "system_metrics.log",
				FileType:     "iostat",
				FileSize:     250,
				UploadTime:   time.Now().Add(-2 * time.Hour),
				FilePath:     "/uploads/search-hash-2",
			},
			{
				Hash:         "search-hash-3",
				OriginalName: "database_performance.csv",
				FileType:     "unknown",
				FileSize:     350,
				UploadTime:   time.Now().Add(-1 * time.Hour),
				FilePath:     "/uploads/search-hash-3",
			},
		}

		for _, file := range searchTestFiles {
			err := db.InsertFile(file)
			require.NoError(t, err)
		}

		// Test search by filename
		files, err := db.GetFiles(10, 0, false, "performance")
		require.NoError(t, err)
		assert.Len(t, files, 2, "Should find 2 files with 'performance' in name")

		// Verify the correct files were found
		foundHashes := make(map[string]bool)
		for _, file := range files {
			foundHashes[file.Hash] = true
		}
		assert.True(t, foundHashes["search-hash-1"], "Should find performance_data.txt")
		assert.True(t, foundHashes["search-hash-3"], "Should find database_performance.csv")

		// Test search by file type
		files, err = db.GetFiles(10, 0, false, "iostat")
		require.NoError(t, err)
		// Find our specific test file among the results
		foundTestFile := false
		for _, file := range files {
			if file.Hash == "search-hash-2" {
				foundTestFile = true
				break
			}
		}
		assert.True(t, foundTestFile, "Should find our test file with 'iostat' type")

		// Test search by hash
		files, err = db.GetFiles(10, 0, false, "search-hash-1")
		require.NoError(t, err)
		assert.Len(t, files, 1, "Should find exactly 1 file with matching hash")
		assert.Equal(t, "search-hash-1", files[0].Hash)

		// Test search by partial hash - use a more specific pattern
		files, err = db.GetFiles(10, 0, false, "search-hash-2")
		require.NoError(t, err)
		assert.Len(t, files, 1, "Should find file with partial hash match")
		assert.Equal(t, "search-hash-2", files[0].Hash)

		// Test case-insensitive search
		files, err = db.GetFiles(10, 0, false, "PERFORMANCE")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(files), 2, "Search should be case-insensitive and find at least 2 files")

		// Test partial match - search for "performance_data" to be more specific
		files, err = db.GetFiles(10, 0, false, "performance_data")
		require.NoError(t, err)
		assert.Len(t, files, 1, "Should find exactly 1 file with 'performance_data' in name")
		assert.Equal(t, "search-hash-1", files[0].Hash)

		// Test search with no results
		files, err = db.GetFiles(10, 0, false, "nonexistent")
		require.NoError(t, err)
		assert.Len(t, files, 0, "Should find no files for non-existent search term")

		// Test empty search query (should return all files)
		files, err = db.GetFiles(10, 0, false, "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(files), 3, "Empty search should return all files")
	})

	t.Run("GetFiles_SearchWithDeleted", func(t *testing.T) {
		// Insert a file and then delete it
		deletedFile := &File{
			Hash:         "deleted-search-hash",
			OriginalName: "deleted_performance.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/deleted-search-hash",
		}

		err := db.InsertFile(deletedFile)
		require.NoError(t, err)

		// Mark as deleted
		err = db.MarkFileDeleted(deletedFile.ID)
		require.NoError(t, err)

		// Search without including deleted files
		files, err := db.GetFiles(10, 0, false, "deleted_performance")
		require.NoError(t, err)
		assert.Len(t, files, 0, "Should not find deleted files when includeDeleted=false")

		// Search including deleted files
		files, err = db.GetFiles(10, 0, true, "deleted_performance")
		require.NoError(t, err)
		assert.Len(t, files, 1, "Should find deleted files when includeDeleted=true")
		assert.True(t, files[0].Deleted, "Found file should be marked as deleted")
	})

	t.Run("GetFiles_SearchWithPagination", func(t *testing.T) {
		// Insert multiple files with similar names for pagination testing
		paginationFiles := []*File{
			{
				Hash:         "page-hash-1",
				OriginalName: "test_file_1.txt",
				FileType:     "ttop",
				FileSize:     100,
				UploadTime:   time.Now().Add(-4 * time.Hour),
				FilePath:     "/uploads/page-hash-1",
			},
			{
				Hash:         "page-hash-2",
				OriginalName: "test_file_2.txt",
				FileType:     "ttop",
				FileSize:     200,
				UploadTime:   time.Now().Add(-3 * time.Hour),
				FilePath:     "/uploads/page-hash-2",
			},
			{
				Hash:         "page-hash-3",
				OriginalName: "test_file_3.txt",
				FileType:     "ttop",
				FileSize:     300,
				UploadTime:   time.Now().Add(-2 * time.Hour),
				FilePath:     "/uploads/page-hash-3",
			},
		}

		for _, file := range paginationFiles {
			err := db.InsertFile(file)
			require.NoError(t, err)
		}

		// Test first page
		files, err := db.GetFiles(2, 0, false, "test_file")
		require.NoError(t, err)
		assert.Len(t, files, 2, "First page should have 2 files")

		// Test second page
		files, err = db.GetFiles(2, 2, false, "test_file")
		require.NoError(t, err)
		assert.Len(t, files, 1, "Second page should have 1 file")

		// Verify ordering (should be DESC by upload_time)
		allFiles, err := db.GetFiles(10, 0, false, "test_file")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(allFiles), 3, "Should have at least 3 test files")

		// Check that files are ordered by upload_time DESC
		for i := 1; i < len(allFiles); i++ {
			assert.True(t, allFiles[i-1].UploadTime.After(allFiles[i].UploadTime) ||
				allFiles[i-1].UploadTime.Equal(allFiles[i].UploadTime),
				"Files should be ordered by upload_time DESC")
		}
	})

	t.Run("GetFiles_LimitParameter", func(t *testing.T) {
		// Insert multiple files to test limit functionality
		limitTestFiles := make([]*File, 8)
		for i := 0; i < 8; i++ {
			limitTestFiles[i] = &File{
				Hash:         fmt.Sprintf("limit-test-hash-%d", i),
				OriginalName: fmt.Sprintf("limit_test_%d.txt", i),
				FileType:     "ttop",
				FileSize:     100,
				UploadTime:   time.Now().Add(-time.Duration(i) * time.Minute),
				FilePath:     fmt.Sprintf("/uploads/limit-test-hash-%d", i),
			}
		}

		for _, file := range limitTestFiles {
			err := db.InsertFile(file)
			require.NoError(t, err)
		}

		// Test limit of 5
		files, err := db.GetFiles(5, 0, false, "")
		require.NoError(t, err)
		assert.LessOrEqual(t, len(files), 5, "Should return at most 5 files")

		// Test limit of 3
		files, err = db.GetFiles(3, 0, false, "")
		require.NoError(t, err)
		assert.LessOrEqual(t, len(files), 3, "Should return at most 3 files")

		// Test limit of 1
		files, err = db.GetFiles(1, 0, false, "")
		require.NoError(t, err)
		assert.LessOrEqual(t, len(files), 1, "Should return at most 1 file")

		// Test limit with search
		files, err = db.GetFiles(3, 0, false, "limit_test")
		require.NoError(t, err)
		assert.LessOrEqual(t, len(files), 3, "Should return at most 3 files with search")

		// Verify all returned files match the search
		for _, file := range files {
			assert.Contains(t, file.OriginalName, "limit_test", "All files should match search")
		}
	})

	t.Run("GetFilesCount", func(t *testing.T) {
		// Insert test files for count testing
		countTestFiles := make([]*File, 5)
		for i := 0; i < 5; i++ {
			countTestFiles[i] = &File{
				Hash:         fmt.Sprintf("count-test-hash-%d", i),
				OriginalName: fmt.Sprintf("count_test_%d.txt", i),
				FileType:     "ttop",
				FileSize:     100,
				UploadTime:   time.Now().Add(-time.Duration(i) * time.Minute),
				FilePath:     fmt.Sprintf("/uploads/count-test-hash-%d", i),
			}
		}

		for _, file := range countTestFiles {
			err := db.InsertFile(file)
			require.NoError(t, err)
		}

		// Test count without search
		count, err := db.GetFilesCount(false, "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 5, "Should count at least 5 files")

		// Test count with search
		count, err = db.GetFilesCount(false, "count_test")
		require.NoError(t, err)
		assert.Equal(t, 5, count, "Should count exactly 5 files matching search")

		// Test count with search that matches nothing
		count, err = db.GetFilesCount(false, "nonexistent")
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Should count 0 files for non-matching search")

		// Test count including deleted files
		// First mark one file as deleted
		err = db.MarkFileDeleted(countTestFiles[0].ID)
		require.NoError(t, err)

		// Count without deleted files
		count, err = db.GetFilesCount(false, "count_test")
		require.NoError(t, err)
		assert.Equal(t, 4, count, "Should count 4 files excluding deleted")

		// Count including deleted files
		count, err = db.GetFilesCount(true, "count_test")
		require.NoError(t, err)
		assert.Equal(t, 5, count, "Should count 5 files including deleted")
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

	t.Run("RestoreFile", func(t *testing.T) {
		// Insert a test file
		file := &File{
			Hash:         "restore-test-hash",
			OriginalName: "restore-test.txt",
			FileType:     "ttop",
			FileSize:     512,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/restore-test-hash",
		}

		err := db.InsertFile(file)
		require.NoError(t, err)

		// Mark as deleted
		err = db.MarkFileDeleted(file.ID)
		require.NoError(t, err)

		// Verify it's marked as deleted
		deletedFile, err := db.GetFileByHash("restore-test-hash")
		require.NoError(t, err)
		assert.True(t, deletedFile.Deleted)
		assert.NotNil(t, deletedFile.DeletedTime)

		// Restore the file with new metadata
		newOriginalName := "restored-file.txt"
		newFileType := "iostat"
		newFileSize := int64(1024)
		newFilePath := "/uploads/restored-file-hash"

		err = db.RestoreFile(file.ID, newOriginalName, newFileType, newFileSize, newFilePath)
		require.NoError(t, err)

		// Verify it's restored
		restoredFile, err := db.GetFileByHash("restore-test-hash")
		require.NoError(t, err)
		assert.False(t, restoredFile.Deleted)
		assert.Nil(t, restoredFile.DeletedTime)
		assert.Equal(t, newOriginalName, restoredFile.OriginalName)
		assert.Equal(t, newFileType, restoredFile.FileType)
		assert.Equal(t, newFileSize, restoredFile.FileSize)
		assert.Equal(t, newFilePath, restoredFile.FilePath)
		// Upload time should be updated
		assert.True(t, restoredFile.UploadTime.After(file.UploadTime))
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

func TestDatabase_Settings(t *testing.T) {
	db := testDB(t)

	t.Run("Set and get setting", func(t *testing.T) {
		err := db.SetSetting("test_key", "test_value")
		require.NoError(t, err)

		value, err := db.GetSetting("test_key")
		require.NoError(t, err)
		assert.Equal(t, "test_value", value)
	})

	t.Run("Get non-existent setting", func(t *testing.T) {
		_, err := db.GetSetting("non_existent_key")
		assert.Error(t, err)
	})

	t.Run("Update existing setting", func(t *testing.T) {
		err := db.SetSetting("update_key", "original_value")
		require.NoError(t, err)

		err = db.SetSetting("update_key", "updated_value")
		require.NoError(t, err)

		value, err := db.GetSetting("update_key")
		require.NoError(t, err)
		assert.Equal(t, "updated_value", value)
	})

	t.Run("Get all settings", func(t *testing.T) {
		// Set some test settings
		err := db.SetSetting("setting1", "value1")
		require.NoError(t, err)
		err = db.SetSetting("setting2", "value2")
		require.NoError(t, err)

		settings, err := db.GetAllSettings()
		require.NoError(t, err)
		assert.Contains(t, settings, "setting1")
		assert.Contains(t, settings, "setting2")
		assert.Equal(t, "value1", settings["setting1"])
		assert.Equal(t, "value2", settings["setting2"])
	})

	t.Run("Initialize settings", func(t *testing.T) {
		defaults := map[string]string{
			"max_disk_usage":      "0.5",
			"file_retention_days": "14",
		}

		err := db.InitializeSettings(defaults)
		require.NoError(t, err)

		// Verify settings were created
		value, err := db.GetSetting("max_disk_usage")
		require.NoError(t, err)
		assert.Equal(t, "0.5", value)

		value, err = db.GetSetting("file_retention_days")
		require.NoError(t, err)
		assert.Equal(t, "14", value)

		// Initialize again - should not overwrite existing values
		defaults["max_disk_usage"] = "0.8"
		err = db.InitializeSettings(defaults)
		require.NoError(t, err)

		// Should still have original value
		value, err = db.GetSetting("max_disk_usage")
		require.NoError(t, err)
		assert.Equal(t, "0.5", value)
	})
}

func TestDatabase_FileCleanup(t *testing.T) {
	db := testDB(t)

	t.Run("GetReportCountByFileID", func(t *testing.T) {
		// Create a test file
		file := &File{
			Hash:         "count-test-hash",
			OriginalName: "count-test.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/count-test-hash",
		}

		err := db.InsertFile(file)
		require.NoError(t, err)

		// Initially should have 0 reports
		count, err := db.GetReportCountByFileID(file.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		// Add two reports
		report1 := &Report{
			FileID:      file.ID,
			ReportType:  "ttop",
			Status:      "completed",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}

		report2 := &Report{
			FileID:      file.ID,
			ReportType:  "iostat",
			Status:      "completed",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}

		err = db.InsertReport(report1)
		require.NoError(t, err)
		err = db.InsertReport(report2)
		require.NoError(t, err)

		// Should now have 2 reports
		count, err = db.GetReportCountByFileID(file.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		// Delete one report
		err = db.DeleteReport(report1.ID)
		require.NoError(t, err)

		// Should now have 1 report
		count, err = db.GetReportCountByFileID(file.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("DeleteFileCompletely", func(t *testing.T) {
		// Create a test file
		file := &File{
			Hash:         "delete-test-hash",
			OriginalName: "delete-test.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/delete-test-hash",
		}

		err := db.InsertFile(file)
		require.NoError(t, err)

		// Verify file exists
		_, err = db.GetFileByID(file.ID)
		require.NoError(t, err)

		// Delete file completely
		err = db.DeleteFileCompletely(file.ID)
		require.NoError(t, err)

		// File should no longer exist
		_, err = db.GetFileByID(file.ID)
		assert.Error(t, err, "File should not exist after complete deletion")
	})
}
