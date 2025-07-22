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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTTop(t *testing.T) {
	t.Run("Parse valid ttop content with two snapshots", func(t *testing.T) {
		sampleContent := `top - 12:02:03 up  3:07,  0 users,  load average: 3.18, 1.16, 0.41
Threads: 262 total,   6 running, 256 sleeping,   0 stopped,   0 zombie
%Cpu(s): 85.7 us,  7.1 sy,  0.0 ni,  5.7 id,  1.4 wa,  0.0 hi,  0.0 si,  0.0 st
MiB Mem :  16008.2 total,  10953.7 free,   3713.5 used,   1341.1 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.  12032.0 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
    997 dremio    20   0 7009048   3.4g  98412 R  87.5  21.9   1:36.52 C2 CompilerThre
    996 dremio    20   0 7009048   3.4g  98412 R  81.2  21.9   1:35.89 C2 CompilerThre
   5190 dremio    20   0 7009064   3.4g  98412 S  18.8  21.9   0:03.83 rbound-command1

top - 12:02:04 up  3:07,  0 users,  load average: 3.18, 1.16, 0.41
Threads: 262 total,   2 running, 260 sleeping,   0 stopped,   0 zombie
%Cpu(s): 75.3 us,  3.2 sy,  0.0 ni, 20.4 id,  0.0 wa,  0.0 hi,  1.0 si,  0.0 st
MiB Mem :  16008.2 total,  10953.7 free,   3713.5 used,   1341.1 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.  12032.0 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
    996 dremio    20   0 7008232   3.4g  98412 S  82.2  21.9   1:36.72 C2 CompilerThre
    997 dremio    20   0 7008232   3.4g  98412 R  82.2  21.9   1:37.35 C2 CompilerThre
    998 dremio    20   0 7008232   3.4g  98412 S  14.9  21.9   0:36.57 C1 CompilerThre
   5715 dremio    20   0 7008416   3.4g  98412 R   9.9  21.9   0:04.59 1927b3c3-3473-d`

		data, err := ParseTTop([]byte(sampleContent))
		require.NoError(t, err)
		require.NotNil(t, data)

		// Should have exactly 2 snapshots
		assert.Len(t, data.Snapshots, 2)

		// Check first snapshot
		snapshot1 := data.Snapshots[0]
		assert.Equal(t, 12, snapshot1.Timestamp.Hour())
		assert.Equal(t, 2, snapshot1.Timestamp.Minute())
		assert.Equal(t, 3, snapshot1.Timestamp.Second())
		assert.Len(t, snapshot1.Threads, 3) // 3 threads in first snapshot

		// Check thread counts
		require.NotNil(t, snapshot1.ThreadCounts)
		assert.Equal(t, 262, snapshot1.ThreadCounts.Total)
		assert.Equal(t, 6, snapshot1.ThreadCounts.Running)
		assert.Equal(t, 256, snapshot1.ThreadCounts.Sleeping)
		assert.Equal(t, 0, snapshot1.ThreadCounts.Stopped)
		assert.Equal(t, 0, snapshot1.ThreadCounts.Zombie)

		// Check system memory
		require.NotNil(t, snapshot1.SystemMemory)
		assert.Equal(t, 16008.2, snapshot1.SystemMemory.MemTotal)
		assert.Equal(t, 10953.7, snapshot1.SystemMemory.MemFree)
		assert.Equal(t, 3713.5, snapshot1.SystemMemory.MemUsed)
		assert.Equal(t, 1341.1, snapshot1.SystemMemory.MemBuffCache)
		assert.Equal(t, 0.0, snapshot1.SystemMemory.SwapTotal)
		assert.Equal(t, 0.0, snapshot1.SystemMemory.SwapFree)
		assert.Equal(t, 0.0, snapshot1.SystemMemory.SwapUsed)
		assert.Equal(t, 12032.0, snapshot1.SystemMemory.MemAvail)

		// Check first thread in first snapshot
		thread1 := snapshot1.Threads[0]
		assert.Equal(t, 997, thread1.PID)
		assert.Equal(t, "dremio", thread1.User)
		assert.Equal(t, 87.5, thread1.CPU)
		assert.Equal(t, 21.9, thread1.MEM)
		assert.Equal(t, "C2 CompilerThre", thread1.Command)

		// Check second snapshot
		snapshot2 := data.Snapshots[1]
		assert.Equal(t, 12, snapshot2.Timestamp.Hour())
		assert.Equal(t, 2, snapshot2.Timestamp.Minute())
		assert.Equal(t, 4, snapshot2.Timestamp.Second())
		assert.Len(t, snapshot2.Threads, 4) // 4 threads in second snapshot

		// Check thread counts for second snapshot
		require.NotNil(t, snapshot2.ThreadCounts)
		assert.Equal(t, 262, snapshot2.ThreadCounts.Total)
		assert.Equal(t, 2, snapshot2.ThreadCounts.Running)
		assert.Equal(t, 260, snapshot2.ThreadCounts.Sleeping)
		assert.Equal(t, 0, snapshot2.ThreadCounts.Stopped)
		assert.Equal(t, 0, snapshot2.ThreadCounts.Zombie)

		// Check system memory for second snapshot (should be same as first)
		require.NotNil(t, snapshot2.SystemMemory)
		assert.Equal(t, 16008.2, snapshot2.SystemMemory.MemTotal)
		assert.Equal(t, 10953.7, snapshot2.SystemMemory.MemFree)
		assert.Equal(t, 3713.5, snapshot2.SystemMemory.MemUsed)
		assert.Equal(t, 1341.1, snapshot2.SystemMemory.MemBuffCache)
		assert.Equal(t, 0.0, snapshot2.SystemMemory.SwapTotal)
		assert.Equal(t, 0.0, snapshot2.SystemMemory.SwapFree)
		assert.Equal(t, 0.0, snapshot2.SystemMemory.SwapUsed)
		assert.Equal(t, 12032.0, snapshot2.SystemMemory.MemAvail)

		// Check first thread in second snapshot
		thread2 := snapshot2.Threads[0]
		assert.Equal(t, 996, thread2.PID)
		assert.Equal(t, "dremio", thread2.User)
		assert.Equal(t, 82.2, thread2.CPU)
		assert.Equal(t, 21.9, thread2.MEM)
		assert.Equal(t, "C2 CompilerThre", thread2.Command)
	})

	t.Run("Parse empty content", func(t *testing.T) {
		data, err := ParseTTop([]byte(""))
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Empty(t, data.Snapshots)
	})

	t.Run("Parse content with no valid snapshots", func(t *testing.T) {
		content := `Some random content
That doesn't match
The expected format`

		data, err := ParseTTop([]byte(content))
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Empty(t, data.Snapshots)
	})

	t.Run("Parse content with malformed thread lines", func(t *testing.T) {
		content := `top - 12:02:03 up  3:07,  0 users,  load average: 3.18, 1.16, 0.41
Threads: 262 total,   6 running, 256 sleeping,   0 stopped,   0 zombie

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
    997 dremio    20   0 7009048   3.4g  98412 R  87.5  21.9   1:36.52 C2 CompilerThre
    invalid line with not enough fields
    996 dremio    20   0 7009048   3.4g  98412 R  81.2  21.9   1:35.89 C2 CompilerThre`

		data, err := ParseTTop([]byte(content))
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Len(t, data.Snapshots, 1)

		// Should have 2 valid threads (malformed line should be ignored)
		assert.Len(t, data.Snapshots[0].Threads, 2)
	})

	t.Run("Parse content with single snapshot", func(t *testing.T) {
		content := `top - 15:30:45 up 1 day,  5:23,  2 users,  load average: 1.23, 1.45, 1.67
Threads: 100 total,   2 running, 98 sleeping,   0 stopped,   0 zombie

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
   1234 root      20   0  123456  12345   1234 R  25.5  10.2   0:30.12 test-process`

		data, err := ParseTTop([]byte(content))
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Len(t, data.Snapshots, 1)

		snapshot := data.Snapshots[0]
		assert.Equal(t, 15, snapshot.Timestamp.Hour())
		assert.Equal(t, 30, snapshot.Timestamp.Minute())
		assert.Equal(t, 45, snapshot.Timestamp.Second())
		assert.Len(t, snapshot.Threads, 1)

		thread := snapshot.Threads[0]
		assert.Equal(t, 1234, thread.PID)
		assert.Equal(t, "root", thread.User)
		assert.Equal(t, 25.5, thread.CPU)
		assert.Equal(t, 10.2, thread.MEM)
		assert.Equal(t, "test-process", thread.Command)
	})
}

