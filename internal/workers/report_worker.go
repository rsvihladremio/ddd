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
	"log"
	"time"

	"github.com/rsvihladremio/ddd/internal/config"
	"github.com/rsvihladremio/ddd/internal/database"
	"github.com/rsvihladremio/ddd/internal/reporters"
)

// ReportWorker handles background report generation
type ReportWorker struct {
	db  *database.DB
	cfg *config.Config
}

// NewReportWorker creates a new report worker
func NewReportWorker(db *database.DB, cfg *config.Config) *ReportWorker {
	return &ReportWorker{
		db:  db,
		cfg: cfg,
	}
}

// Start begins the report worker loop
func (w *ReportWorker) Start() {
	log.Println("Starting report worker...")

	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for range ticker.C {
		w.processReports()
	}
}

// processReports processes pending reports
func (w *ReportWorker) processReports() {
	reports, err := w.db.GetPendingReports()
	if err != nil {
		log.Printf("Error getting pending reports: %v", err)
		return
	}

	for _, report := range reports {
		w.processReport(report)
	}
}

// processReport processes a single report
func (w *ReportWorker) processReport(report *database.Report) {
	log.Printf("Processing report %d for file %d (type: %s)", report.ID, report.FileID, report.ReportType)

	// Update status to running
	err := w.db.UpdateReport(report.ID, "running", "", "")
	if err != nil {
		log.Printf("Error updating report status: %v", err)
		return
	}

	// Get file information
	file, err := w.getFileByID(report.FileID)
	if err != nil {
		log.Printf("Error getting file %d: %v", report.FileID, err)
		if err := w.db.UpdateReport(report.ID, "failed", "", "File not found"); err != nil {
			log.Printf("Error updating report status to failed: %v", err)
		}
		return
	}

	// Generate report based on type
	var reportData string
	var reportErr error

	switch report.ReportType {
	case "ttop":
		reportData, reportErr = reporters.GenerateTTopReport(file.FilePath)
	case "iostat":
		reportData, reportErr = reporters.GenerateIOStatReport(file.FilePath)
	case "jfr":
		reportData, reportErr = reporters.GenerateJFRReport(file.FilePath)
	default:
		reportErr = fmt.Errorf("unknown report type: %s", report.ReportType)
	}

	// Update report with results
	if reportErr != nil {
		log.Printf("Error generating report: %v", reportErr)
		if err := w.db.UpdateReport(report.ID, "failed", "", reportErr.Error()); err != nil {
			log.Printf("Error updating report status to failed: %v", err)
		}
	} else {
		log.Printf("Report %d completed successfully", report.ID)
		if err := w.db.UpdateReport(report.ID, "completed", reportData, ""); err != nil {
			log.Printf("Error updating report status to completed: %v", err)
		}
	}
}

// getFileByID retrieves a file by ID (helper method)
func (w *ReportWorker) getFileByID(fileID int) (*database.File, error) {
	// This is a simplified implementation - in a real app you'd add this method to the DB
	// For now, we'll implement a basic query
	query := `
		SELECT id, hash, original_name, file_type, file_size, upload_time, file_path, deleted, deleted_time
		FROM files WHERE id = ?
	`
	row := w.db.QueryRow(query, fileID)

	file := &database.File{}
	err := row.Scan(&file.ID, &file.Hash, &file.OriginalName, &file.FileType,
		&file.FileSize, &file.UploadTime, &file.FilePath, &file.Deleted, &file.DeletedTime)
	if err != nil {
		return nil, err
	}
	return file, nil
}
