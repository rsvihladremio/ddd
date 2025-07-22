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

package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rsvihladremio/ddd/internal/config"
	"github.com/rsvihladremio/ddd/internal/database"
	"github.com/rsvihladremio/ddd/internal/handlers"
	"github.com/rsvihladremio/ddd/internal/workers"
)

func main() {
	var (
		port       = flag.String("port", "8080", "Server port")
		dbPath     = flag.String("db", "./ddd.db", "SQLite database path")
		uploadsDir = flag.String("uploads", "./uploads", "Uploads directory")
	)
	flag.Parse()

	// Initialize configuration
	cfg := &config.Config{
		Port:              *port,
		DBPath:            *dbPath,
		UploadsDir:        *uploadsDir,
		MaxDiskUsage:      0.5, // Default fallback value
		FileRetentionDays: 14,  // Default fallback value
	}

	// Create uploads directory if it doesn't exist
	if err := os.MkdirAll(cfg.UploadsDir, 0750); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}

	// Initialize database
	db, err := database.Initialize(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	// Initialize settings in database with sensible defaults
	defaultSettings := map[string]string{
		"max_disk_usage":      "0.500000", // 50%
		"file_retention_days": "14",       // 14 days
	}
	if err := db.InitializeSettings(defaultSettings); err != nil {
		log.Fatalf("Failed to initialize settings: %v", err)
	}

	// Start background workers
	reportWorker := workers.NewReportWorker(db, cfg)
	cleanupWorker := workers.NewCleanupWorker(db, cfg)

	go reportWorker.Start()
	go cleanupWorker.Start()

	// Initialize handlers with cleanup worker reference
	h := handlers.New(db, cfg, cleanupWorker)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static/"))))

	// API routes
	mux.HandleFunc("/api/upload", h.HandleUpload)
	mux.HandleFunc("/api/files", h.HandleFiles)
	mux.HandleFunc("/api/files/", h.HandleFileOperations)
	mux.HandleFunc("/api/files/{id}/redetect", h.HandleRedetectFileType)
	mux.HandleFunc("/api/reports/", h.HandleReports)
	mux.HandleFunc("/api/reports/content/", h.HandleReportContent)
	mux.HandleFunc("/api/disk-usage", h.HandleDiskUsage)
	mux.HandleFunc("/api/settings", h.HandleSettings)

	// Report viewer page
	mux.HandleFunc("/report/", h.HandleReportPage)

	// Main page
	mux.HandleFunc("/", h.HandleIndex)

	log.Printf("Starting DDD server on port %s", cfg.Port)
	log.Printf("Database: %s", cfg.DBPath)
	log.Printf("Uploads directory: %s", cfg.UploadsDir)
	log.Printf("Settings are managed in database and configurable via web UI")

	// Create HTTP server with timeouts for security
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
