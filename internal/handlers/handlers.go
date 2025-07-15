package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rsvihladremio/ddd/internal/config"
	"github.com/rsvihladremio/ddd/internal/database"
	"github.com/rsvihladremio/ddd/internal/detector"
)

const DDDVersion = "1.0.0"

// Handlers contains the HTTP handlers
type Handlers struct {
	db  *database.DB
	cfg *config.Config
}

// New creates a new Handlers instance
func New(db *database.DB, cfg *config.Config) *Handlers {
	return &Handlers{
		db:  db,
		cfg: cfg,
	}
}

// HandleIndex serves the main page
func (h *Handlers) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "./web/index.html")
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
	defer file.Close()

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
		// File already exists, return existing file info
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"file":    existingFile,
			"message": "File already exists",
		})
		return
	}

	// Detect file type
	fileType := detector.DetectFileType(header.Filename, fileContent)

	// Save file to disk
	filePath := filepath.Join(h.cfg.UploadsDir, hash)
	outFile, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"file":    dbFile,
		"message": "File uploaded successfully",
	})
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

	limit := 50 // default
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

	includeDeleted := false
	if includeDeletedStr == "true" {
		includeDeleted = true
	}

	files, err := h.db.GetFiles(limit, offset, includeDeleted)
	if err != nil {
		http.Error(w, "Failed to get files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"files":   files,
	})
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
		err := h.db.MarkFileDeleted(fileID)
		if err != nil {
			http.Error(w, "Failed to delete file", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "File marked as deleted",
		})

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
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"reports": reports,
		})

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
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"report":  report,
			"message": "Report queued for processing",
		})

	case http.MethodDelete:
		// Delete report by report ID
		err := h.db.DeleteReport(id)
		if err != nil {
			http.Error(w, "Failed to delete report", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Report deleted successfully",
		})

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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"report_data": report.ReportData,
	})
}

// shouldAutoGenerateReport determines if we should automatically generate a report for a file type
func (h *Handlers) shouldAutoGenerateReport(fileType string) bool {
	switch fileType {
	case detector.FileTypeJFR, detector.FileTypeTTop, detector.FileTypeIOStat,
		detector.FileTypeDremioProfile, detector.FileTypeQueriesJSON:
		return true
	default:
		return false
	}
}
