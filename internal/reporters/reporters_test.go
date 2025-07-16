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

func TestGenerateTTopReport_HTMLReport(t *testing.T) {
	t.Run("HTML report included in generated report", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "ttop_with_html.txt")

		// Create sample ttop content with multiple snapshots
		sampleContent := `top - 12:02:03 up  3:07,  0 users,  load average: 3.18, 1.16, 0.41
Threads: 262 total,   6 running, 256 sleeping,   0 stopped,   0 zombie
%Cpu(s): 85.7 us,  7.1 sy,  0.0 ni,  5.7 id,  1.4 wa,  0.0 hi,  0.0 si,  0.0 st
MiB Mem :  16008.2 total,  10953.7 free,   3713.5 used,   1341.1 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.  12032.0 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
    997 dremio    20   0 7009048   3.4g  98412 R  87.5  21.9   1:36.52 C2 CompilerThre
    996 dremio    20   0 7009048   3.4g  98412 R  81.2  21.9   1:35.89 C2 CompilerThre
   5190 dremio    20   0 7009064   3.4g  98412 S  18.8  21.9   0:03.83 rbound-command1

top - 12:02:04 up  3:07,  0 users,  load average: 3.18, 1.16, 0.41
Threads: 262 total,   2 running, 260 sleeping,   0 stopped,   0 zombie
%Cpu(s): 75.3 us,  3.2 sy,  0.0 ni, 20.4 id,  0.0 wa,  0.0 hi,  1.0 si,  0.0 st
MiB Mem :  16008.2 total,  10953.7 free,   3713.5 used,   1341.1 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.  12032.0 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
    996 dremio    20   0 7008232   3.4g  98412 S  82.2  21.9   1:36.72 C2 CompilerThre
    997 dremio    20   0 7008232   3.4g  98412 R  82.2  21.9   1:37.35 C2 CompilerThre
    998 dremio    20   0 7008232   3.4g  98412 S  14.9  21.9   0:36.57 C1 CompilerThre`

		err := os.WriteFile(filePath, []byte(sampleContent), 0644)
		require.NoError(t, err)

		// Generate report
		reportJSON, err := GenerateTTopReport(filePath)
		require.NoError(t, err)
		assert.NotEmpty(t, reportJSON)

		// Parse and validate the report
		var report map[string]interface{}
		err = json.Unmarshal([]byte(reportJSON), &report)
		require.NoError(t, err)

		// Verify all original fields are still present
		assert.Equal(t, "ttop", report["type"])
		assert.Equal(t, float64(len(sampleContent)), report["file_size"])
		assert.Contains(t, report, "summary")
		assert.Contains(t, report, "analysis")
		assert.Contains(t, report, "generated_at")

		// Verify new fields are present
		assert.Contains(t, report, "html_report")
		assert.Contains(t, report, "snapshot_count")
		assert.Contains(t, report, "unique_threads")
		assert.Contains(t, report, "peak_threads")

		// Verify HTML report contains chart elements
		htmlReport, ok := report["html_report"].(string)
		require.True(t, ok, "html_report should be a string")
		assert.NotEmpty(t, htmlReport)

		// Check that HTML contains expected chart canvases
		assert.Contains(t, htmlReport, "<canvas id=\"threadCountChart\">")
		assert.Contains(t, htmlReport, "<canvas id=\"cpuChart\">")
		assert.Contains(t, htmlReport, "<canvas id=\"memoryChart\">")

		// Verify Chart.js is included
		assert.Contains(t, htmlReport, "chart.js")
		assert.Contains(t, htmlReport, "new Chart(")

		// Verify snapshot count is correct
		assert.Equal(t, float64(2), report["snapshot_count"])

		// Verify unique threads count
		assert.Equal(t, float64(4), report["unique_threads"]) // PIDs: 997, 996, 5190, 998

		// Verify peak threads count
		assert.Equal(t, float64(3), report["peak_threads"]) // First snapshot has 3 threads
	})

	t.Run("HTML report with empty ttop content", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "empty_ttop.txt")

		err := os.WriteFile(filePath, []byte(""), 0644)
		require.NoError(t, err)

		reportJSON, err := GenerateTTopReport(filePath)
		require.NoError(t, err)

		var report map[string]interface{}
		err = json.Unmarshal([]byte(reportJSON), &report)
		require.NoError(t, err)

		// Should still have html_report field
		assert.Contains(t, report, "html_report")
		htmlReport, ok := report["html_report"].(string)
		require.True(t, ok)
		assert.NotEmpty(t, htmlReport)

		// Should indicate no data available
		assert.Contains(t, htmlReport, "No data available")
	})

	t.Run("HTML report content validation", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "single_snapshot.txt")

		sampleContent := `top - 15:30:45 up 1 day,  5:23,  2 users,  load average: 1.23, 1.45, 1.67
Threads: 100 total,   2 running, 98 sleeping,   0 stopped,   0 zombie

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
   1234 root      20   0  123456  12345   1234 R  25.5  10.2   0:30.12 test-process
   5678 user      20   0  654321  54321   4321 S  15.0   5.1   0:15.30 another-process`

		err := os.WriteFile(filePath, []byte(sampleContent), 0644)
		require.NoError(t, err)

		reportJSON, err := GenerateTTopReport(filePath)
		require.NoError(t, err)

		var report map[string]interface{}
		err = json.Unmarshal([]byte(reportJSON), &report)
		require.NoError(t, err)

		htmlReport := report["html_report"].(string)

		// Verify HTML structure
		assert.Contains(t, htmlReport, "<!DOCTYPE html>")
		assert.Contains(t, htmlReport, "<html lang=\"en\">")
		assert.Contains(t, htmlReport, "<head>")
		assert.Contains(t, htmlReport, "<body>")
		assert.Contains(t, htmlReport, "</html>")

		// Verify chart containers
		assert.Contains(t, htmlReport, "Thread Count Over Time")
		assert.Contains(t, htmlReport, "CPU Usage - Top 5 Busiest Threads")
		assert.Contains(t, htmlReport, "Memory Usage by User")

		// Verify Chart.js CDN is included
		assert.Contains(t, htmlReport, "cdn.jsdelivr.net/npm/chart.js")

		// Verify summary information
		assert.Contains(t, htmlReport, "1 snapshots") // Single snapshot
		assert.Contains(t, htmlReport, "2") // Should mention 2 unique threads
	})
}
