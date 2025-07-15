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

package reporters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rsvihladremio/ddd/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTTopReport(t *testing.T) {
	t.Run("Valid ttop file", func(t *testing.T) {
		// Create a temporary ttop file
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "ttop.txt")

		ttopContent := testutil.SampleFiles["ttop"].Content
		err := os.WriteFile(filePath, ttopContent, 0644)
		require.NoError(t, err)

		// Generate report
		reportJSON, err := GenerateTTopReport(filePath)
		require.NoError(t, err)
		assert.NotEmpty(t, reportJSON)

		// Parse and validate the report
		var report map[string]interface{}
		err = json.Unmarshal([]byte(reportJSON), &report)
		require.NoError(t, err)

		assert.Equal(t, "ttop", report["type"])
		assert.Equal(t, float64(len(ttopContent)), report["file_size"])
		assert.Contains(t, report, "summary")
		assert.Contains(t, report, "analysis")
		assert.Contains(t, report, "generated_at")
	})

	t.Run("Non-existent file", func(t *testing.T) {
		_, err := GenerateTTopReport("/non/existent/file.txt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file")
	})

	t.Run("Empty file", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "empty.txt")

		err := os.WriteFile(filePath, []byte(""), 0644)
		require.NoError(t, err)

		reportJSON, err := GenerateTTopReport(filePath)
		require.NoError(t, err)

		var report map[string]interface{}
		err = json.Unmarshal([]byte(reportJSON), &report)
		require.NoError(t, err)

		assert.Equal(t, float64(0), report["file_size"])
	})

	t.Run("Large ttop file", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "large_ttop.txt")

		// Create a larger ttop file with multiple processes
		largeContent := "PID USER TIME %CPU COMMAND\n"
		for i := 1000; i < 2000; i++ {
			largeContent += "1234 root 10:30 25.5 java -jar app.jar\n"
		}

		err := os.WriteFile(filePath, []byte(largeContent), 0644)
		require.NoError(t, err)

		reportJSON, err := GenerateTTopReport(filePath)
		require.NoError(t, err)

		var report map[string]interface{}
		err = json.Unmarshal([]byte(reportJSON), &report)
		require.NoError(t, err)

		assert.Equal(t, "ttop", report["type"])
		assert.Equal(t, float64(len(largeContent)), report["file_size"])
	})
}

func TestReportGeneration_Integration(t *testing.T) {
	t.Run("Generate reports for all sample file types", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test ttop report generation
		ttopPath := filepath.Join(tempDir, "ttop.txt")
		err := os.WriteFile(ttopPath, testutil.SampleFiles["ttop"].Content, 0644)
		require.NoError(t, err)

		ttopReport, err := GenerateTTopReport(ttopPath)
		require.NoError(t, err)
		assert.NotEmpty(t, ttopReport)

		// Validate JSON structure
		var reportData map[string]interface{}
		err = json.Unmarshal([]byte(ttopReport), &reportData)
		require.NoError(t, err)

		// Check required fields
		requiredFields := []string{"type", "file_size", "summary", "analysis", "generated_at"}
		for _, field := range requiredFields {
			assert.Contains(t, reportData, field, "Report should contain field: %s", field)
		}
	})

	t.Run("Report generation with special characters", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "special_chars.txt")

		// Content with special characters and unicode
		specialContent := "PID USER TIME %CPU COMMAND\n1234 röot 10:30 25.5 java -jar app.jar\n5678 üser 10:31 15.2 python script.py\n"

		err := os.WriteFile(filePath, []byte(specialContent), 0644)
		require.NoError(t, err)

		reportJSON, err := GenerateTTopReport(filePath)
		require.NoError(t, err)

		// Should be valid JSON despite special characters
		var report map[string]interface{}
		err = json.Unmarshal([]byte(reportJSON), &report)
		require.NoError(t, err)
	})

	t.Run("Concurrent report generation", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create multiple test files
		numFiles := 5
		filePaths := make([]string, numFiles)

		for i := 0; i < numFiles; i++ {
			filePath := filepath.Join(tempDir, "ttop_"+string(rune('a'+i))+".txt")
			err := os.WriteFile(filePath, testutil.SampleFiles["ttop"].Content, 0644)
			require.NoError(t, err)
			filePaths[i] = filePath
		}

		// Generate reports concurrently
		results := make(chan string, numFiles)
		errors := make(chan error, numFiles)

		for _, filePath := range filePaths {
			go func(path string) {
				report, err := GenerateTTopReport(path)
				if err != nil {
					errors <- err
					return
				}
				results <- report
			}(filePath)
		}

		// Collect results
		for i := 0; i < numFiles; i++ {
			select {
			case report := <-results:
				assert.NotEmpty(t, report)

				// Validate JSON
				var reportData map[string]interface{}
				err := json.Unmarshal([]byte(report), &reportData)
				require.NoError(t, err)

			case err := <-errors:
				t.Fatalf("Unexpected error in concurrent report generation: %v", err)
			}
		}
	})
}

