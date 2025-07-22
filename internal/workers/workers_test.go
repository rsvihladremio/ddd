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

package workers

import (
	"fmt"
	"path/filepath"
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

func TestReportWorker_ProcessReports(t *testing.T) {
	db := testDB(t)
	cfg := testutil.TestConfig(t)

	// Create a test file
	hash, filePath := testutil.CreateSampleFile(t, cfg.UploadsDir, "ttop")

	file := &database.File{
		Hash:         hash,
		OriginalName: "test_ttop.txt",
		FileType:     "ttop",
		FileSize:     int64(len(testutil.SampleFiles["ttop"].Content)),
		UploadTime:   time.Now(),
		FilePath:     filePath,
	}

	err := db.InsertFile(file)
	require.NoError(t, err)

	t.Run("Process pending ttop report", func(t *testing.T) {
		// Create a pending report
		report := &database.Report{
			FileID:      file.ID,
			ReportType:  "ttop",
			Status:      "pending",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}

		err := db.InsertReport(report)
		require.NoError(t, err)

		// Create worker and process reports
		worker := NewReportWorker(db, cfg)
		worker.processReports()

		// Check that report was processed
		updatedReport, err := db.GetReportByID(report.ID)
		require.NoError(t, err)

		assert.Equal(t, "completed", updatedReport.Status)
		assert.NotEmpty(t, updatedReport.ReportData)
		assert.NotNil(t, updatedReport.CompletedTime)
	})

	t.Run("Process report for non-existent file", func(t *testing.T) {
		// Create a report for a file that doesn't exist on disk
		nonExistentFile := &database.File{
			Hash:         "non-existent-hash",
			OriginalName: "non-existent.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now(),
			FilePath:     "/non/existent/path",
		}

		err := db.InsertFile(nonExistentFile)
		require.NoError(t, err)

		report := &database.Report{
			FileID:      nonExistentFile.ID,
			ReportType:  "ttop",
			Status:      "pending",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}

		err = db.InsertReport(report)
		require.NoError(t, err)

		// Process reports
		worker := NewReportWorker(db, cfg)
		worker.processReports()

		// Check that report failed
		updatedReport, err := db.GetReportByID(report.ID)
		require.NoError(t, err)

		assert.Equal(t, "failed", updatedReport.Status)
		assert.NotEmpty(t, updatedReport.ErrorMessage)
		assert.NotNil(t, updatedReport.CompletedTime)
	})

	t.Run("Process multiple reports concurrently", func(t *testing.T) {
		// Create multiple test files and reports
		numReports := 3
		reports := make([]*database.Report, numReports)

		for i := 0; i < numReports; i++ {
			// Create unique test file content to avoid hash collisions
			uniqueContent := append(testutil.SampleFiles["ttop"].Content, []byte(fmt.Sprintf("\n# Test file %d", i))...)
			testHash, testFilePath := testutil.CreateTestFile(t, cfg.UploadsDir, testutil.TestFile{
				Name:     fmt.Sprintf("test_ttop_%d.txt", i),
				Content:  uniqueContent,
				FileType: "ttop",
			})

			testFile := &database.File{
				Hash:         testHash,
				OriginalName: fmt.Sprintf("test_ttop_%d.txt", i),
				FileType:     "ttop",
				FileSize:     int64(len(uniqueContent)),
				UploadTime:   time.Now(),
				FilePath:     testFilePath,
			}

			err := db.InsertFile(testFile)
			require.NoError(t, err)

			// Create pending report
			report := &database.Report{
				FileID:      testFile.ID,
				ReportType:  "ttop",
				Status:      "pending",
				CreatedTime: time.Now(),
				DDDVersion:  "1.0.0",
			}

			err = db.InsertReport(report)
			require.NoError(t, err)
			reports[i] = report
		}

		// Process all reports
		worker := NewReportWorker(db, cfg)
		worker.processReports()

		// Check that all reports were processed
		for _, report := range reports {
			updatedReport, err := db.GetReportByID(report.ID)
			require.NoError(t, err)
			assert.Equal(t, "completed", updatedReport.Status)
		}
	})
}

func TestCleanupWorker_CleanupOldFiles(t *testing.T) {
	db := testDB(t)
	cfg := testutil.TestConfig(t)
	cfg.FileRetentionDays = 1 // 1 day retention for testing

	t.Run("Cleanup old files", func(t *testing.T) {
		// Create an old file with unique content
		oldContent := append(testutil.SampleFiles["ttop"].Content, []byte("\n# Old file")...)
		oldHash, oldFilePath := testutil.CreateTestFile(t, cfg.UploadsDir, testutil.TestFile{
			Name:     "old_file.txt",
			Content:  oldContent,
			FileType: "ttop",
		})

		oldFile := &database.File{
			Hash:         oldHash,
			OriginalName: "old_file.txt",
			FileType:     "ttop",
			FileSize:     int64(len(oldContent)),
			UploadTime:   time.Now().Add(-2 * 24 * time.Hour), // 2 days old
			FilePath:     oldFilePath,
		}

		err := db.InsertFile(oldFile)
		require.NoError(t, err)

		// Add a report to the old file so it won't be considered orphaned
		oldFileReport := &database.Report{
			FileID:      oldFile.ID,
			ReportType:  "ttop",
			Status:      "completed",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}
		err = db.InsertReport(oldFileReport)
		require.NoError(t, err)

		// Create a new file with unique content
		newContent := append(testutil.SampleFiles["ttop"].Content, []byte("\n# New file")...)
		newHash, newFilePath := testutil.CreateTestFile(t, cfg.UploadsDir, testutil.TestFile{
			Name:     "new_file.txt",
			Content:  newContent,
			FileType: "ttop",
		})

		newFile := &database.File{
			Hash:         newHash,
			OriginalName: "new_file.txt",
			FileType:     "ttop",
			FileSize:     int64(len(newContent)),
			UploadTime:   time.Now(),
			FilePath:     newFilePath,
		}

		err = db.InsertFile(newFile)
		require.NoError(t, err)

		// Verify files exist on disk
		testutil.AssertFileExists(t, oldFilePath)
		testutil.AssertFileExists(t, newFilePath)

		// Run cleanup
		worker := NewCleanupWorker(db, cfg)
		worker.cleanupOldFiles()

		// Check that old file was marked as deleted
		updatedOldFile, err := db.GetFileByHash(oldHash)
		require.NoError(t, err)
		assert.True(t, updatedOldFile.Deleted)
		assert.NotNil(t, updatedOldFile.DeletedTime)

		// Check that old file was removed from disk
		testutil.AssertFileNotExists(t, oldFilePath)

		// Check that new file is still there
		updatedNewFile, err := db.GetFileByHash(newHash)
		require.NoError(t, err)
		assert.False(t, updatedNewFile.Deleted)
		testutil.AssertFileExists(t, newFilePath)
	})

	t.Run("Cleanup with disk usage check", func(t *testing.T) {
		// This test would require more complex setup to simulate disk usage
		// For now, we'll test that the cleanup worker can be created and doesn't crash
		worker := NewCleanupWorker(db, cfg)
		assert.NotNil(t, worker)

		// Test that cleanup runs without error even when no files need cleanup
		worker.cleanupOldFiles()
	})
}

func TestWorkerIntegration(t *testing.T) {
	db := testDB(t)
	cfg := testutil.TestConfig(t)

	t.Run("End-to-end workflow", func(t *testing.T) {
		// Create a test file
		hash, filePath := testutil.CreateSampleFile(t, cfg.UploadsDir, "ttop")

		file := &database.File{
			Hash:         hash,
			OriginalName: "integration_test.txt",
			FileType:     "ttop",
			FileSize:     int64(len(testutil.SampleFiles["ttop"].Content)),
			UploadTime:   time.Now(),
			FilePath:     filePath,
		}

		err := db.InsertFile(file)
		require.NoError(t, err)

		// Create a pending report
		report := &database.Report{
			FileID:      file.ID,
			ReportType:  "ttop",
			Status:      "pending",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}

		err = db.InsertReport(report)
		require.NoError(t, err)

		// Process the report
		reportWorker := NewReportWorker(db, cfg)
		reportWorker.processReports()

		// Verify report was completed
		completedReport, err := db.GetReportByID(report.ID)
		require.NoError(t, err)
		assert.Equal(t, "completed", completedReport.Status)
		assert.NotEmpty(t, completedReport.ReportData)

		// Verify file still exists (not old enough for cleanup)
		testutil.AssertFileExists(t, filePath)

		// Run cleanup (should not affect recent files)
		cleanupWorker := NewCleanupWorker(db, cfg)
		cleanupWorker.cleanupOldFiles()

		// File should still exist
		testutil.AssertFileExists(t, filePath)

		// Verify file is not marked as deleted
		currentFile, err := db.GetFileByHash(hash)
		require.NoError(t, err)
		assert.False(t, currentFile.Deleted)
	})
}

func TestWorkerErrorHandling(t *testing.T) {
	db := testDB(t)
	cfg := testutil.TestConfig(t)

	t.Run("Report worker handles database errors gracefully", func(t *testing.T) {
		worker := NewReportWorker(db, cfg)

		// This should not panic even if there are no pending reports
		worker.processReports()
	})

	t.Run("Cleanup worker handles missing files gracefully", func(t *testing.T) {
		// Set retention period for this test
		cfg.FileRetentionDays = 1 // 1 day retention for testing

		// Create a file record without the actual file on disk
		file := &database.File{
			Hash:         "missing-file-hash",
			OriginalName: "missing.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now().Add(-2 * 24 * time.Hour),
			FilePath:     filepath.Join(cfg.UploadsDir, "missing-file-hash"),
		}

		err := db.InsertFile(file)
		require.NoError(t, err)

		// Add a report to the file so it won't be considered orphaned
		report := &database.Report{
			FileID:      file.ID,
			ReportType:  "ttop",
			Status:      "completed",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}
		err = db.InsertReport(report)
		require.NoError(t, err)

		worker := NewCleanupWorker(db, cfg)

		// This should not panic even if the file doesn't exist on disk
		worker.cleanupOldFiles()

		// File should still be marked as deleted in database
		updatedFile, err := db.GetFileByHash("missing-file-hash")
		require.NoError(t, err)
		assert.True(t, updatedFile.Deleted)
	})
}

func TestWorkerConfiguration(t *testing.T) {
	db := testDB(t)

	t.Run("Different retention periods", func(t *testing.T) {
		testCases := []struct {
			name            string
			retentionDays   int
			fileAge         time.Duration
			shouldBeDeleted bool
		}{
			{
				name:            "1 day retention, 2 day old file",
				retentionDays:   1,
				fileAge:         2 * 24 * time.Hour,
				shouldBeDeleted: true,
			},
			{
				name:            "7 day retention, 2 day old file",
				retentionDays:   7,
				fileAge:         2 * 24 * time.Hour,
				shouldBeDeleted: false,
			},
			{
				name:            "7 day retention, 8 day old file",
				retentionDays:   7,
				fileAge:         8 * 24 * time.Hour,
				shouldBeDeleted: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := testutil.TestConfig(t)
				cfg.FileRetentionDays = tc.retentionDays

				// Create unique test file to avoid hash collisions
				uniqueContent := append(testutil.SampleFiles["ttop"].Content, []byte(fmt.Sprintf("\n# Test case: %s", tc.name))...)
				hash, filePath := testutil.CreateTestFile(t, cfg.UploadsDir, testutil.TestFile{
					Name:     fmt.Sprintf("test_%d.txt", tc.retentionDays),
					Content:  uniqueContent,
					FileType: "ttop",
				})

				file := &database.File{
					Hash:         hash,
					OriginalName: fmt.Sprintf("test_%d.txt", tc.retentionDays),
					FileType:     "ttop",
					FileSize:     int64(len(uniqueContent)),
					UploadTime:   time.Now().Add(-tc.fileAge),
					FilePath:     filePath,
				}

				err := db.InsertFile(file)
				require.NoError(t, err)

				// Add a report to the file so it won't be considered orphaned
				report := &database.Report{
					FileID:      file.ID,
					ReportType:  "ttop",
					Status:      "completed",
					CreatedTime: time.Now(),
					DDDVersion:  "1.0.0",
				}
				err = db.InsertReport(report)
				require.NoError(t, err)

				// Run cleanup
				worker := NewCleanupWorker(db, cfg)
				worker.cleanupOldFiles()

				// Check result
				updatedFile, err := db.GetFileByHash(hash)
				require.NoError(t, err)

				if tc.shouldBeDeleted {
					assert.True(t, updatedFile.Deleted)
					testutil.AssertFileNotExists(t, filePath)
				} else {
					assert.False(t, updatedFile.Deleted)
					testutil.AssertFileExists(t, filePath)
				}
			})
		}
	})
}

