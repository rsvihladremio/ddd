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
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CPUStats represents CPU utilization statistics from iostat
type CPUStats struct {
	User   float64 `json:"user"`   // %user - CPU utilization for user level
	Nice   float64 `json:"nice"`   // %nice - CPU utilization for user level with nice priority
	System float64 `json:"system"` // %system - CPU utilization for system level
	IOWait float64 `json:"iowait"` // %iowait - CPU utilization for I/O wait
	Steal  float64 `json:"steal"`  // %steal - CPU utilization for involuntary wait
	Idle   float64 `json:"idle"`   // %idle - CPU utilization for idle time
}

// DeviceStats represents I/O statistics for a single device
type DeviceStats struct {
	Device               string  `json:"device"`                   // Device name (e.g., sda)
	ReadsPerS            float64 `json:"reads_per_s"`              // r/s - reads per second
	ReadKBPerS           float64 `json:"read_kb_per_s"`            // rkB/s - kilobytes read per second
	ReadReqMergedPerS    float64 `json:"read_req_merged_per_s"`    // rrqm/s - read requests merged per second
	ReadReqMergedPct     float64 `json:"read_req_merged_pct"`      // %rrqm - percentage of read requests merged
	ReadAwait            float64 `json:"read_await"`               // r_await - average time for read requests
	ReadReqSize          float64 `json:"read_req_size"`            // rareq-sz - average size of read requests
	WritesPerS           float64 `json:"writes_per_s"`             // w/s - writes per second
	WriteKBPerS          float64 `json:"write_kb_per_s"`           // wkB/s - kilobytes written per second
	WriteReqMergedPerS   float64 `json:"write_req_merged_per_s"`   // wrqm/s - write requests merged per second
	WriteReqMergedPct    float64 `json:"write_req_merged_pct"`     // %wrqm - percentage of write requests merged
	WriteAwait           float64 `json:"write_await"`              // w_await - average time for write requests
	WriteReqSize         float64 `json:"write_req_size"`           // wareq-sz - average size of write requests
	DiscardsPerS         float64 `json:"discards_per_s"`           // d/s - discards per second
	DiscardKBPerS        float64 `json:"discard_kb_per_s"`         // dkB/s - kilobytes discarded per second
	DiscardReqMergedPerS float64 `json:"discard_req_merged_per_s"` // drqm/s - discard requests merged per second
	DiscardReqMergedPct  float64 `json:"discard_req_merged_pct"`   // %drqm - percentage of discard requests merged
	DiscardAwait         float64 `json:"discard_await"`            // d_await - average time for discard requests
	DiscardReqSize       float64 `json:"discard_req_size"`         // dareq-sz - average size of discard requests
	FlushesPerS          float64 `json:"flushes_per_s"`            // f/s - flushes per second
	FlushAwait           float64 `json:"flush_await"`              // f_await - average time for flush requests
	AvgQueueSize         float64 `json:"avg_queue_size"`           // aqu-sz - average queue size
	Utilization          float64 `json:"utilization"`              // %util - device utilization percentage
}

// IOStatSnapshot represents a single snapshot of system I/O statistics
type IOStatSnapshot struct {
	Timestamp time.Time     `json:"timestamp"` // When this snapshot was taken
	CPUStats  *CPUStats     `json:"cpu_stats"` // CPU utilization statistics
	Devices   []DeviceStats `json:"devices"`   // I/O statistics for all devices
}

// IOStatReportData represents the complete parsed iostat report data
type IOStatReportData struct {
	SystemInfo string           `json:"system_info"` // System information from header
	Snapshots  []IOStatSnapshot `json:"snapshots"`   // All snapshots from the iostat output
}

