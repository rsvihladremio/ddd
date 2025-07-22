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

package testutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rsvihladremio/ddd/internal/config"
	"github.com/stretchr/testify/require"
)

// secureReadFile safely reads a file with path validation for testing
func secureReadFile(filePath string) ([]byte, error) {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(filePath)

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("invalid file path: directory traversal detected")
	}

	return os.ReadFile(cleanPath)
}

// TestConfig creates a test configuration with temporary directories
func TestConfig(t *testing.T) *config.Config {
	t.Helper()

	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "uploads")
	dbPath := filepath.Join(tempDir, "test.db")

	err := os.MkdirAll(uploadsDir, 0750)
	require.NoError(t, err)

	return &config.Config{
		Port:              "8080",
		DBPath:            dbPath,
		UploadsDir:        uploadsDir,
		MaxDiskUsage:      0.8,
		FileRetentionDays: 7,
	}
}

// TestFile represents a test file fixture
type TestFile struct {
	Name     string
	Content  []byte
	FileType string
}

// CreateTestFile creates a test file on disk and returns its hash
func CreateTestFile(t *testing.T, uploadsDir string, testFile TestFile) (string, string) {
	t.Helper()

	hasher := sha256.New()
	hasher.Write(testFile.Content)
	hash := hex.EncodeToString(hasher.Sum(nil))

	filePath := filepath.Join(uploadsDir, hash)
	err := os.WriteFile(filePath, testFile.Content, 0600)
	require.NoError(t, err)

	return hash, filePath
}

// SampleFiles provides common test file fixtures
var SampleFiles = map[string]TestFile{
	"ttop": {
		Name:     "ttop.txt",
		Content:  []byte("PID USER TIME %CPU COMMAND\n1234 root 10:30 25.5 java -jar app.jar\n5678 user 10:31 15.2 python script.py\n"),
		FileType: "ttop",
	},
	"iostat": {
		Name: "iostat.txt",
		Content: []byte(`Linux 5.10.0-32-cloud-amd64 (test-system) 	09/04/24 	_x86_64_	(4 CPU)

09/04/24 12:07:20
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
           2.36    0.00    0.40    0.04    0.01   97.20

Device            r/s     rkB/s   rrqm/s  %rrqm r_await rareq-sz     w/s     wkB/s   wrqm/s  %wrqm w_await wareq-sz     d/s     dkB/s   drqm/s  %drqm d_await dareq-sz     f/s f_await  aqu-sz  %util
sda              2.08     94.38     0.31  13.07    0.89    45.47    9.58    210.39     5.55  36.68    2.74    21.96    0.09    377.20     0.00   0.00    0.95  4151.86    3.94    0.06    0.03   1.39

09/04/24 12:07:21
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
          33.91    0.00    7.67    2.72    0.00   55.69

Device            r/s     rkB/s   rrqm/s  %rrqm r_await rareq-sz     w/s     wkB/s   wrqm/s  %wrqm w_await wareq-sz     d/s     dkB/s   drqm/s  %drqm d_await dareq-sz     f/s f_await  aqu-sz  %util
sda              0.00      0.00     0.00   0.00    0.00     0.00  395.00  38116.00   133.00  25.19    8.65    96.50    1.00      4.00     0.00   0.00    1.00     4.00  122.00    0.06    3.42  39.20`),
		FileType: "iostat",
	},
	"queries_json": {
		Name:     "queries.json",
		Content:  []byte(`{"queries": [{"id": "123", "sql": "SELECT * FROM table", "duration": 1000}]}`),
		FileType: "queries_json",
	},
	"unknown": {
		Name:     "unknown.txt",
		Content:  []byte("This is an unknown file type"),
		FileType: "unknown",
	},
}

// CreateSampleFile creates a sample file from the predefined fixtures
func CreateSampleFile(t *testing.T, uploadsDir, fileType string) (string, string) {
	t.Helper()

	testFile, exists := SampleFiles[fileType]
	require.True(t, exists, "Sample file type %s not found", fileType)

	return CreateTestFile(t, uploadsDir, testFile)
}

// AssertFileExists checks if a file exists at the given path
func AssertFileExists(t *testing.T, filePath string) {
	t.Helper()

	_, err := os.Stat(filePath)
	require.NoError(t, err, "File should exist at %s", filePath)
}

// AssertFileNotExists checks if a file does not exist at the given path
func AssertFileNotExists(t *testing.T, filePath string) {
	t.Helper()

	_, err := os.Stat(filePath)
	require.True(t, os.IsNotExist(err), "File should not exist at %s", filePath)
}

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	t.Helper()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ticker.C:
			if condition() {
				return
			}
		case <-timeoutCh:
			t.Fatalf("Timeout waiting for condition: %s", message)
		}
	}
}

// CreateTempFile creates a temporary file with given content
func CreateTempFile(t *testing.T, content []byte, filename string) string {
	t.Helper()

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, filename)

	err := os.WriteFile(filePath, content, 0600)
	require.NoError(t, err)

	return filePath
}

// ReadFileContent reads and returns file content
func ReadFileContent(t *testing.T, filePath string) []byte {
	t.Helper()

	content, err := secureReadFile(filePath)
	require.NoError(t, err)

	return content
}

// CreateMultipartFormData creates multipart form data for file upload testing
func CreateMultipartFormData(t *testing.T, fieldName, fileName string, content []byte) ([]byte, string) {
	t.Helper()

	boundary := "----WebKitFormBoundary7MA4YWxkTrZu0gW"

	var body strings.Builder
	body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n", fieldName, fileName))
	body.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	body.Write(content)
	body.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary))

	contentType := fmt.Sprintf("multipart/form-data; boundary=%s", boundary)

	return []byte(body.String()), contentType
}

// MockReader implements io.Reader for testing
type MockReader struct {
	data []byte
	pos  int
	err  error
}

// NewMockReader creates a new mock reader
func NewMockReader(data []byte, err error) *MockReader {
	return &MockReader{
		data: data,
		err:  err,
	}
}

// Read implements io.Reader
func (m *MockReader) Read(p []byte) (n int, err error) {
	if m.err != nil {
		return 0, m.err
	}

	if m.pos >= len(m.data) {
		return 0, io.EOF
	}

	n = copy(p, m.data[m.pos:])
	m.pos += n

	return n, nil
}
