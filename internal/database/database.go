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

package database

import (
	"database/sql"
	"log"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

// DB wraps the sql.DB with additional methods
type DB struct {
	*sql.DB
}

// Initialize creates and initializes the SQLite database
func Initialize(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}

	// Create tables
	if err := createTables(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hash TEXT UNIQUE NOT NULL,
		original_name TEXT NOT NULL,
		file_type TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		upload_time DATETIME NOT NULL,
		file_path TEXT NOT NULL,
		deleted BOOLEAN DEFAULT FALSE,
		deleted_time DATETIME
	);

	CREATE TABLE IF NOT EXISTS reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_id INTEGER NOT NULL,
		report_type TEXT NOT NULL,
		status TEXT NOT NULL, -- 'pending', 'running', 'completed', 'failed'
		created_time DATETIME NOT NULL,
		completed_time DATETIME,
		ddd_version TEXT NOT NULL,
		report_data TEXT, -- JSON data
		error_message TEXT,
		FOREIGN KEY (file_id) REFERENCES files(id)
	);

	CREATE TABLE IF NOT EXISTS worker_status (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		worker_type TEXT NOT NULL,
		last_run DATETIME NOT NULL,
		status TEXT NOT NULL,
		message TEXT
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_files_hash ON files(hash);
	CREATE INDEX IF NOT EXISTS idx_files_upload_time ON files(upload_time);
	CREATE INDEX IF NOT EXISTS idx_reports_file_id ON reports(file_id);
	CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);
	`

	_, err := db.Exec(schema)
	return err
}

// File represents a file record in the database
type File struct {
	ID           int        `json:"id"`
	Hash         string     `json:"hash"`
	OriginalName string     `json:"original_name"`
	FileType     string     `json:"file_type"`
	FileSize     int64      `json:"file_size"`
	UploadTime   time.Time  `json:"upload_time"`
	FilePath     string     `json:"file_path"`
	Deleted      bool       `json:"deleted"`
	DeletedTime  *time.Time `json:"deleted_time,omitempty"`
}

// Report represents a report record in the database
type Report struct {
	ID            int        `json:"id"`
	FileID        int        `json:"file_id"`
	ReportType    string     `json:"report_type"`
	Status        string     `json:"status"`
	CreatedTime   time.Time  `json:"created_time"`
	CompletedTime *time.Time `json:"completed_time,omitempty"`
	DDDVersion    string     `json:"ddd_version"`
	ReportData    string     `json:"report_data,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
}

// WorkerStatus represents worker status in the database
type WorkerStatus struct {
	ID         int       `json:"id"`
	WorkerType string    `json:"worker_type"`
	LastRun    time.Time `json:"last_run"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
}

// Setting represents a configuration setting in the database
type Setting struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	UpdatedTime time.Time `json:"updated_time"`
}

// InsertFile inserts a new file record
func (db *DB) InsertFile(file *File) error {
	query := `
		INSERT INTO files (hash, original_name, file_type, file_size, upload_time, file_path)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	result, err := db.Exec(query, file.Hash, file.OriginalName, file.FileType,
		file.FileSize, file.UploadTime, file.FilePath)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	file.ID = int(id)
	return nil
}

