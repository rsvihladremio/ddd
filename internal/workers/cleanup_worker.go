package workers

import (
	"log"
	"os"
	"syscall"
	"time"

	"github.com/rsvihladremio/ddd/internal/config"
	"github.com/rsvihladremio/ddd/internal/database"
)

// CleanupWorker handles background file cleanup
type CleanupWorker struct {
	db  *database.DB
	cfg *config.Config
}

// NewCleanupWorker creates a new cleanup worker
func NewCleanupWorker(db *database.DB, cfg *config.Config) *CleanupWorker {
	return &CleanupWorker{
		db:  db,
		cfg: cfg,
	}
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
		}
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

	// Check if cleanup is needed
	needsCleanup := diskUsage > w.cfg.MaxDiskUsage

	// Get files that are candidates for deletion
	cutoffTime := time.Now().AddDate(0, 0, -w.cfg.FileRetentionDays)
	files, err := w.getFilesForCleanup(cutoffTime, needsCleanup)
	if err != nil {
		log.Printf("Error getting files for cleanup: %v", err)
		return
	}

	if len(files) == 0 {
		log.Println("No files need cleanup")
		return
	}

	log.Printf("Found %d files for cleanup", len(files))

	// Delete files
	for _, file := range files {
		if err := w.deleteFile(file); err != nil {
			log.Printf("Error deleting file %s: %v", file.FilePath, err)
		} else {
			log.Printf("Deleted file: %s (original: %s)", file.FilePath, file.OriginalName)
		}
	}

	log.Println("File cleanup process completed")
}

// cleanupOldFiles performs cleanup of old files based on retention policy
func (w *CleanupWorker) cleanupOldFiles() {
	cutoffTime := time.Now().Add(-time.Duration(w.cfg.FileRetentionDays) * 24 * time.Hour)

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
}

// getDiskUsage calculates current disk usage percentage
func (w *CleanupWorker) getDiskUsage() (float64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(w.cfg.UploadsDir, &stat)
	if err != nil {
		return 0, err
	}

	// Calculate usage percentage
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	return float64(used) / float64(total), nil
}

// getFilesForCleanup retrieves files that should be cleaned up
func (w *CleanupWorker) getFilesForCleanup(cutoffTime time.Time, forceCleanup bool) ([]*database.File, error) {
	// For now, both cases use the same logic - files older than cutoff time
	// In the future, we could implement more sophisticated cleanup policies
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
