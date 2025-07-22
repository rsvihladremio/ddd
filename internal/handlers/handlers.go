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

package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rsvihladremio/ddd/internal/config"
	"github.com/rsvihladremio/ddd/internal/database"
	"github.com/rsvihladremio/ddd/internal/detector"
)

const DDDVersion = "1.0.0"

// CleanupWorker interface to avoid circular imports
type CleanupWorker interface {
	TriggerCleanup()
}

// Handlers contains the HTTP handlers
type Handlers struct {
	db            *database.DB
	cfg           *config.Config
	cleanupWorker CleanupWorker
}

// New creates a new Handlers instance
func New(db *database.DB, cfg *config.Config, cleanupWorker CleanupWorker) *Handlers {
	return &Handlers{
		db:            db,
		cfg:           cfg,
		cleanupWorker: cleanupWorker,
	}
}

// getMaxDiskUsage retrieves max disk usage setting from database
func (h *Handlers) getMaxDiskUsage() (float64, error) {
	value, err := h.db.GetSetting("max_disk_usage")
	if err != nil {
		// Fall back to config if setting not found
		return h.cfg.MaxDiskUsage, nil
	}
	return strconv.ParseFloat(value, 64)
}

// getFileRetentionDays retrieves file retention days setting from database
func (h *Handlers) getFileRetentionDays() (int, error) {
	value, err := h.db.GetSetting("file_retention_days")
	if err != nil {
		// Fall back to config if setting not found
		return h.cfg.FileRetentionDays, nil
	}
	return strconv.Atoi(value)
}

// HandleIndex serves the main page
func (h *Handlers) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "./web/index.html")
}