func TestParseTimestampFromTopLine(t *testing.T) {
	t.Run("Valid timestamp parsing", func(t *testing.T) {
		line := "top - 12:02:03 up  3:07,  0 users,  load average: 3.18, 1.16, 0.41"
		timestamp, err := parseTimestampFromTopLine(line)
		require.NoError(t, err)

		assert.Equal(t, 12, timestamp.Hour())
		assert.Equal(t, 2, timestamp.Minute())
		assert.Equal(t, 3, timestamp.Second())
	})

	t.Run("Invalid timestamp format", func(t *testing.T) {
		line := "top - invalid"
		_, err := parseTimestampFromTopLine(line)
		require.Error(t, err)
	})

	t.Run("Missing timestamp", func(t *testing.T) {
		line := "top -"
		_, err := parseTimestampFromTopLine(line)
		require.Error(t, err)
	})
}

func TestParseThreadLine(t *testing.T) {
	t.Run("Valid thread line parsing", func(t *testing.T) {
		line := "    997 dremio    20   0 7009048   3.4g  98412 R  87.5  21.9   1:36.52 C2 CompilerThre"
		thread, err := parseThreadLine(line)
		require.NoError(t, err)

		assert.Equal(t, 997, thread.PID)
		assert.Equal(t, "dremio", thread.User)
		assert.Equal(t, 87.5, thread.CPU)
		assert.Equal(t, 21.9, thread.MEM)
		assert.Equal(t, "C2 CompilerThre", thread.Command)
	})

	t.Run("Thread line with multi-word command", func(t *testing.T) {
		line := "   1234 root      20   0  123456  12345   1234 R  25.5  10.2   0:30.12 java -jar application.jar"
		thread, err := parseThreadLine(line)
		require.NoError(t, err)

		assert.Equal(t, 1234, thread.PID)
		assert.Equal(t, "root", thread.User)
		assert.Equal(t, 25.5, thread.CPU)
		assert.Equal(t, 10.2, thread.MEM)
		assert.Equal(t, "java -jar application.jar", thread.Command)
	})

	t.Run("Invalid thread line - insufficient fields", func(t *testing.T) {
		line := "997 dremio"
		_, err := parseThreadLine(line)
		require.Error(t, err)
	})

	t.Run("Invalid thread line - non-numeric PID", func(t *testing.T) {
		line := "    ABC dremio    20   0 7009048   3.4g  98412 R  87.5  21.9   1:36.52 C2 CompilerThre"
		_, err := parseThreadLine(line)
		require.Error(t, err)
	})

	t.Run("Thread line with invalid CPU/MEM values", func(t *testing.T) {
		line := "    997 dremio    20   0 7009048   3.4g  98412 R  invalid  invalid   1:36.52 C2 CompilerThre"
		thread, err := parseThreadLine(line)
		require.NoError(t, err)

		// Should default to 0.0 for invalid CPU/MEM values
		assert.Equal(t, 997, thread.PID)
		assert.Equal(t, "dremio", thread.User)
		assert.Equal(t, 0.0, thread.CPU)
		assert.Equal(t, 0.0, thread.MEM)
		assert.Equal(t, "C2 CompilerThre", thread.Command)
	})
}