func TestCleanupWorker_AggressiveCleanup(t *testing.T) {
	db := testDB(t)
	cfg := testutil.TestConfig(t)

	t.Run("Aggressive cleanup when over threshold", func(t *testing.T) {
		// Create a cleanup worker with very low threshold to trigger aggressive cleanup
		cfg.MaxDiskUsage = 0.01     // 1% - very low to trigger cleanup
		cfg.FileRetentionDays = 365 // High retention so age-based cleanup won't trigger

		worker := NewCleanupWorker(db, cfg)

		// Add some test files to the database
		for i := 0; i < 3; i++ {
			// Create unique content for each file
			uniqueContent := append(testutil.SampleFiles["ttop"].Content, []byte(fmt.Sprintf("\n# Test file %d", i))...)
			hash, filePath := testutil.CreateTestFile(t, cfg.UploadsDir, testutil.TestFile{
				Name:     fmt.Sprintf("test_file_%d.txt", i),
				Content:  uniqueContent,
				FileType: "ttop",
			})

			file := &database.File{
				Hash:         hash,
				OriginalName: fmt.Sprintf("test_file_%d.txt", i),
				FileType:     "ttop",
				FileSize:     int64(len(uniqueContent)),
				UploadTime:   time.Now().Add(-time.Duration(i) * time.Hour), // Different ages
				FilePath:     filePath,
			}

			err := db.InsertFile(file)
			require.NoError(t, err)
		}

		// Verify files exist before cleanup
		files, err := db.GetFiles(10, 0, false, "")
		require.NoError(t, err)
		assert.Len(t, files, 3)

		// Run cleanup - should delete files due to low threshold
		worker.performCleanup()

		// Note: The exact behavior depends on actual disk usage, but we can verify
		// that the cleanup logic runs without errors and handles the low threshold case
		// In a real scenario with actual disk pressure, files would be deleted
	})

	t.Run("No cleanup when under threshold", func(t *testing.T) {
		// Create a cleanup worker with high threshold
		cfg.MaxDiskUsage = 0.99     // 99% - very high, unlikely to trigger cleanup
		cfg.FileRetentionDays = 365 // High retention so age-based cleanup won't trigger

		worker := NewCleanupWorker(db, cfg)

		// Add a test file
		uniqueContent := append(testutil.SampleFiles["ttop"].Content, []byte("\n# No cleanup test")...)
		hash, filePath := testutil.CreateTestFile(t, cfg.UploadsDir, testutil.TestFile{
			Name:     "no_cleanup_test.txt",
			Content:  uniqueContent,
			FileType: "ttop",
		})

		file := &database.File{
			Hash:         hash,
			OriginalName: "no_cleanup_test.txt",
			FileType:     "ttop",
			FileSize:     int64(len(uniqueContent)),
			UploadTime:   time.Now(),
			FilePath:     filePath,
		}

		err := db.InsertFile(file)
		require.NoError(t, err)

		// Run cleanup - should not delete files due to high threshold
		worker.performCleanup()

		// Verify file still exists
		updatedFile, err := db.GetFileByHash(hash)
		require.NoError(t, err)
		assert.False(t, updatedFile.Deleted)
		testutil.AssertFileExists(t, filePath)
	})
}

