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
	"log"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/rsvihladremio/ddd/internal/config"
	"github.com/rsvihladremio/ddd/internal/database"
)

// CleanupWorker handles background file cleanup
type CleanupWorker struct {
	db          *database.DB
	cfg         *config.Config
	triggerChan chan struct{}
}

// NewCleanupWorker creates a new cleanup worker
func NewCleanupWorker(db *database.DB, cfg *config.Config) *CleanupWorker {
	return &CleanupWorker{
		db:          db,
		cfg:         cfg,
		triggerChan: make(chan struct{}, 1), // Buffered channel to avoid blocking
	}
}

// getMaxDiskUsage retrieves max disk usage setting from database
func (w *CleanupWorker) getMaxDiskUsage() (float64, error) {
	value, err := w.db.GetSetting("max_disk_usage")
	if err != nil {
		// Fall back to config if setting not found
		return w.cfg.MaxDiskUsage, nil
	}
	return strconv.ParseFloat(value, 64)
}

// getFileRetentionDays retrieves file retention days setting from database
func (w *CleanupWorker) getFileRetentionDays() (int, error) {
	value, err := w.db.GetSetting("file_retention_days")
	if err != nil {
		// Fall back to config if setting not found
		return w.cfg.FileRetentionDays, nil
	}
	return strconv.Atoi(value)
}

// Start begins the cleanup worker loop
func (w *CleanupWorker) Start() {
	log.Println("Starting cleanup worker...")

	ticker := time.NewTicker(1 * time.Hour) // Check every hour
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.performCleanup()
		case <-w.triggerChan:
			log.Println("Cleanup triggered by settings change")
			w.performCleanup()
		}
	}
}

// TriggerCleanup triggers an immediate cleanup (non-blocking)
func (w *CleanupWorker) TriggerCleanup() {
	select {
	case w.triggerChan <- struct{}{}:
		// Successfully triggered
	default:
		// Channel is full, cleanup is already pending
	}
}

// performCleanup performs file cleanup based on configured policies
func (w *CleanupWorker) performCleanup() {
	log.Println("Starting file cleanup process...")

	// Get disk usage
	diskUsage, err := w.getDiskUsage()
	if err != nil {
		log.Printf("Error getting disk usage: %v", err)
		return
	}

	log.Printf("Current disk usage: %.2f%%", diskUsage*100)

	// Get settings from database
	maxDiskUsage, err := w.getMaxDiskUsage()
	if err != nil {
		log.Printf("Error getting max disk usage setting: %v", err)
		maxDiskUsage = w.cfg.MaxDiskUsage // fallback
	}

	fileRetentionDays, err := w.getFileRetentionDays()
	if err != nil {
		log.Printf("Error getting file retention days setting: %v", err)
		fileRetentionDays = w.cfg.FileRetentionDays // fallback
	}

	// Check if cleanup is needed
	needsCleanup := diskUsage > maxDiskUsage

	log.Printf("Cleanup check: disk usage %.2f%% vs threshold %.2f%% - cleanup needed: %v",
		diskUsage*100, maxDiskUsage*100, needsCleanup)

	if !needsCleanup {
		// Also clean up files older than retention period even if under threshold
		log.Printf("Disk usage under threshold, but checking for files older than %d days", fileRetentionDays)
		w.cleanupOldFiles()
		return
	}

	// We're over the threshold - delete oldest files until we're under threshold
	log.Printf("Disk usage %.2f%% exceeds threshold %.2f%% - starting aggressive cleanup",
		diskUsage*100, maxDiskUsage*100)

	deletedCount := 0
	currentDiskUsage := diskUsage

	for currentDiskUsage > maxDiskUsage {
		// Get oldest files (limit to batches to avoid memory issues)
		files, err := w.getOldestFiles(50) // Get 50 oldest files at a time
		if err != nil {
			log.Printf("Error getting oldest files: %v", err)
			return
		}

		if len(files) == 0 {
			log.Println("No more files to delete, but still over threshold")
			break
		}

		log.Printf("Found %d oldest files to delete", len(files))

		// Delete files one by one and check disk usage
		for _, file := range files {
			if err := w.deleteFile(file); err != nil {
				log.Printf("Error deleting file %s: %v", file.FilePath, err)
				continue
			}

			deletedCount++
			log.Printf("Deleted file: %s (original: %s)", file.FilePath, file.OriginalName)

			// Check disk usage after every few deletions
			if deletedCount%5 == 0 {
				newDiskUsage, err := w.getDiskUsage()
				if err != nil {
					log.Printf("Error checking disk usage: %v", err)
					continue
				}
				currentDiskUsage = newDiskUsage
				log.Printf("After deleting %d files: disk usage now %.2f%%", deletedCount, currentDiskUsage*100)

				if currentDiskUsage <= maxDiskUsage {
					log.Printf("Disk usage now below threshold (%.2f%% <= %.2f%%), stopping cleanup",
						currentDiskUsage*100, maxDiskUsage*100)
					break
				}
			}
		}
	}

	// Final disk usage check
	finalDiskUsage, err := w.getDiskUsage()
	if err == nil {
		log.Printf("Cleanup completed: deleted %d files, disk usage: %.2f%% -> %.2f%%",
			deletedCount, diskUsage*100, finalDiskUsage*100)
	} else {
		log.Printf("Cleanup completed: deleted %d files", deletedCount)
	}

	// Clean up deleted file entries that have no reports
	w.cleanupOrphanedFileEntries()
}