func TestTTopReportDataStructure(t *testing.T) {
	t.Run("Data structure creation and access", func(t *testing.T) {
		// Create test data
		timestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		thread := ThreadInfo{
			PID:     1234,
			User:    "testuser",
			CPU:     25.5,
			MEM:     10.2,
			Command: "test-command",
		}

		snapshot := TTopSnapshot{
			Timestamp:    timestamp,
			ThreadCounts: &ThreadCounts{Total: 1, Running: 1, Sleeping: 0, Stopped: 0, Zombie: 0},
			SystemMemory: &SystemMemory{MemTotal: 1000.0, MemFree: 500.0, MemUsed: 400.0, MemBuffCache: 100.0, SwapTotal: 0.0, SwapFree: 0.0, SwapUsed: 0.0, MemAvail: 600.0},
			Threads:      []ThreadInfo{thread},
		}

		data := TTopReportData{
			Snapshots: []TTopSnapshot{snapshot},
		}

		// Verify structure
		assert.Len(t, data.Snapshots, 1)
		assert.Equal(t, timestamp, data.Snapshots[0].Timestamp)
		assert.Len(t, data.Snapshots[0].Threads, 1)
		assert.Equal(t, thread, data.Snapshots[0].Threads[0])
	})
}

func TestParseThreadCountsLine(t *testing.T) {
	t.Run("Parse valid thread counts line", func(t *testing.T) {
		line := "Threads: 262 total,   6 running, 256 sleeping,   0 stopped,   0 zombie"

		counts, err := parseThreadCountsLine(line)
		require.NoError(t, err)
		require.NotNil(t, counts)

		assert.Equal(t, 262, counts.Total)
		assert.Equal(t, 6, counts.Running)
		assert.Equal(t, 256, counts.Sleeping)
		assert.Equal(t, 0, counts.Stopped)
		assert.Equal(t, 0, counts.Zombie)
	})

	t.Run("Parse thread counts line with different values", func(t *testing.T) {
		line := "Threads: 100 total,   2 running, 95 sleeping,   2 stopped,   1 zombie"

		counts, err := parseThreadCountsLine(line)
		require.NoError(t, err)
		require.NotNil(t, counts)

		assert.Equal(t, 100, counts.Total)
		assert.Equal(t, 2, counts.Running)
		assert.Equal(t, 95, counts.Sleeping)
		assert.Equal(t, 2, counts.Stopped)
		assert.Equal(t, 1, counts.Zombie)
	})

	t.Run("Parse malformed thread counts line", func(t *testing.T) {
		line := "Threads: invalid format"

		counts, err := parseThreadCountsLine(line)
		assert.Error(t, err)
		assert.Nil(t, counts)
	})
}

