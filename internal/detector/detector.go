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
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
)

// FileType constants
const (
	FileTypeJFR           = "jfr"
	FileTypeTTop          = "ttop"
	FileTypeIOStat        = "iostat"
	FileTypeArchive       = "archive"
	FileTypeUnknown       = "unknown"
)

// DetectFileType detects the type of file based on content first, then filename as fallback
func DetectFileType(filename string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))

	// Handle archives first (they need special processing)
	if isArchive(ext) {
		return detectArchiveContent(content)
	}

	// If we have content, prioritize content-based detection
	if len(content) > 0 {
		// Try content-based detection first
		if isTTopFile(content) {
			return FileTypeTTop
		}

		if isIOStatFile(content) {
			return FileTypeIOStat
		}
	}

	// Fallback to filename-based detection
	baseName := strings.ToLower(filepath.Base(filename))

	// JFR files (primarily filename-based since they're binary)
	if ext == ".jfr" {
		return FileTypeJFR
	}

	// Filename-based fallbacks for when content is not available or doesn't match
	if strings.Contains(baseName, "ttop") && (ext == ".txt" || ext == "") {
		return FileTypeTTop
	}

	if strings.Contains(baseName, "iostat") {
		return FileTypeIOStat
	}

	return FileTypeUnknown
}

// isArchive checks if the file extension indicates an archive
func isArchive(ext string) bool {
	archiveExts := []string{".zip", ".tar", ".tar.gz", ".tgz", ".gz"}
	for _, archiveExt := range archiveExts {
		if ext == archiveExt {
			return true
		}
	}
	return false
}

// detectArchiveContent analyzes archive content to determine the primary file type
func detectArchiveContent(content []byte) string {
	// Try to detect ZIP files
	if zipFiles := extractZipFileList(content); len(zipFiles) > 0 {
		return analyzeFileList(zipFiles)
	}

	// Try to detect TAR files (including gzipped)
	if tarFiles := extractTarFileList(content); len(tarFiles) > 0 {
		return analyzeFileList(tarFiles)
	}

	return FileTypeArchive
}

// extractZipFileList extracts file list from ZIP archive
func extractZipFileList(content []byte) []string {
	reader := bytes.NewReader(content)
	zipReader, err := zip.NewReader(reader, int64(len(content)))
	if err != nil {
		return nil
	}

	var files []string
	for _, file := range zipReader.File {
		if !file.FileInfo().IsDir() {
			files = append(files, file.Name)
		}
	}
	return files
}

// extractTarFileList extracts file list from TAR archive (handles gzip compression)
func extractTarFileList(content []byte) []string {
	reader := bytes.NewReader(content)

	// Try gzipped tar first
	if gzReader, err := gzip.NewReader(reader); err == nil {
		defer func() {
			if err := gzReader.Close(); err != nil {
				log.Printf("Error closing gzip reader: %v", err)
			}
		}()
		tarReader := tar.NewReader(gzReader)
		return readTarFiles(tarReader)
	}

	// Try plain tar
	if _, err := reader.Seek(0, 0); err != nil {
		log.Printf("Error seeking reader: %v", err)
		return []string{}
	}
	tarReader := tar.NewReader(reader)
	return readTarFiles(tarReader)
}

// readTarFiles reads file names from tar reader
func readTarFiles(tarReader *tar.Reader) []string {
	var files []string
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		if header.Typeflag == tar.TypeReg {
			files = append(files, header.Name)
		}
	}
	return files
}

// analyzeFileList determines the primary file type based on file list (filename patterns only)
func analyzeFileList(files []string) string {
	jfrCount := 0
	ttopCount := 0
	iostatCount := 0

	for _, filename := range files {
		// Use filename-based detection only for archives
		fileType := detectFileTypeByName(filename)
		switch fileType {
		case FileTypeJFR:
			jfrCount++
		case FileTypeTTop:
			ttopCount++
		case FileTypeIOStat:
			iostatCount++
		}
	}

	// Return the most common type
	if jfrCount > 0 {
		return FileTypeJFR
	}
	if ttopCount > 0 {
		return FileTypeTTop
	}
	if iostatCount > 0 {
		return FileTypeIOStat
	}

	return FileTypeArchive
}

// detectFileTypeByName detects file type based only on filename patterns (for archive analysis)
func detectFileTypeByName(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	baseName := strings.ToLower(filepath.Base(filename))

	// JFR files
	if ext == ".jfr" {
		return FileTypeJFR
	}

	// TTop files (look for ttop in filename)
	if strings.Contains(baseName, "ttop") && (ext == ".txt" || ext == "") {
		return FileTypeTTop
	}

	// IOStat files
	if strings.Contains(baseName, "iostat") {
		return FileTypeIOStat
	}

	return FileTypeUnknown
}

// isTTopFile checks if content looks like a ttop file
func isTTopFile(content []byte) bool {
	contentStr := string(content[:min(1000, len(content))])
	// ttop files typically have headers like "PID", "USER", "TIME", etc.
	return strings.Contains(contentStr, "PID") &&
		strings.Contains(contentStr, "USER") &&
		(strings.Contains(contentStr, "TIME") || strings.Contains(contentStr, "%CPU"))
}

// isIOStatFile checks if content looks like an iostat file
func isIOStatFile(content []byte) bool {
	contentStr := string(content[:min(1000, len(content))])
	// iostat files typically have headers like "Device", "tps", "kB_read/s", etc.
	return strings.Contains(contentStr, "Device") &&
		(strings.Contains(contentStr, "tps") ||
			strings.Contains(contentStr, "kB_read/s") ||
			strings.Contains(contentStr, "r/s"))
}

// isQueriesJSONFile checks if content looks like a queries.json file
func isQueriesJSONFile(content []byte) bool {
	// Try to parse as JSON and check for query-like structure
	var data any 
	if err := json.Unmarshal(content, &data); err != nil {
		return false
	}

	// Check if it's an array or object with query-related fields
	contentStr := string(content[:min(1000, len(content))])
	return strings.Contains(contentStr, "query") ||
		strings.Contains(contentStr, "sql") ||
		strings.Contains(contentStr, "execution")
}

// isDremioProfileFile checks if content looks like a Dremio profile file
func isDremioProfileFile(content []byte) bool {
	// Try to parse as JSON and check for Dremio-specific fields
	var data map[string]any
	if err := json.Unmarshal(content, &data); err != nil {
		return false
	}

	// Check for Dremio-specific fields
	contentStr := strings.ToLower(string(content[:min(2000, len(content))]))
	return strings.Contains(contentStr, "dremio") ||
		strings.Contains(contentStr, "fragment") ||
		strings.Contains(contentStr, "operator") ||
		(strings.Contains(contentStr, "profile") && strings.Contains(contentStr, "query"))
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