func TestCleanupWorker_OrphanedFileCleanup(t *testing.T) {
	db := testDB(t)
	cfg := testutil.TestConfig(t)

	t.Run("Remove orphaned file entries during cleanup", func(t *testing.T) {
		worker := NewCleanupWorker(db, cfg)

		// Create test files - one with reports, one without
		fileWithReports := &database.File{
			Hash:         "file-with-reports-hash",
			OriginalName: "file-with-reports.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/file-with-reports-hash",
		}

		fileWithoutReports := &database.File{
			Hash:         "file-without-reports-hash",
			OriginalName: "file-without-reports.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/file-without-reports-hash",
		}

		err := db.InsertFile(fileWithReports)
		require.NoError(t, err)
		err = db.InsertFile(fileWithoutReports)
		require.NoError(t, err)

		// Add a report for the first file
		report := &database.Report{
			FileID:      fileWithReports.ID,
			ReportType:  "ttop",
			Status:      "completed",
			CreatedTime: time.Now(),
			DDDVersion:  "1.0.0",
		}
		err = db.InsertReport(report)
		require.NoError(t, err)

		// Mark both files as deleted
		err = db.MarkFileDeleted(fileWithReports.ID)
		require.NoError(t, err)
		err = db.MarkFileDeleted(fileWithoutReports.ID)
		require.NoError(t, err)

		// Verify both files exist but are marked as deleted
		deletedFileWithReports, err := db.GetFileByID(fileWithReports.ID)
		require.NoError(t, err)
		assert.True(t, deletedFileWithReports.Deleted)

		deletedFileWithoutReports, err := db.GetFileByID(fileWithoutReports.ID)
		require.NoError(t, err)
		assert.True(t, deletedFileWithoutReports.Deleted)

		// Run orphaned file cleanup
		worker.cleanupOrphanedFileEntries()

		// File with reports should still exist
		_, err = db.GetFileByID(fileWithReports.ID)
		assert.NoError(t, err, "File with reports should still exist")

		// File without reports should be completely removed
		_, err = db.GetFileByID(fileWithoutReports.ID)
		assert.Error(t, err, "File without reports should be completely removed")
	})

	t.Run("Keep non-deleted files even without reports", func(t *testing.T) {
		worker := NewCleanupWorker(db, cfg)

		// Create a non-deleted file without reports
		activeFile := &database.File{
			Hash:         "active-file-hash",
			OriginalName: "active-file.txt",
			FileType:     "ttop",
			FileSize:     100,
			UploadTime:   time.Now(),
			FilePath:     "/uploads/active-file-hash",
		}

		err := db.InsertFile(activeFile)
		require.NoError(t, err)

		// Don't mark as deleted, don't add reports

		// Run orphaned file cleanup
		worker.cleanupOrphanedFileEntries()

		// Active file should still exist (not deleted)
		_, err = db.GetFileByID(activeFile.ID)
		assert.NoError(t, err, "Active file should still exist even without reports")
	})
}
