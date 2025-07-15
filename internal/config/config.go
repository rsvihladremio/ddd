package config

// Config holds the application configuration
type Config struct {
	Port              string
	DBPath            string
	UploadsDir        string
	MaxDiskUsage      float64 // 0.0 to 1.0
	FileRetentionDays int
}