// HandleReportPage serves the report viewer page
func (h *Handlers) HandleReportPage(w http.ResponseWriter, r *http.Request) {
	// Extract report ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 2 {
		http.NotFound(w, r)
		return
	}

	reportIDStr := pathParts[1]
	reportID, err := strconv.Atoi(reportIDStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Verify report exists
	report, err := h.db.GetReportByID(reportID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Get file information
	file, err := h.db.GetFileByID(report.FileID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Serve the report page with metadata
	h.serveReportPage(w, report, file)
}

// HandleUpload handles file uploads
func (h *Handlers) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(100 << 20) // 100MB max
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing uploaded file: %v", err)
		}
	}()

	// Calculate file hash
	hasher := sha256.New()
	fileContent, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}
	hasher.Write(fileContent)
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Check if file already exists
	existingFile, err := h.db.GetFileByHash(hash)
	if err == nil {
		if !existingFile.Deleted {
			// File already exists and is not deleted, return existing file info
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"file":    existingFile,
				"message": "File already exists",
			}); err != nil {
				log.Printf("Error encoding JSON response: %v", err)
			}
			return
		} else {
			// File exists but is deleted - restore it
			fileType := detector.DetectFileType(header.Filename, fileContent)
			filePath := filepath.Join(h.cfg.UploadsDir, hash)

			// Validate that the file path is within the uploads directory
			if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(h.cfg.UploadsDir)) {
				http.Error(w, "Invalid file path", http.StatusBadRequest)
				return
			}

			// Save file to disk
			outFile, err := os.Create(filePath)
			if err != nil {
				http.Error(w, "Failed to save file", http.StatusInternalServerError)
				return
			}
			defer func() {
				if err := outFile.Close(); err != nil {
					log.Printf("Error closing output file: %v", err)
				}
			}()

			_, err = outFile.Write(fileContent)
			if err != nil {
				http.Error(w, "Failed to write file", http.StatusInternalServerError)
				return
			}

			// Restore the file in database
			err = h.db.RestoreFile(existingFile.ID, header.Filename, fileType, int64(len(fileContent)), filePath)
			if err != nil {
				http.Error(w, "Failed to restore file record", http.StatusInternalServerError)
				return
			}

			// Get updated file record
			restoredFile, err := h.db.GetFileByHash(hash)
			if err != nil {
				http.Error(w, "Failed to get restored file", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"file":    restoredFile,
				"message": "File restored successfully",
			}); err != nil {
				log.Printf("Error encoding JSON response: %v", err)
			}
			return
		}
	}

	// Detect file type
	fileType := detector.DetectFileType(header.Filename, fileContent)

	// Save file to disk
	filePath := filepath.Join(h.cfg.UploadsDir, hash)
	// Validate that the file path is within the uploads directory
	if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(h.cfg.UploadsDir)) {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}
	outFile, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			log.Printf("Error closing output file: %v", err)
		}
	}()

	_, err = outFile.Write(fileContent)
	if err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	// Save file record to database
	dbFile := &database.File{
		Hash:         hash,
		OriginalName: header.Filename,
		FileType:     fileType,
		FileSize:     int64(len(fileContent)),
		UploadTime:   time.Now(),
		FilePath:     filePath,
	}

	err = h.db.InsertFile(dbFile)
	if err != nil {
		http.Error(w, "Failed to save file record", http.StatusInternalServerError)
		return
	}

	// Automatically create a report for the uploaded file if we know how to handle it
	if h.shouldAutoGenerateReport(fileType) {
		report := &database.Report{
			FileID:      dbFile.ID,
			ReportType:  fileType,
			Status:      "pending",
			CreatedTime: time.Now(),
			DDDVersion:  DDDVersion,
		}

		if err := h.db.InsertReport(report); err != nil {
			// Log error but don't fail the upload
			log.Printf("Failed to create automatic report for file %d: %v", dbFile.ID, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"file":    dbFile,
		"message": "File uploaded successfully",
	}); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// HandleFiles handles file listing and searching
func (h *Handlers) HandleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	includeDeletedStr := r.URL.Query().Get("include_deleted")
	searchQuery := r.URL.Query().Get("search")

	limit := 5 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0 // default
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	includeDeleted := includeDeletedStr == "true"

	files, err := h.db.GetFiles(limit, offset, includeDeleted, searchQuery)
	if err != nil {
		http.Error(w, "Failed to get files", http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	totalCount, err := h.db.GetFilesCount(includeDeleted, searchQuery)
	if err != nil {
		http.Error(w, "Failed to get files count", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"files":       files,
		"total":       totalCount,
		"page":        (offset / limit) + 1,
		"page_size":   limit,
		"total_pages": (totalCount + limit - 1) / limit, // Ceiling division
	}); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// HandleFileOperations handles individual file operations (delete, etc.)
func (h *Handlers) HandleFileOperations(w http.ResponseWriter, r *http.Request) {
	// Extract file ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	fileIDStr := pathParts[2]
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		// Get file info first to get the file path
		file, err := h.db.GetFileByID(fileID)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		// Remove physical file from disk
		if err := os.Remove(file.FilePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: Failed to remove physical file %s: %v", file.FilePath, err)
			// Continue with database update even if file removal fails
		}

		// Mark file as deleted in database
		err = h.db.MarkFileDeleted(fileID)
		if err != nil {
			http.Error(w, "Failed to delete file", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "File deleted successfully",
		}); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleReports handles report operations
func (h *Handlers) HandleReports(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path (could be file ID or report ID depending on context)
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	idStr := pathParts[2]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Get reports by file ID
		reports, err := h.db.GetReportsByFileID(id)
		if err != nil {
			http.Error(w, "Failed to get reports", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"reports": reports,
		}); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
		}

	case http.MethodPost:
		// Create new report for file ID
		var req struct {
			ReportType string `json:"report_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		report := &database.Report{
			FileID:      id,
			ReportType:  req.ReportType,
			Status:      "pending",
			CreatedTime: time.Now(),
			DDDVersion:  DDDVersion,
		}

		err := h.db.InsertReport(report)
		if err != nil {
			http.Error(w, "Failed to create report", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"report":  report,
			"message": "Report queued for processing",
		}); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
		}

	case http.MethodDelete:
		// Get the report first to find the associated file
		report, err := h.db.GetReportByID(id)
		if err != nil {
			http.Error(w, "Report not found", http.StatusNotFound)
			return
		}

		// Delete the report
		err = h.db.DeleteReport(id)
		if err != nil {
			http.Error(w, "Failed to delete report", http.StatusInternalServerError)
			return
		}

		// Check if the associated file is deleted and has no remaining reports
		fileDeleted := false
		var deletedFileName string
		file, err := h.db.GetFileByID(report.FileID)
		if err != nil {
			log.Printf("Warning: Could not check file status after report deletion: %v", err)
		} else if file.Deleted {
			// File is deleted, check if it has any remaining reports
			reportCount, err := h.db.GetReportCountByFileID(file.ID)
			if err != nil {
				log.Printf("Warning: Could not count remaining reports for deleted file %d: %v", file.ID, err)
			} else if reportCount == 0 {
				// No reports left for this deleted file, remove it completely
				err = h.db.DeleteFileCompletely(file.ID)
				if err != nil {
					log.Printf("Warning: Could not completely delete file entry %d: %v", file.ID, err)
				} else {
					log.Printf("Completely removed deleted file entry %d (%s) - no reports remaining",
						file.ID, file.OriginalName)
					fileDeleted = true
					deletedFileName = file.OriginalName
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": true,
			"message": "Report deleted successfully",
		}

		// Include file deletion information if a file was completely removed
		if fileDeleted {
			response["file_deleted"] = true
			response["deleted_file_id"] = report.FileID
			response["deleted_file_name"] = deletedFileName
			response["message"] = fmt.Sprintf("Report deleted successfully. File '%s' was completely removed (no reports remaining).", deletedFileName)
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleReportContent handles individual report content requests
func (h *Handlers) HandleReportContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract report ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid report ID", http.StatusBadRequest)
		return
	}

	reportIDStr := pathParts[3]
	reportID, err := strconv.Atoi(reportIDStr)
	if err != nil {
		http.Error(w, "Invalid report ID", http.StatusBadRequest)
		return
	}

	// Get the specific report
	report, err := h.db.GetReportByID(reportID)
	if err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"report_data": report.ReportData,
	}); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// HandleDiskUsage returns disk usage information for uploads and database directories
func (h *Handlers) HandleDiskUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type diskStats struct {
		Path      string  `json:"path"`
		Total     uint64  `json:"total_bytes"`
		Free      uint64  `json:"free_bytes"`
		Used      uint64  `json:"used_bytes"`
		Available uint64  `json:"available_bytes"`
		Percent   float64 `json:"used_percent"`
	}

	var stat syscall.Statfs_t

	// Get uploads directory stats
	uploadsPath := filepath.Clean(h.cfg.UploadsDir)
	if err := syscall.Statfs(uploadsPath, &stat); err != nil {
		http.Error(w, "Failed to get uploads directory stats", http.StatusInternalServerError)
		return
	}
	// Convert Bsize to uint64 - gosec G115 is acceptable here as Bsize represents block size
	blockSize := uint64(stat.Bsize) // #nosec G115

	uploadsStats := diskStats{
		Path:      uploadsPath,
		Total:     uint64(stat.Blocks) * blockSize,
		Free:      uint64(stat.Bfree) * blockSize,
		Available: uint64(stat.Bavail) * blockSize,
	}
	uploadsStats.Used = uploadsStats.Total - uploadsStats.Free
	uploadsStats.Percent = float64(uploadsStats.Used) / float64(uploadsStats.Total) * 100

	// Get database directory stats
	dbPath := filepath.Clean(filepath.Dir(h.cfg.DBPath))
	if err := syscall.Statfs(dbPath, &stat); err != nil {
		http.Error(w, "Failed to get database directory stats", http.StatusInternalServerError)
		return
	}
	dbStats := diskStats{
		Path:      dbPath,
		Total:     uint64(stat.Blocks) * blockSize,
		Free:      uint64(stat.Bfree) * blockSize,
		Available: uint64(stat.Bavail) * blockSize,
	}
	dbStats.Used = dbStats.Total - dbStats.Free
	dbStats.Percent = float64(dbStats.Used) / float64(dbStats.Total) * 100

	// Check if both paths are on same filesystem
	sameFS := false
	var stat1, stat2 syscall.Statfs_t
	if err := syscall.Statfs(uploadsPath, &stat1); err == nil {
		if err := syscall.Statfs(dbPath, &stat2); err == nil {
			sameFS = stat1.Fsid == stat2.Fsid
		}
	}

	// Get settings from database
	maxDiskUsage, err := h.getMaxDiskUsage()
	if err != nil {
		log.Printf("Error getting max disk usage setting: %v", err)
		maxDiskUsage = h.cfg.MaxDiskUsage // fallback
	}

	fileRetentionDays, err := h.getFileRetentionDays()
	if err != nil {
		log.Printf("Error getting file retention days setting: %v", err)
		fileRetentionDays = h.cfg.FileRetentionDays // fallback
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success":             true,
		"uploads":             uploadsStats,
		"database":            dbStats,
		"same_filesystem":     sameFS,
		"max_disk_usage":      maxDiskUsage,
		"file_retention_days": fileRetentionDays,
	}); err != nil {
		log.Printf("Error encoding disk usage JSON response: %v", err)
	}
}

// HandleSettings handles getting and updating application settings
func (h *Handlers) HandleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Get settings from database
		maxDiskUsage, err := h.getMaxDiskUsage()
		if err != nil {
			log.Printf("Error getting max disk usage setting: %v", err)
			maxDiskUsage = h.cfg.MaxDiskUsage // fallback
		}

		fileRetentionDays, err := h.getFileRetentionDays()
		if err != nil {
			log.Printf("Error getting file retention days setting: %v", err)
			fileRetentionDays = h.cfg.FileRetentionDays // fallback
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success":             true,
			"max_disk_usage":      maxDiskUsage,
			"file_retention_days": fileRetentionDays,
		}); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
		}
	case http.MethodPost:
		var req struct {
			MaxDiskUsage      string `json:"max_disk_usage"`
			FileRetentionDays string `json:"file_retention_days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate and update MaxDiskUsage
		if maxUsage, err := strconv.ParseFloat(req.MaxDiskUsage, 64); err == nil {
			if maxUsage >= 0 && maxUsage <= 100 {
				// Get current threshold to check if it's being lowered
				currentThreshold, err := h.getMaxDiskUsage()
				if err != nil {
					currentThreshold = h.cfg.MaxDiskUsage // fallback
				}

				// JS sends percentage, convert to decimal and save to database
				maxUsageDecimal := maxUsage / 100.0
				if err := h.db.SetSetting("max_disk_usage", fmt.Sprintf("%.6f", maxUsageDecimal)); err != nil {
					log.Printf("Error saving max_disk_usage setting: %v", err)
					http.Error(w, "Failed to save max_disk_usage setting", http.StatusInternalServerError)
					return
				}
				// Also update config for backward compatibility
				h.cfg.MaxDiskUsage = maxUsageDecimal

				// If threshold was lowered, trigger immediate cleanup
				if maxUsageDecimal < currentThreshold && h.cleanupWorker != nil {
					log.Printf("Cleanup threshold lowered from %.1f%% to %.1f%%, triggering cleanup", currentThreshold*100, maxUsageDecimal*100)
					go h.cleanupWorker.TriggerCleanup()
				}
			} else {
				http.Error(w, "max_disk_usage must be between 0 and 100", http.StatusBadRequest)
				return
			}
		} else {
			http.Error(w, "Invalid max_disk_usage value", http.StatusBadRequest)
			return
		}

		// Validate and update FileRetentionDays
		if retentionDays, err := strconv.Atoi(req.FileRetentionDays); err == nil {
			if retentionDays >= 0 {
				if err := h.db.SetSetting("file_retention_days", fmt.Sprintf("%d", retentionDays)); err != nil {
					log.Printf("Error saving file_retention_days setting: %v", err)
					http.Error(w, "Failed to save file_retention_days setting", http.StatusInternalServerError)
					return
				}
				// Also update config for backward compatibility
				h.cfg.FileRetentionDays = retentionDays
			} else {
				http.Error(w, "file_retention_days must be non-negative", http.StatusBadRequest)
				return
			}
		} else {
			http.Error(w, "Invalid file_retention_days value", http.StatusBadRequest)
			return
		}

		log.Printf("Updated settings: MaxDiskUsage=%.2f%%, FileRetentionDays=%d", h.cfg.MaxDiskUsage*100, h.cfg.FileRetentionDays)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Settings updated successfully",
		}); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleRedetectFileType re-detects the file type for an existing file
func (h *Handlers) HandleRedetectFileType(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract file ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 { // expecting /api/files/{id}/redetect
		http.Error(w, "Invalid file ID in path", http.StatusBadRequest)
		return
	}

	fileIDStr := pathParts[2]
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	// Get file info
	file, err := h.db.GetFileByID(fileID)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Read file content from disk
	content, err := os.ReadFile(file.FilePath)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Re-detect file type
	newFileType := detector.DetectFileType(file.OriginalName, content)

	// Update the file type in database
	if err := h.db.UpdateFileFileType(fileID, newFileType); err != nil {
		http.Error(w, "Failed to update file type", http.StatusInternalServerError)
		return
	}

	// Get updated file record to return
	updatedFile, err := h.db.GetFileByID(fileID)
	if err != nil {
		http.Error(w, "Failed to retrieve updated file record", http.StatusInternalServerError)
		return
	}

	// Automatically create a report for the updated file type if we know how to handle it
	if h.shouldAutoGenerateReport(newFileType) {
		report := &database.Report{
			FileID:      updatedFile.ID,
			ReportType:  newFileType,
			Status:      "pending",
			CreatedTime: time.Now(),
			DDDVersion:  DDDVersion,
		}

		if err := h.db.InsertReport(report); err != nil {
			// Log error but don't fail the re-detect
			log.Printf("Failed to create automatic report for file %d after re-detection: %v", updatedFile.ID, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"file":    updatedFile,
		"message": "File type re-detected successfully",
	}); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// serveReportPage serves the report viewer HTML page
func (h *Handlers) serveReportPage(w http.ResponseWriter, report *database.Report, file *database.File) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DDD Report: ` + report.ReportType + ` - ` + file.OriginalName + `</title>
    <link rel="stylesheet" href="https://fonts.googleapis.com/css?family=Roboto:300,400,500,700&display=swap">
    <link rel="stylesheet" href="https://fonts.googleapis.com/icon?family=Material+Icons">
    <link rel="stylesheet" href="/static/css/material.min.css">
    <link rel="stylesheet" href="/static/css/styles.css">
    <style>
        .report-page {
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
        }
        .report-header {
            background: white;
            padding: 24px;
            border-radius: 4px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
        .report-content-page {
            background: white;
            padding: 24px;
            border-radius: 4px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            min-height: 400px;
        }
        .back-link {
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <div class="report-page">
        <div class="back-link">
            <a href="/" class="mdl-button mdl-js-button mdl-button--icon">
                <i class="material-icons">arrow_back</i>
            </a>
            <a href="/" class="mdl-button mdl-js-button">Back to Files</a>
        </div>

        <div class="report-header">
            <h1>` + report.ReportType + ` Report</h1>
            <p><strong>File:</strong> ` + file.OriginalName + `</p>
            <p><strong>Status:</strong> <span class="status-badge status-` + report.Status + `">` + report.Status + `</span></p>
            <p><strong>Created:</strong> ` + report.CreatedTime.Format("2006-01-02 15:04:05") + `</p>
            <p><strong>DDD Version:</strong> ` + report.DDDVersion + `</p>
            ` + func() string {
		if report.CompletedTime != nil {
			return `<p><strong>Completed:</strong> ` + report.CompletedTime.Format("2006-01-02 15:04:05") + `</p>`
		}
		return ""
	}() + `
            ` + func() string {
		if report.ErrorMessage != "" {
			return `<p><strong>Error:</strong> <span style="color: #d32f2f;">` + report.ErrorMessage + `</span></p>`
		}
		return ""
	}() + `
        </div>

        <div class="report-content-page" id="report-content">
            ` + func() string {
		if report.Status == "completed" {
			return `<div class="loading">Loading report content...</div>`
		} else {
			return `<div class="error-message">Report is not completed yet.</div>`
		}
	}() + `
        </div>
    </div>

    <script src="/static/js/material.min.js"></script>
    <script>
        // Load report content if completed
        if ('` + report.Status + `' === 'completed') {
            fetch('/api/reports/content/` + strconv.Itoa(report.ID) + `')
                .then(response => response.json())
                .then(data => {
                    if (data.success) {
                        document.getElementById('report-content').innerHTML = renderReportData(data.report_data);
                    } else {
                        document.getElementById('report-content').innerHTML = '<div class="error-message">Failed to load report content</div>';
                    }
                })
                .catch(error => {
                    document.getElementById('report-content').innerHTML = '<div class="error-message">Error loading report: ' + error.message + '</div>';
                });
        }

        function renderReportData(reportDataStr) {
            try {
                const reportData = JSON.parse(reportDataStr);

                // If there's an HTML report, serve it as a complete page
                if (reportData.html_report) {
                    // Replace the entire page with the HTML report
                    document.open();
                    document.write(reportData.html_report);
                    document.close();
                    return; // Don't return anything since we've replaced the page
                }

                // Fallback to summary and analysis for other report types
                return '<div class="report-content">' +
                    '<h4>Report Summary</h4>' +
                    '<p>' + (reportData.summary || 'No summary available') + '</p>' +
                    '<h4>Analysis</h4>' +
                    '<p>' + (reportData.analysis || 'No analysis available') + '</p>' +
                    '</div>';
            } catch (error) {
                return '<pre class="report-raw-data">' + escapeHtml(reportDataStr) + '</pre>';
            }
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("Error writing HTML response: %v", err)
	}
}

// shouldAutoGenerateReport determines if we should automatically generate a report for a file type
func (h *Handlers) shouldAutoGenerateReport(fileType string) bool {
	switch fileType {
	case detector.FileTypeJFR, detector.FileTypeTTop, detector.FileTypeIOStat:
		return true
	default:
		return false
	}
}
