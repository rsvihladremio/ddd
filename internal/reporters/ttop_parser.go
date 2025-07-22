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

// ThreadInfo represents a single thread's information at a specific point in time
type ThreadInfo struct {
	PID     int     `json:"pid"`     // Process ID
	User    string  `json:"user"`    // User running the process
	CPU     float64 `json:"cpu"`     // CPU usage percentage
	MEM     float64 `json:"mem"`     // Memory usage percentage
	Command string  `json:"command"` // Command name/line
}

// ThreadCounts represents the global thread counts from the "Threads:" line
type ThreadCounts struct {
	Total    int `json:"total"`    // Total number of threads
	Running  int `json:"running"`  // Number of running threads
	Sleeping int `json:"sleeping"` // Number of sleeping threads
	Stopped  int `json:"stopped"`  // Number of stopped threads
	Zombie   int `json:"zombie"`   // Number of zombie threads
}

// SystemMemory represents system memory information from "MiB Mem:" and "MiB Swap:" lines
type SystemMemory struct {
	MemTotal     float64 `json:"mem_total"`      // Total memory in MiB
	MemFree      float64 `json:"mem_free"`       // Free memory in MiB
	MemUsed      float64 `json:"mem_used"`       // Used memory in MiB
	MemBuffCache float64 `json:"mem_buff_cache"` // Buffer/cache memory in MiB
	SwapTotal    float64 `json:"swap_total"`     // Total swap in MiB
	SwapFree     float64 `json:"swap_free"`      // Free swap in MiB
	SwapUsed     float64 `json:"swap_used"`      // Used swap in MiB
	MemAvail     float64 `json:"mem_avail"`      // Available memory in MiB
}

// TTopSnapshot represents a single snapshot of thread information with timestamp
type TTopSnapshot struct {
	Timestamp    time.Time     `json:"timestamp"`     // When this snapshot was taken
	ThreadCounts *ThreadCounts `json:"thread_counts"` // Global thread counts from "Threads:" line
	SystemMemory *SystemMemory `json:"system_memory"` // System memory information from "MiB Mem:" and "MiB Swap:" lines
	Threads      []ThreadInfo  `json:"threads"`       // List of threads in this snapshot
}

// TTopReportData represents the complete parsed ttop report data
type TTopReportData struct {
	Snapshots []TTopSnapshot `json:"snapshots"` // All snapshots from the ttop output
}

// ParseTTop parses ttop output content and extracts thread information over time
// The parser looks for lines starting with "top - " to identify snapshot boundaries
// and extracts thread information from subsequent lines that start with a PID (integer)
func ParseTTop(content []byte) (*TTopReportData, error) {
	if len(content) == 0 {
		return &TTopReportData{Snapshots: []TTopSnapshot{}}, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	var snapshots []TTopSnapshot
	var currentSnapshot *TTopSnapshot

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check if this line starts a new snapshot
		if strings.HasPrefix(line, "top - ") {
			// Save previous snapshot if it exists
			if currentSnapshot != nil {
				snapshots = append(snapshots, *currentSnapshot)
			}

			// Parse timestamp from the "top - " line
			timestamp, err := parseTimestampFromTopLine(line)
			if err != nil {
				// If we can't parse timestamp, use current time as fallback
				timestamp = time.Now()
			}

			// Start new snapshot
			currentSnapshot = &TTopSnapshot{
				Timestamp:    timestamp,
				ThreadCounts: nil, // Will be populated when we find the "Threads:" line
				SystemMemory: nil, // Will be populated when we find the "MiB Mem:" and "MiB Swap:" lines
				Threads:      []ThreadInfo{},
			}
			continue
		}

		// Check if this line contains thread counts
		if strings.HasPrefix(line, "Threads:") && currentSnapshot != nil {
			threadCounts, err := parseThreadCountsLine(line)
			if err == nil {
				currentSnapshot.ThreadCounts = threadCounts
			}
			continue
		}

		// Check if this line contains memory information
		if strings.HasPrefix(line, "MiB Mem") && currentSnapshot != nil {
			if currentSnapshot.SystemMemory == nil {
				currentSnapshot.SystemMemory = &SystemMemory{}
			}
			err := parseMemoryLine(line, currentSnapshot.SystemMemory)
			if err != nil {
				// Log error but continue parsing
			}
			continue
		}

		// Check if this line contains swap information
		if strings.HasPrefix(line, "MiB Swap") && currentSnapshot != nil {
			if currentSnapshot.SystemMemory == nil {
				currentSnapshot.SystemMemory = &SystemMemory{}
			}
			err := parseSwapLine(line, currentSnapshot.SystemMemory)
			if err != nil {
				// Log error but continue parsing
			}
			continue
		}

		// If we're in a snapshot, try to parse thread information
		if currentSnapshot != nil {
			threadInfo, err := parseThreadLine(line)
			if err == nil {
				currentSnapshot.Threads = append(currentSnapshot.Threads, threadInfo)
			}
			// Silently ignore lines that don't parse as thread info
		}
	}

	// Don't forget to add the last snapshot
	if currentSnapshot != nil {
		snapshots = append(snapshots, *currentSnapshot)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading ttop content: %w", err)
	}

	return &TTopReportData{Snapshots: snapshots}, nil
}

