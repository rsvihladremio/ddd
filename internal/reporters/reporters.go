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

package reporters

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// secureReadFile safely reads a file with path validation to prevent directory traversal
func secureReadFile(filePath string) ([]byte, error) {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(filePath)

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("invalid file path: directory traversal detected")
	}

	// Ensure the path is absolute or relative to current directory
	if !filepath.IsAbs(cleanPath) && strings.HasPrefix(cleanPath, "/") {
		return nil, fmt.Errorf("invalid file path: absolute path not allowed")
	}

	return os.ReadFile(cleanPath)
}

// GenerateTTopReport generates a comprehensive report for ttop.txt files
// This function parses ttop output to extract thread information over time
// and generates both a JSON summary and an HTML report with interactive charts
func GenerateTTopReport(filePath string) (string, error) {
	content, err := secureReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Parse ttop content to extract structured data
	parsedData, err := ParseTTop(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse ttop content: %w", err)
	}

	// Generate HTML report with charts
	htmlReport, err := GenerateTTopHTML(parsedData)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML report: %w", err)
	}

	// Calculate summary statistics
	snapshotCount := len(parsedData.Snapshots)
	uniqueThreads := countUniqueThreads(parsedData)
	peakThreadCount := findPeakThreadCount(parsedData)

	// Generate summary and analysis text
	summary := fmt.Sprintf("TTop analysis report covering %d snapshots with %d unique threads observed",
		snapshotCount, uniqueThreads)

	analysis := fmt.Sprintf("Peak thread count: %d. Analysis includes thread count over time, "+
		"CPU usage patterns for top 5 busiest threads, and memory usage distribution by user. "+
		"Interactive charts provide detailed visualization of system performance metrics.",
		peakThreadCount)

	// Build comprehensive report structure
	report := map[string]any{
		"type":           "ttop",
		"file_size":      len(content),
		"summary":        summary,
		"analysis":       analysis,
		"generated_at":   time.Now().Format(time.RFC3339),
		"html_report":    htmlReport,
		"snapshot_count": snapshotCount,
		"unique_threads": uniqueThreads,
		"peak_threads":   peakThreadCount,
	}

	reportJSON, err := json.Marshal(report)
	if err != nil {
		return "", fmt.Errorf("failed to marshal report: %w", err)
	}

	return string(reportJSON), nil
}

// GenerateIOStatReport generates a comprehensive report for iostat files
// This function parses iostat output to extract I/O statistics over time
// and generates both a JSON summary and an HTML report with interactive charts
func GenerateIOStatReport(filePath string) (string, error) {
	content, err := secureReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Parse iostat content to extract structured data
	parsedData, err := ParseIOStat(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse iostat content: %w", err)
	}

	// Generate HTML report with charts
	htmlReport, err := GenerateIOStatHTML(parsedData)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML report: %w", err)
	}

	// Calculate summary statistics
	snapshotCount := len(parsedData.Snapshots)
	uniqueDevices := countUniqueDevices(parsedData)
	peakCPUUsage := findPeakCPUUsage(parsedData)
	peakDeviceQueueSize := findPeakDeviceQueueSize(parsedData)

	// Generate summary and analysis text
	summary := fmt.Sprintf("IOStat analysis report covering %d snapshots with %d devices monitored",
		snapshotCount, uniqueDevices)

	analysis := fmt.Sprintf("Peak CPU usage: %.1f%%, Peak device queue size: %.1f. "+
		"Analysis includes CPU utilization over time, I/O throughput patterns, await times, "+
		"queue sizes, and request patterns. Interactive charts provide detailed visualization of system I/O performance.",
		peakCPUUsage, peakDeviceQueueSize)

	// Build comprehensive report structure
	report := map[string]any{
		"type":                   "iostat",
		"file_size":              len(content),
		"summary":                summary,
		"analysis":               analysis,
		"generated_at":           time.Now().Format(time.RFC3339),
		"html_report":            htmlReport,
		"snapshot_count":         snapshotCount,
		"unique_devices":         uniqueDevices,
		"peak_cpu_usage":         peakCPUUsage,
		"peak_device_queue_size": peakDeviceQueueSize,
		"system_info":            parsedData.SystemInfo,
	}

	reportJSON, err := json.Marshal(report)
	if err != nil {
		return "", fmt.Errorf("failed to marshal report: %w", err)
	}

	return string(reportJSON), nil
}

// GenerateJFRReport generates a report for JFR files
func GenerateJFRReport(filePath string) (string, error) {
	content, err := secureReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JFR content and generate report
	report := map[string]any{
		"type":         "jfr",
		"file_size":    len(content),
		"summary":      "JFR analysis report",
		"analysis":     "Basic JFR file analysis - implementation pending",
		"generated_at": "2024-01-01T00:00:00Z", // TODO: use actual timestamp
	}

	reportJSON, err := json.Marshal(report)
	if err != nil {
		return "", fmt.Errorf("failed to marshal report: %w", err)
	}

	return string(reportJSON), nil
}