// ParseIOStat parses iostat output content and extracts I/O statistics over time
func ParseIOStat(content []byte) (*IOStatReportData, error) {
	if len(content) == 0 {
		return &IOStatReportData{Snapshots: []IOStatSnapshot{}}, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	var snapshots []IOStatSnapshot
	var currentSnapshot *IOStatSnapshot
	var systemInfo string
	var inDeviceSection bool
	var lineNumber int
	var expectingCPUStats bool

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Capture system information from the first line
		if systemInfo == "" && strings.Contains(line, "Linux") {
			systemInfo = line
			continue
		}

		// Check if this line contains a timestamp (MM/DD/YY format)
		if isTimestampLine(line) {
			// Validate previous snapshot if it exists
			if currentSnapshot != nil {
				if expectingCPUStats {
					return nil, fmt.Errorf("line %d: expected CPU statistics after timestamp, but found another timestamp", lineNumber)
				}
				snapshots = append(snapshots, *currentSnapshot)
			}

			// Parse timestamp
			timestamp, err := parseIOStatTimestamp(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: failed to parse timestamp: %w", lineNumber, err)
			}

			// Start new snapshot
			currentSnapshot = &IOStatSnapshot{
				Timestamp: timestamp,
				CPUStats:  nil,
				Devices:   []DeviceStats{},
			}
			inDeviceSection = false
			expectingCPUStats = true
			continue
		}

		// Check if this line contains CPU statistics header
		if strings.HasPrefix(line, "avg-cpu:") && currentSnapshot != nil {
			if !expectingCPUStats {
				return nil, fmt.Errorf("line %d: unexpected CPU statistics header", lineNumber)
			}
			inDeviceSection = false
			continue // Skip the header line
		}

		// Parse CPU statistics line (the line after avg-cpu header)
		if expectingCPUStats && currentSnapshot != nil && currentSnapshot.CPUStats == nil && !strings.HasPrefix(line, "Device") && !strings.HasPrefix(line, "avg-cpu:") && !isTimestampLine(line) {
			// Try to parse as CPU stats - should have 6 numeric fields
			fields := strings.Fields(line)
			if len(fields) == 6 {
				cpuStats, err := parseCPUStatsLine(line)
				if err != nil {
					return nil, fmt.Errorf("line %d: failed to parse CPU statistics: %w", lineNumber, err)
				}
				currentSnapshot.CPUStats = cpuStats
				expectingCPUStats = false
			} else if len(fields) > 0 {
				// Non-empty line that doesn't have 6 fields when we expect CPU stats
				return nil, fmt.Errorf("line %d: expected CPU statistics with 6 fields, got %d fields", lineNumber, len(fields))
			}
			continue
		}

		// Check if this line is the device header
		if strings.HasPrefix(line, "Device") {
			inDeviceSection = true
			continue
		}

		// Parse device statistics
		if inDeviceSection && currentSnapshot != nil {
			deviceStats, err := parseDeviceStatsLine(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: failed to parse device statistics: %w", lineNumber, err)
			}
			currentSnapshot.Devices = append(currentSnapshot.Devices, deviceStats)
		}
	}

	// Validate the last snapshot
	if currentSnapshot != nil {
		if expectingCPUStats {
			return nil, fmt.Errorf("line %d: expected CPU statistics after timestamp, but reached end of file", lineNumber)
		}
		snapshots = append(snapshots, *currentSnapshot)
	}

	return &IOStatReportData{
		SystemInfo: systemInfo,
		Snapshots:  snapshots,
	}, nil
}

// isTimestampLine checks if a line contains a timestamp in MM/DD/YY format
func isTimestampLine(line string) bool {
	// Look for pattern like "09/04/24 12:07:20"
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return false
	}

	// Check if first part looks like MM/DD/YY
	datePart := parts[0]
	if len(datePart) == 8 && strings.Count(datePart, "/") == 2 {
		return true
	}

	return false
}

// parseIOStatTimestamp parses timestamp from a line like "09/04/24 12:07:20"
func parseIOStatTimestamp(line string) (time.Time, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("invalid timestamp line format")
	}

	dateTimeStr := parts[0] + " " + parts[1]

	// Parse MM/DD/YY HH:MM:SS format
	parsedTime, err := time.Parse("01/02/06 15:04:05", dateTimeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp %s: %w", dateTimeStr, err)
	}

	return parsedTime, nil
}

// parseCPUStatsLine parses a line with CPU statistics
func parseCPUStatsLine(line string) (*CPUStats, error) {
	fields := strings.Fields(line)
	if len(fields) != 6 {
		return nil, fmt.Errorf("expected 6 CPU stat fields, got %d", len(fields))
	}

	user, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user CPU: %w", err)
	}

	nice, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse nice CPU: %w", err)
	}

	system, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse system CPU: %w", err)
	}

	iowait, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse iowait CPU: %w", err)
	}

	steal, err := strconv.ParseFloat(fields[4], 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse steal CPU: %w", err)
	}

	idle, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse idle CPU: %w", err)
	}

	return &CPUStats{
		User:   user,
		Nice:   nice,
		System: system,
		IOWait: iowait,
		Steal:  steal,
		Idle:   idle,
	}, nil
}

// parseDeviceStatsLine parses a line with device I/O statistics
func parseDeviceStatsLine(line string) (DeviceStats, error) {
	fields := strings.Fields(line)
	if len(fields) != 23 {
		return DeviceStats{}, fmt.Errorf("expected 23 device stat fields, got %d", len(fields))
	}

	device := fields[0]

	// Parse all numeric fields
	values := make([]float64, 22)
	for i := 1; i < 23; i++ {
		val, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return DeviceStats{}, fmt.Errorf("failed to parse field %d: %w", i, err)
		}
		values[i-1] = val
	}

	return DeviceStats{
		Device:               device,
		ReadsPerS:            values[0],
		ReadKBPerS:           values[1],
		ReadReqMergedPerS:    values[2],
		ReadReqMergedPct:     values[3],
		ReadAwait:            values[4],
		ReadReqSize:          values[5],
		WritesPerS:           values[6],
		WriteKBPerS:          values[7],
		WriteReqMergedPerS:   values[8],
		WriteReqMergedPct:    values[9],
		WriteAwait:           values[10],
		WriteReqSize:         values[11],
		DiscardsPerS:         values[12],
		DiscardKBPerS:        values[13],
		DiscardReqMergedPerS: values[14],
		DiscardReqMergedPct:  values[15],
		DiscardAwait:         values[16],
		DiscardReqSize:       values[17],
		FlushesPerS:          values[18],
		FlushAwait:           values[19],
		AvgQueueSize:         values[20],
		Utilization:          values[21],
	}, nil
}
