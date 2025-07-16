//	Copyright 2023 Dremio Corporation
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

// TTopSnapshot represents a single snapshot of thread information with timestamp
type TTopSnapshot struct {
	Timestamp time.Time    `json:"timestamp"` // When this snapshot was taken
	Threads   []ThreadInfo `json:"threads"`   // List of threads in this snapshot
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
				Timestamp: timestamp,
				Threads:   []ThreadInfo{},
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