func TestParseMemoryLine(t *testing.T) {
	t.Run("Parse valid memory line", func(t *testing.T) {
		line := "MiB Mem :  16008.2 total,  10953.7 free,   3713.5 used,   1341.1 buff/cache"
		memory := &SystemMemory{}

		err := parseMemoryLine(line, memory)
		require.NoError(t, err)

		assert.Equal(t, 16008.2, memory.MemTotal)
		assert.Equal(t, 10953.7, memory.MemFree)
		assert.Equal(t, 3713.5, memory.MemUsed)
		assert.Equal(t, 1341.1, memory.MemBuffCache)
	})

	t.Run("Parse malformed memory line", func(t *testing.T) {
		line := "MiB Mem : invalid format"
		memory := &SystemMemory{}

		err := parseMemoryLine(line, memory)
		assert.Error(t, err)
	})
}

func TestParseSwapLine(t *testing.T) {
	t.Run("Parse valid swap line", func(t *testing.T) {
		line := "MiB Swap:      0.0 total,      0.0 free,      0.0 used.  12032.0 avail Mem"
		memory := &SystemMemory{}

		err := parseSwapLine(line, memory)
		require.NoError(t, err)

		assert.Equal(t, 0.0, memory.SwapTotal)
		assert.Equal(t, 0.0, memory.SwapFree)
		assert.Equal(t, 0.0, memory.SwapUsed)
		assert.Equal(t, 12032.0, memory.MemAvail)
	})

	t.Run("Parse swap line with non-zero values", func(t *testing.T) {
		line := "MiB Swap:   1024.0 total,    512.0 free,    512.0 used.  8000.0 avail Mem"
		memory := &SystemMemory{}

		err := parseSwapLine(line, memory)
		require.NoError(t, err)

		assert.Equal(t, 1024.0, memory.SwapTotal)
		assert.Equal(t, 512.0, memory.SwapFree)
		assert.Equal(t, 512.0, memory.SwapUsed)
		assert.Equal(t, 8000.0, memory.MemAvail)
	})

	t.Run("Parse malformed swap line", func(t *testing.T) {
		line := "MiB Swap: invalid format"
		memory := &SystemMemory{}

		err := parseSwapLine(line, memory)
		assert.Error(t, err)
	})
}