// parseTimestampFromTopLine extracts timestamp from a line like "top - 12:02:03 up  3:07,  0 users,  load average: 3.18, 1.16, 0.41"
func parseTimestampFromTopLine(line string) (time.Time, error) {
	// Look for the timestamp pattern HH:MM:SS after "top - "
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return time.Time{}, fmt.Errorf("invalid top line format")
	}

	// The timestamp should be the third field (index 2)
	timeStr := parts[2]

	// Parse HH:MM:SS format
	parsedTime, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp %s: %w", timeStr, err)
	}

	// Since we only have time, not date, we'll use today's date
	// In a real scenario, you might want to handle date parsing differently
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(),
		parsedTime.Hour(), parsedTime.Minute(), parsedTime.Second(), 0, now.Location()), nil
}

// parseThreadCountsLine parses a line like "Threads: 262 total,   6 running, 256 sleeping,   0 stopped,   0 zombie"
func parseThreadCountsLine(line string) (*ThreadCounts, error) {
	// Remove "Threads: " prefix
	line = strings.TrimPrefix(line, "Threads:")
	line = strings.TrimSpace(line)

	// Split by commas to get individual counts
	parts := strings.Split(line, ",")
	if len(parts) != 5 {
		return nil, fmt.Errorf("expected 5 thread count parts, got %d", len(parts))
	}

	counts := &ThreadCounts{}

	// Parse each part
	for _, part := range parts {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}

		count, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		// Determine which type based on the second field
		switch fields[1] {
		case "total":
			counts.Total = count
		case "running":
			counts.Running = count
		case "sleeping":
			counts.Sleeping = count
		case "stopped":
			counts.Stopped = count
		case "zombie":
			counts.Zombie = count
		}
	}

	return counts, nil
}

// parseMemoryLine parses a line like "MiB Mem :  16008.2 total,  10953.7 free,   3713.5 used,   1341.1 buff/cache"
func parseMemoryLine(line string, memory *SystemMemory) error {
	// Remove "MiB Mem :" prefix
	line = strings.TrimPrefix(line, "MiB Mem")
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, ":") {
		line = strings.TrimPrefix(line, ":")
		line = strings.TrimSpace(line)
	}

	// Split by commas to get individual memory values
	parts := strings.Split(line, ",")
	if len(parts) != 4 {
		return fmt.Errorf("expected 4 memory parts, got %d", len(parts))
	}

	// Parse each part
	for _, part := range parts {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}

		value, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}

		// Determine which type based on the second field
		switch fields[1] {
		case "total":
			memory.MemTotal = value
		case "free":
			memory.MemFree = value
		case "used":
			memory.MemUsed = value
		case "buff/cache":
			memory.MemBuffCache = value
		}
	}

	return nil
}

// parseSwapLine parses a line like "MiB Swap:      0.0 total,      0.0 free,      0.0 used.  12032.0 avail Mem"
func parseSwapLine(line string, memory *SystemMemory) error {
	// Remove "MiB Swap:" prefix
	line = strings.TrimPrefix(line, "MiB Swap:")
	line = strings.TrimSpace(line)

	// Look for "avail Mem" to separate swap info from available memory
	availIndex := strings.Index(line, "avail Mem")
	var swapPart string

	if availIndex != -1 {
		// Find the start of the available memory value (work backwards from "avail Mem")
		beforeAvail := line[:availIndex]
		// Find the last space before "avail" to get the number
		fields := strings.Fields(beforeAvail)
		if len(fields) > 0 {
			// The last field should be the available memory value
			availValue := fields[len(fields)-1]
			if value, err := strconv.ParseFloat(availValue, 64); err == nil {
				memory.MemAvail = value
			}
			// Remove the available memory part to get just the swap info
			swapPart = strings.TrimSpace(strings.TrimSuffix(beforeAvail, availValue))
		}
	} else {
		swapPart = line
	}

	// Remove trailing period if present
	swapPart = strings.TrimSuffix(swapPart, ".")
	swapPart = strings.TrimSpace(swapPart)

	// Parse swap information
	parts := strings.Split(swapPart, ",")
	if len(parts) != 3 {
		return fmt.Errorf("expected 3 swap parts, got %d", len(parts))
	}

	// Parse each swap part
	for _, part := range parts {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}

		value, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}

		// Determine which type based on the second field (remove trailing punctuation)
		fieldType := strings.TrimSuffix(fields[1], ".")
		switch fieldType {
		case "total":
			memory.SwapTotal = value
		case "free":
			memory.SwapFree = value
		case "used":
			memory.SwapUsed = value
		}
	}

	return nil
}

// parseThreadLine parses a line that represents thread information
// Expected format: PID USER PR NI VIRT RES SHR S %CPU %MEM TIME+ COMMAND
// We need at least 12 columns to extract all required information
func parseThreadLine(line string) (ThreadInfo, error) {
	fields := strings.Fields(line)

	// Need at least 12 fields to have all the required information
	if len(fields) < 12 {
		return ThreadInfo{}, fmt.Errorf("insufficient fields in thread line")
	}

	// First field should be PID (integer)
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return ThreadInfo{}, fmt.Errorf("invalid PID: %w", err)
	}

	// Second field is USER
	user := fields[1]

	// %CPU is at index 8 (9th column)
	cpu, err := strconv.ParseFloat(fields[8], 64)
	if err != nil {
		// If CPU parsing fails, default to 0.0
		cpu = 0.0
	}

	// %MEM is at index 9 (10th column)
	mem, err := strconv.ParseFloat(fields[9], 64)
	if err != nil {
		// If MEM parsing fails, default to 0.0
		mem = 0.0
	}

	// COMMAND starts at index 11 (12th column) and may span multiple fields
	command := strings.Join(fields[11:], " ")

	return ThreadInfo{
		PID:     pid,
		User:    user,
		CPU:     cpu,
		MEM:     mem,
		Command: command,
	}, nil
}
