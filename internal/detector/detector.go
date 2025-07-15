package detector

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"path/filepath"
	"strings"
)

// FileType constants
const (
	FileTypeJFR           = "jfr"
	FileTypeTTop          = "ttop"
	FileTypeIOStat        = "iostat"
	FileTypeDremioProfile = "dremio_profile"
	FileTypeQueriesJSON   = "queries_json"
	FileTypeArchive       = "archive"
	FileTypeUnknown       = "unknown"
)

// DetectFileType detects the type of file based on filename and content
func DetectFileType(filename string, content []byte) string {
	// Check by file extension first
	ext := strings.ToLower(filepath.Ext(filename))
	baseName := strings.ToLower(filepath.Base(filename))

	// JFR files
	if ext == ".jfr" {
		return FileTypeJFR
	}

	// Archive files
	if isArchive(ext) {
		return detectArchiveContent(content)
	}

	// ttop.txt files
	if strings.Contains(baseName, "ttop") && (ext == ".txt" || ext == "") {
		if isTTopFile(content) {
			return FileTypeTTop
		}
	}

	// iostat files
	if strings.Contains(baseName, "iostat") || isIOStatFile(content) {
		return FileTypeIOStat
	}

	// queries.json files
	if strings.Contains(baseName, "queries") && ext == ".json" {
		if isQueriesJSONFile(content) {
			return FileTypeQueriesJSON
		}
	}

	// Dremio profile files (usually JSON with specific structure)
	if isDremioProfileFile(content) {
		return FileTypeDremioProfile
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
		defer gzReader.Close()
		tarReader := tar.NewReader(gzReader)
		return readTarFiles(tarReader)
	}

	// Try plain tar
	reader.Seek(0, 0)
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

// analyzeFileList determines the primary file type based on file list
func analyzeFileList(files []string) string {
	jfrCount := 0
	ttopCount := 0
	iostatCount := 0
	queriesJSONCount := 0
	profileCount := 0

	for _, filename := range files {
		fileType := DetectFileType(filename, nil) // We can't read content from archive easily
		switch fileType {
		case FileTypeJFR:
			jfrCount++
		case FileTypeTTop:
			ttopCount++
		case FileTypeIOStat:
			iostatCount++
		case FileTypeQueriesJSON:
			queriesJSONCount++
		case FileTypeDremioProfile:
			profileCount++
		}
	}

	// Return the most common type
	if queriesJSONCount > 0 {
		return FileTypeQueriesJSON
	}
	if jfrCount > 0 {
		return FileTypeJFR
	}
	if profileCount > 0 {
		return FileTypeDremioProfile
	}
	if ttopCount > 0 {
		return FileTypeTTop
	}
	if iostatCount > 0 {
		return FileTypeIOStat
	}

	return FileTypeArchive
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
	var data interface{}
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
	var data map[string]interface{}
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