// GetFileByHash retrieves a file by its hash
func (db *DB) GetFileByHash(hash string) (*File, error) {
	query := `
		SELECT id, hash, original_name, file_type, file_size, upload_time, file_path, deleted, deleted_time
		FROM files WHERE hash = ?
	`
	row := db.QueryRow(query, hash)

	file := &File{}
	err := row.Scan(&file.ID, &file.Hash, &file.OriginalName, &file.FileType,
		&file.FileSize, &file.UploadTime, &file.FilePath, &file.Deleted, &file.DeletedTime)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// GetFiles retrieves files with optional filters
func (db *DB) GetFiles(limit, offset int, includeDeleted bool, searchQuery string) ([]*File, error) {
	query := `
		SELECT id, hash, original_name, file_type, file_size, upload_time, file_path, deleted, deleted_time
		FROM files
	`
	args := []interface{}{}
	conditions := []string{}

	if !includeDeleted {
		conditions = append(conditions, "deleted = FALSE")
	}

	if searchQuery != "" {
		conditions = append(conditions, "(original_name LIKE ? OR file_type LIKE ? OR hash LIKE ?)")
		searchPattern := "%" + searchQuery + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY upload_time DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	files := make([]*File, 0)
	for rows.Next() {
		file := &File{}
		err := rows.Scan(&file.ID, &file.Hash, &file.OriginalName, &file.FileType,
			&file.FileSize, &file.UploadTime, &file.FilePath, &file.Deleted, &file.DeletedTime)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

// GetFilesCount returns the total count of files matching the search criteria
func (db *DB) GetFilesCount(includeDeleted bool, searchQuery string) (int, error) {
	query := `SELECT COUNT(*) FROM files`
	args := []interface{}{}
	conditions := []string{}

	if !includeDeleted {
		conditions = append(conditions, "deleted = FALSE")
	}

	if searchQuery != "" {
		conditions = append(conditions, "(original_name LIKE ? OR file_type LIKE ? OR hash LIKE ?)")
		searchPattern := "%" + searchQuery + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// MarkFileDeleted marks a file as deleted
func (db *DB) MarkFileDeleted(fileID int) error {
	query := `UPDATE files SET deleted = TRUE, deleted_time = ? WHERE id = ?`
	_, err := db.Exec(query, time.Now(), fileID)
	return err
}

// RestoreFile restores a deleted file by updating its metadata and clearing deleted status
func (db *DB) RestoreFile(fileID int, originalName, fileType string, fileSize int64, filePath string) error {
	query := `
		UPDATE files
		SET deleted = FALSE,
		    deleted_time = NULL,
		    original_name = ?,
		    file_type = ?,
		    file_size = ?,
		    file_path = ?,
		    upload_time = ?
		WHERE id = ?
	`
	_, err := db.Exec(query, originalName, fileType, fileSize, filePath, time.Now(), fileID)
	return err
}

// InsertReport inserts a new report record
func (db *DB) InsertReport(report *Report) error {
	query := `
		INSERT INTO reports (file_id, report_type, status, created_time, ddd_version, report_data, error_message, completed_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := db.Exec(query, report.FileID, report.ReportType, report.Status,
		report.CreatedTime, report.DDDVersion, report.ReportData, report.ErrorMessage, report.CompletedTime)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	report.ID = int(id)
	return nil
}

// UpdateReport updates a report's status and data
func (db *DB) UpdateReport(reportID int, status string, reportData, errorMessage string) error {
	query := `
		UPDATE reports
		SET status = ?, completed_time = ?, report_data = ?, error_message = ?
		WHERE id = ?
	`
	completedTime := time.Now()
	_, err := db.Exec(query, status, completedTime, reportData, errorMessage, reportID)
	return err
}

// GetReportsByFileID retrieves all reports for a file (without report data for efficiency)
func (db *DB) GetReportsByFileID(fileID int) ([]*Report, error) {
	query := `
		SELECT id, file_id, report_type, status, created_time, completed_time,
		       ddd_version, COALESCE(error_message, '') as error_message
		FROM reports WHERE file_id = ? ORDER BY created_time DESC
	`
	rows, err := db.Query(query, fileID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	reports := make([]*Report, 0)
	for rows.Next() {
		report := &Report{}
		err := rows.Scan(&report.ID, &report.FileID, &report.ReportType, &report.Status,
			&report.CreatedTime, &report.CompletedTime, &report.DDDVersion, &report.ErrorMessage)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// GetPendingReports retrieves reports with pending status
func (db *DB) GetPendingReports() ([]*Report, error) {
	query := `
		SELECT id, file_id, report_type, status, created_time, completed_time,
		       ddd_version, COALESCE(report_data, '') as report_data, COALESCE(error_message, '') as error_message
		FROM reports WHERE status = 'pending' ORDER BY created_time ASC
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	reports := make([]*Report, 0)
	for rows.Next() {
		report := &Report{}
		err := rows.Scan(&report.ID, &report.FileID, &report.ReportType, &report.Status,
			&report.CreatedTime, &report.CompletedTime, &report.DDDVersion,
			&report.ReportData, &report.ErrorMessage)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// GetFilesOlderThan retrieves files older than the specified time
func (db *DB) GetFilesOlderThan(cutoffTime time.Time) ([]*File, error) {
	query := `
		SELECT id, hash, original_name, file_type, file_size, upload_time, file_path, deleted, deleted_time
		FROM files
		WHERE deleted = FALSE AND upload_time < ?
		ORDER BY upload_time ASC
	`
	rows, err := db.Query(query, cutoffTime)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	files := make([]*File, 0)
	for rows.Next() {
		file := &File{}
		err := rows.Scan(&file.ID, &file.Hash, &file.OriginalName, &file.FileType,
			&file.FileSize, &file.UploadTime, &file.FilePath, &file.Deleted, &file.DeletedTime)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

// GetReportByID retrieves a specific report by ID
func (db *DB) GetReportByID(reportID int) (*Report, error) {
	query := `
		SELECT id, file_id, report_type, status, created_time, completed_time,
		       ddd_version, COALESCE(report_data, '') as report_data, COALESCE(error_message, '') as error_message
		FROM reports WHERE id = ?
	`
	row := db.QueryRow(query, reportID)

	report := &Report{}
	err := row.Scan(&report.ID, &report.FileID, &report.ReportType, &report.Status,
		&report.CreatedTime, &report.CompletedTime, &report.DDDVersion,
		&report.ReportData, &report.ErrorMessage)
	if err != nil {
		return nil, err
	}
	return report, nil
}

// DeleteReport deletes a report by ID
func (db *DB) DeleteReport(reportID int) error {
	query := `DELETE FROM reports WHERE id = ?`
	_, err := db.Exec(query, reportID)
	return err
}

// GetReportCountByFileID returns the number of reports for a given file
func (db *DB) GetReportCountByFileID(fileID int) (int, error) {
	query := `SELECT COUNT(*) FROM reports WHERE file_id = ?`
	var count int
	err := db.QueryRow(query, fileID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// DeleteFileCompletely removes a file entry completely from the database
func (db *DB) DeleteFileCompletely(fileID int) error {
	query := `DELETE FROM files WHERE id = ?`
	_, err := db.Exec(query, fileID)
	return err
}

// UpdateReportStatus updates only the status of a report
func (db *DB) UpdateReportStatus(reportID int, status string) error {
	return db.UpdateReport(reportID, status, "", "")
}

// CompleteReport marks a report as completed with data
func (db *DB) CompleteReport(reportID int, reportData string) error {
	return db.UpdateReport(reportID, "completed", reportData, "")
}

// FailReport marks a report as failed with error message
func (db *DB) FailReport(reportID int, errorMessage string) error {
	return db.UpdateReport(reportID, "failed", "", errorMessage)
}

// UpdateFileFileType updates the file type for a given file ID
func (db *DB) UpdateFileFileType(fileID int, fileType string) error {
	query := `UPDATE files SET file_type = ?, upload_time = ? WHERE id = ?`
	_, err := db.Exec(query, fileType, time.Now(), fileID)
	return err
}

// GetFileByID retrieves a file by ID
func (db *DB) GetFileByID(fileID int) (*File, error) {
	query := `
		SELECT id, hash, original_name, file_type, file_size, upload_time, file_path, deleted, deleted_time
		FROM files WHERE id = ?
	`
	row := db.QueryRow(query, fileID)

	file := &File{}
	err := row.Scan(&file.ID, &file.Hash, &file.OriginalName, &file.FileType,
		&file.FileSize, &file.UploadTime, &file.FilePath, &file.Deleted, &file.DeletedTime)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// UpdateWorkerStatus updates or inserts worker status
func (db *DB) UpdateWorkerStatus(workerType, status, message string) error {
	query := `
		INSERT OR REPLACE INTO worker_status (worker_type, last_run, status, message)
		VALUES (?, ?, ?, ?)
	`
	_, err := db.Exec(query, workerType, time.Now(), status, message)
	return err
}

// GetSetting retrieves a setting value by key
func (db *DB) GetSetting(key string) (string, error) {
	query := `SELECT value FROM settings WHERE key = ?`
	row := db.QueryRow(query, key)

	var value string
	err := row.Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetSetting sets a setting value by key
func (db *DB) SetSetting(key, value string) error {
	query := `
		INSERT OR REPLACE INTO settings (key, value, updated_time)
		VALUES (?, ?, ?)
	`
	_, err := db.Exec(query, key, value, time.Now())
	return err
}

// GetAllSettings retrieves all settings as a map
func (db *DB) GetAllSettings() (map[string]string, error) {
	query := `SELECT key, value FROM settings`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		settings[key] = value
	}

	return settings, rows.Err()
}

// InitializeSettings sets default settings if they don't exist
func (db *DB) InitializeSettings(defaults map[string]string) error {
	for key, defaultValue := range defaults {
		// Check if setting already exists
		_, err := db.GetSetting(key)
		if err != nil {
			// Setting doesn't exist, create it with default value
			if err := db.SetSetting(key, defaultValue); err != nil {
				return err
			}
			log.Printf("Initialized setting %s with default value: %s", key, defaultValue)
		}
	}
	return nil
}
