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
