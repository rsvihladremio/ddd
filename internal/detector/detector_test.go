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

package detector

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/rsvihladremio/ddd/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		content      []byte
		expectedType string
	}{
		{
			name:         "JFR file by extension",
			filename:     "profile.jfr",
			content:      []byte("FLR\x00"),
			expectedType: FileTypeJFR,
		},
		{
			name:         "TTop file by name and content",
			filename:     "ttop.txt",
			content:      testutil.SampleFiles["ttop"].Content,
			expectedType: FileTypeTTop,
		},
		{
			name:         "TTop file with different name",
			filename:     "system_ttop_output.txt",
			content:      testutil.SampleFiles["ttop"].Content,
			expectedType: FileTypeTTop,
		},
		{
			name:         "IOStat file by content",
			filename:     "iostat_output.txt",
			content:      testutil.SampleFiles["iostat"].Content,
			expectedType: FileTypeIOStat,
		},
		{
			name:         "Unknown file type",
			filename:     "unknown.txt",
			content:      []byte("This is some random content"),
			expectedType: FileTypeUnknown,
		},
		{
			name:         "Empty file",
			filename:     "empty.txt",
			content:      []byte(""),
			expectedType: FileTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFileType(tt.filename, tt.content)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

func TestDetectArchiveContent(t *testing.T) {
	t.Run("ZIP archive with JFR files", func(t *testing.T) {
		zipContent := createTestZip(t, map[string][]byte{
			"profile1.jfr": []byte("FLR\x00"),
			"profile2.jfr": []byte("FLR\x00"),
			"readme.txt":   []byte("This is a readme"),
		})

		result := DetectFileType("archive.zip", zipContent)
		assert.Equal(t, FileTypeJFR, result)
	})

	t.Run("ZIP archive with ttop files", func(t *testing.T) {
		zipContent := createTestZip(t, map[string][]byte{
			"ttop.txt":     testutil.SampleFiles["ttop"].Content,
			"ttop_old.txt": testutil.SampleFiles["ttop"].Content,
		})

		result := DetectFileType("archive.zip", zipContent)
		// With content-first detection, archives with ttop files should be detected as ttop
		assert.Equal(t, FileTypeTTop, result)
	})

	t.Run("Archive with unknown content", func(t *testing.T) {
		zipContent := createTestZip(t, map[string][]byte{
			"file1.txt": []byte("unknown content"),
			"file2.txt": []byte("more unknown content"),
		})

		result := DetectFileType("archive.zip", zipContent)
		assert.Equal(t, FileTypeArchive, result)
	})
}

func TestIsTTopFile(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "Valid ttop content",
			content:  testutil.SampleFiles["ttop"].Content,
			expected: true,
		},
		{
			name:     "Valid ttop with different header",
			content:  []byte("PID  USER     TIME  %CPU COMMAND\n1234 root     10:30 25.5 java -jar app.jar\n"),
			expected: true,
		},
		{
			name:     "Invalid content - no PID header",
			content:  []byte("USER TIME %CPU COMMAND\nroot 10:30 25.5 java\n"),
			expected: false,
		},
		{
			name:     "Invalid content - wrong format",
			content:  []byte("This is not a ttop file"),
			expected: false,
		},
		{
			name:     "Empty content",
			content:  []byte(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTTopFile(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsIOStatFile(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "Valid iostat content",
			content:  testutil.SampleFiles["iostat"].Content,
			expected: true,
		},
		{
			name:     "Valid iostat with different format",
			content:  []byte("Device:         rrqm/s   wrqm/s     r/s     w/s\nsda               0.00     0.00    0.00    0.00\n"),
			expected: true,
		},
		{
			name:     "Invalid content - no Device header",
			content:  []byte("rrqm/s wrqm/s r/s w/s\n0.00 0.00 0.00 0.00\n"),
			expected: false,
		},
		{
			name:     "Invalid content - wrong format",
			content:  []byte("This is not an iostat file"),
			expected: false,
		},
		{
			name:     "Empty content",
			content:  []byte(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIOStatFile(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsQueriesJSONFile(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "Valid queries JSON",
			content:  testutil.SampleFiles["queries_json"].Content,
			expected: true,
		},
		{
			name:     "Valid queries JSON with different structure",
			content:  []byte(`{"queries": [{"id": "456", "sql": "SELECT COUNT(*) FROM users"}]}`),
			expected: true,
		},
		{
			name:     "Invalid JSON",
			content:  []byte(`{"queries": [`),
			expected: false,
		},
		{
			name:     "Valid JSON but no queries field",
			content:  []byte(`{"data": [{"id": "123"}]}`),
			expected: false,
		},
		{
			name:     "Empty content",
			content:  []byte(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isQueriesJSONFile(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDremioProfileFile(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "Valid Dremio profile",
			content:  []byte(`{"query": {"sql": "SELECT * FROM table"}, "profile": {"duration": 1000}}`),
			expected: true,
		},
		{
			name:     "Valid Dremio profile with different structure",
			content:  []byte(`{"query": {"queryId": "123"}, "profile": {"nodes": []}}`),
			expected: true,
		},
		{
			name:     "Invalid JSON",
			content:  []byte(`{"query": {`),
			expected: false,
		},
		{
			name:     "Valid JSON but missing query field",
			content:  []byte(`{"profile": {"duration": 1000}}`),
			expected: false,
		},
		{
			name:     "Valid JSON but missing profile field",
			content:  []byte(`{"query": {"sql": "SELECT * FROM table"}}`),
			expected: false,
		},
		{
			name:     "Empty content",
			content:  []byte(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDremioProfileFile(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsArchive(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{".zip", ".zip", true},
		{".tar", ".tar", true},
		{".gz", ".gz", true},
		{".tgz", ".tgz", true},
		{".tar.gz", ".tar.gz", true},
		{".txt", ".txt", false},
		{".jfr", ".jfr", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isArchive(tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper functions for creating test archives

func createTestZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for filename, content := range files {
		writer, err := zipWriter.Create(filename)
		require.NoError(t, err)

		_, err = writer.Write(content)
		require.NoError(t, err)
	}

	err := zipWriter.Close()
	require.NoError(t, err)

	return buf.Bytes()
}

func createTestTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	for filename, content := range files {
		header := &tar.Header{
			Name: filename,
			Size: int64(len(content)),
		}

		err := tarWriter.WriteHeader(header)
		require.NoError(t, err)

		_, err = tarWriter.Write(content)
		require.NoError(t, err)
	}

	err := tarWriter.Close()
	require.NoError(t, err)

	err = gzipWriter.Close()
	require.NoError(t, err)

	return buf.Bytes()
}