func TestReportValidation(t *testing.T) {
	t.Run("Report JSON structure validation", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.txt")

		err := os.WriteFile(filePath, testutil.SampleFiles["ttop"].Content, 0644)
		require.NoError(t, err)

		reportJSON, err := GenerateTTopReport(filePath)
		require.NoError(t, err)

		// Parse the JSON
		var report map[string]interface{}
		err = json.Unmarshal([]byte(reportJSON), &report)
		require.NoError(t, err)

		// Validate field types
		assert.IsType(t, "", report["type"], "type should be string")
		assert.IsType(t, float64(0), report["file_size"], "file_size should be number")
		assert.IsType(t, "", report["summary"], "summary should be string")
		assert.IsType(t, "", report["analysis"], "analysis should be string")
		assert.IsType(t, "", report["generated_at"], "generated_at should be string")

		// Validate field values
		assert.Equal(t, "ttop", report["type"])
		assert.Greater(t, report["file_size"], float64(0))
		assert.NotEmpty(t, report["summary"])
		assert.NotEmpty(t, report["analysis"])
		assert.NotEmpty(t, report["generated_at"])
	})

	t.Run("Report consistency across multiple generations", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "consistent.txt")

		content := testutil.SampleFiles["ttop"].Content
		err := os.WriteFile(filePath, content, 0644)
		require.NoError(t, err)

		// Generate the same report multiple times
		reports := make([]map[string]interface{}, 3)

		for i := 0; i < 3; i++ {
			reportJSON, err := GenerateTTopReport(filePath)
			require.NoError(t, err)

			err = json.Unmarshal([]byte(reportJSON), &reports[i])
			require.NoError(t, err)
		}

		// Check that consistent fields are the same
		for i := 1; i < len(reports); i++ {
			assert.Equal(t, reports[0]["type"], reports[i]["type"])
			assert.Equal(t, reports[0]["file_size"], reports[i]["file_size"])
			assert.Equal(t, reports[0]["summary"], reports[i]["summary"])
			// Note: generated_at might differ, so we don't check it
		}
	})
}

func TestReportErrorHandling(t *testing.T) {
	t.Run("Permission denied", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping permission test when running as root")
		}

		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "no_permission.txt")

		// Create file and remove read permission
		err := os.WriteFile(filePath, testutil.SampleFiles["ttop"].Content, 0644)
		require.NoError(t, err)

		err = os.Chmod(filePath, 0000)
		require.NoError(t, err)

		// Restore permissions for cleanup
		t.Cleanup(func() {
			if err := os.Chmod(filePath, 0644); err != nil {
				t.Logf("Warning: could not restore file permissions: %v", err)
			}
		})

		_, err = GenerateTTopReport(filePath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file")
	})

	t.Run("Directory instead of file", func(t *testing.T) {
		tempDir := t.TempDir()

		_, err := GenerateTTopReport(tempDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file")
	})
}