// cleanupOldFiles performs cleanup of old files based on retention policy
func (w *CleanupWorker) cleanupOldFiles() {
	// Get file retention days from database
	fileRetentionDays, err := w.getFileRetentionDays()
	if err != nil {
		log.Printf("Error getting file retention days setting: %v", err)
		fileRetentionDays = w.cfg.FileRetentionDays // fallback
	}

	cutoffTime := time.Now().Add(-time.Duration(fileRetentionDays) * 24 * time.Hour)

	files, err := w.getFilesForCleanup(cutoffTime, false)
	if err != nil {
		log.Printf("Error getting files for cleanup: %v", err)
		return
	}

	for _, file := range files {
		if err := w.deleteFile(file); err != nil {
			log.Printf("Error deleting file %s: %v", file.FilePath, err)
		}
	}

	// Clean up deleted file entries that have no reports
	w.cleanupOrphanedFileEntries()
}

// getDiskUsage calculates current disk usage percentage
func (w *CleanupWorker) getDiskUsage() (float64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(w.cfg.UploadsDir, &stat)
	if err != nil {
		return 0, err
	}

	// Calculate usage percentage
	// Convert Bsize to uint64 - gosec G115 is acceptable here as Bsize represents block size
	// which is always positive in valid filesystem contexts
	blockSize := uint64(stat.Bsize) // #nosec G115
	total := stat.Blocks * blockSize
	free := stat.Bavail * blockSize
	used := total - free

	return float64(used) / float64(total), nil
}

// getFilesForCleanup retrieves files that should be cleaned up
func (w *CleanupWorker) getFilesForCleanup(cutoffTime time.Time, forceCleanup bool) ([]*database.File, error) {
	// For now, both cases use the same logic - files older than cutoff time
	// In the future, we could implement more sophisticated cleanup policies
	// The forceCleanup parameter is reserved for future use
	_ = forceCleanup // TODO: implement force cleanup logic
	return w.db.GetFilesOlderThan(cutoffTime)
}

// deleteFile deletes a file from disk and marks it as deleted in the database
func (w *CleanupWorker) deleteFile(file *database.File) error {
	// Delete physical file
	if err := os.Remove(file.FilePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Mark as deleted in database
	return w.db.MarkFileDeleted(file.ID)
}

// getOldestFiles retrieves the oldest files from the database
func (w *CleanupWorker) getOldestFiles(limit int) ([]*database.File, error) {
	query := `
		SELECT id, original_name, file_path, hash, file_type, file_size, upload_time, deleted
		FROM files
		WHERE deleted = 0
		ORDER BY upload_time ASC
		LIMIT ?
	`

	rows, err := w.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	var files []*database.File
	for rows.Next() {
		var file database.File
		if err := rows.Scan(&file.ID, &file.OriginalName, &file.FilePath, &file.Hash,
			&file.FileType, &file.FileSize, &file.UploadTime, &file.Deleted); err != nil {
			return nil, err
		}
		files = append(files, &file)
	}

	return files, rows.Err()
}

// cleanupOrphanedFileEntries removes deleted file entries that have no reports
func (w *CleanupWorker) cleanupOrphanedFileEntries() {
	log.Println("Checking for orphaned file entries (deleted files with no reports)...")

	// Find deleted files that have no reports
	query := `
		SELECT f.id, f.original_name, f.file_path
		FROM files f
		LEFT JOIN reports r ON f.id = r.file_id
		WHERE f.deleted = 1 AND r.file_id IS NULL
	`

	rows, err := w.db.Query(query)
	if err != nil {
		log.Printf("Error querying for orphaned file entries: %v", err)
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	var orphanedFiles []struct {
		ID           int
		OriginalName string
		FilePath     string
	}

	for rows.Next() {
		var file struct {
			ID           int
			OriginalName string
			FilePath     string
		}
		if err := rows.Scan(&file.ID, &file.OriginalName, &file.FilePath); err != nil {
			log.Printf("Error scanning orphaned file row: %v", err)
			continue
		}
		orphanedFiles = append(orphanedFiles, file)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating orphaned file rows: %v", err)
		return
	}

	if len(orphanedFiles) == 0 {
		log.Println("No orphaned file entries found")
		return
	}

	log.Printf("Found %d orphaned file entries to remove", len(orphanedFiles))

	// Remove each orphaned file entry
	removedCount := 0
	for _, file := range orphanedFiles {
		err := w.db.DeleteFileCompletely(file.ID)
		if err != nil {
			log.Printf("Error removing orphaned file entry %d (%s): %v", file.ID, file.OriginalName, err)
		} else {
			log.Printf("Removed orphaned file entry %d (%s)", file.ID, file.OriginalName)
			removedCount++
		}
	}

	log.Printf("Orphaned file cleanup completed: removed %d file entries", removedCount)
}
