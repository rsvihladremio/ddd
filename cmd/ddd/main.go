package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/rsvihladremio/ddd/internal/config"
	"github.com/rsvihladremio/ddd/internal/database"
	"github.com/rsvihladremio/ddd/internal/handlers"
	"github.com/rsvihladremio/ddd/internal/workers"
)

func main() {
	var (
		port              = flag.String("port", "8080", "Server port")
		dbPath            = flag.String("db", "./ddd.db", "SQLite database path")
		uploadsDir        = flag.String("uploads", "./uploads", "Uploads directory")
		maxDiskUsage      = flag.Float64("max-disk-usage", 0.5, "Maximum disk usage percentage (0.0-1.0)")
		fileRetentionDays = flag.Int("file-retention-days", 14, "File retention period in days")
	)
	flag.Parse()

	// Initialize configuration
	cfg := &config.Config{
		Port:              *port,
		DBPath:            *dbPath,
		UploadsDir:        *uploadsDir,
		MaxDiskUsage:      *maxDiskUsage,
		FileRetentionDays: *fileRetentionDays,
	}

	// Create uploads directory if it doesn't exist
	if err := os.MkdirAll(cfg.UploadsDir, 0755); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}

	// Initialize database
	db, err := database.Initialize(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize handlers
	h := handlers.New(db, cfg)

	// Start background workers
	reportWorker := workers.NewReportWorker(db, cfg)
	cleanupWorker := workers.NewCleanupWorker(db, cfg)

	go reportWorker.Start()
	go cleanupWorker.Start()

	// Setup HTTP routes
	mux := http.NewServeMux()

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static/"))))

	// API routes
	mux.HandleFunc("/api/upload", h.HandleUpload)
	mux.HandleFunc("/api/files", h.HandleFiles)
	mux.HandleFunc("/api/files/", h.HandleFileOperations)
	mux.HandleFunc("/api/reports/", h.HandleReports)
	mux.HandleFunc("/api/reports/content/", h.HandleReportContent)

	// Main page
	mux.HandleFunc("/", h.HandleIndex)

	log.Printf("Starting DDD server on port %s", cfg.Port)
	log.Printf("Database: %s", cfg.DBPath)
	log.Printf("Uploads directory: %s", cfg.UploadsDir)
	log.Printf("Max disk usage: %.1f%%", cfg.MaxDiskUsage*100)
	log.Printf("File retention: %d days", cfg.FileRetentionDays)

	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
