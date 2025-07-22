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

func TestParseIOStat(t *testing.T) {
	t.Run("Parse valid iostat content with multiple snapshots", func(t *testing.T) {
		sampleContent := `Linux 5.10.0-32-cloud-amd64 (ddc-test-dremio-master) 	09/04/24 	_x86_64_	(4 CPU)

09/04/24 12:07:20
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
           2.36    0.00    0.40    0.04    0.01   97.20

Device            r/s     rkB/s   rrqm/s  %rrqm r_await rareq-sz     w/s     wkB/s   wrqm/s  %wrqm w_await wareq-sz     d/s     dkB/s   drqm/s  %drqm d_await dareq-sz     f/s f_await  aqu-sz  %util
sda              2.08     94.38     0.31  13.07    0.89    45.47    9.58    210.39     5.55  36.68    2.74    21.96    0.09    377.20     0.00   0.00    0.95  4151.86    3.94    0.06    0.03   1.39


09/04/24 12:07:21
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
          33.91    0.00    7.67    2.72    0.00   55.69

Device            r/s     rkB/s   rrqm/s  %rrqm r_await rareq-sz     w/s     wkB/s   wrqm/s  %wrqm w_await wareq-sz     d/s     dkB/s   drqm/s  %drqm d_await dareq-sz     f/s f_await  aqu-sz  %util
sda              0.00      0.00     0.00   0.00    0.00     0.00  395.00  38116.00   133.00  25.19    8.65    96.50    1.00      4.00     0.00   0.00    1.00     4.00  122.00    0.06    3.42  39.20`

		data, err := ParseIOStat([]byte(sampleContent))
		require.NoError(t, err)
		require.NotNil(t, data)

		// Should have exactly 2 snapshots
		assert.Len(t, data.Snapshots, 2)

		// Check system info
		assert.Contains(t, data.SystemInfo, "Linux 5.10.0-32-cloud-amd64")
		assert.Contains(t, data.SystemInfo, "ddc-test-dremio-master")

		// Check first snapshot
		snapshot1 := data.Snapshots[0]
		assert.Equal(t, 2024, snapshot1.Timestamp.Year())
		assert.Equal(t, time.September, snapshot1.Timestamp.Month())
		assert.Equal(t, 4, snapshot1.Timestamp.Day())
		assert.Equal(t, 12, snapshot1.Timestamp.Hour())
		assert.Equal(t, 7, snapshot1.Timestamp.Minute())
		assert.Equal(t, 20, snapshot1.Timestamp.Second())

		// Check CPU stats for first snapshot
		require.NotNil(t, snapshot1.CPUStats)
		assert.Equal(t, 2.36, snapshot1.CPUStats.User)
		assert.Equal(t, 0.00, snapshot1.CPUStats.Nice)
		assert.Equal(t, 0.40, snapshot1.CPUStats.System)
		assert.Equal(t, 0.04, snapshot1.CPUStats.IOWait)
		assert.Equal(t, 0.01, snapshot1.CPUStats.Steal)
		assert.Equal(t, 97.20, snapshot1.CPUStats.Idle)

		// Check device stats for first snapshot
		assert.Len(t, snapshot1.Devices, 1)
		device1 := snapshot1.Devices[0]
		assert.Equal(t, "sda", device1.Device)
		assert.Equal(t, 2.08, device1.ReadsPerS)
		assert.Equal(t, 94.38, device1.ReadKBPerS)
		assert.Equal(t, 0.31, device1.ReadReqMergedPerS)
		assert.Equal(t, 13.07, device1.ReadReqMergedPct)
		assert.Equal(t, 0.89, device1.ReadAwait)
		assert.Equal(t, 45.47, device1.ReadReqSize)
		assert.Equal(t, 9.58, device1.WritesPerS)
		assert.Equal(t, 210.39, device1.WriteKBPerS)
		assert.Equal(t, 1.39, device1.Utilization)

		// Check second snapshot
		snapshot2 := data.Snapshots[1]
		assert.Equal(t, 12, snapshot2.Timestamp.Hour())
		assert.Equal(t, 7, snapshot2.Timestamp.Minute())
		assert.Equal(t, 21, snapshot2.Timestamp.Second())

		// Check CPU stats for second snapshot
		require.NotNil(t, snapshot2.CPUStats)
		assert.Equal(t, 33.91, snapshot2.CPUStats.User)
		assert.Equal(t, 7.67, snapshot2.CPUStats.System)
		assert.Equal(t, 2.72, snapshot2.CPUStats.IOWait)
		assert.Equal(t, 55.69, snapshot2.CPUStats.Idle)

		// Check device stats for second snapshot
		assert.Len(t, snapshot2.Devices, 1)
		device2 := snapshot2.Devices[0]
		assert.Equal(t, "sda", device2.Device)
		assert.Equal(t, 0.00, device2.ReadsPerS)
		assert.Equal(t, 395.00, device2.WritesPerS)
		assert.Equal(t, 38116.00, device2.WriteKBPerS)
		assert.Equal(t, 39.20, device2.Utilization)
	})

	t.Run("Parse empty content", func(t *testing.T) {
		data, err := ParseIOStat([]byte(""))
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Empty(t, data.Snapshots)
	})

	t.Run("Parse content with no valid snapshots", func(t *testing.T) {
		content := `Some random content
That doesn't match
The expected format`

		data, err := ParseIOStat([]byte(content))
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Empty(t, data.Snapshots)
	})

	t.Run("Parse content with malformed device lines should error", func(t *testing.T) {
		content := `Linux 5.10.0-32-cloud-amd64 (test-system) 	09/04/24 	_x86_64_	(4 CPU)

09/04/24 12:07:20
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
           2.36    0.00    0.40    0.04    0.01   97.20

Device            r/s     rkB/s   rrqm/s  %rrqm r_await rareq-sz     w/s     wkB/s   wrqm/s  %wrqm w_await wareq-sz     d/s     dkB/s   drqm/s  %drqm d_await dareq-sz     f/s f_await  aqu-sz  %util
sda              2.08     94.38     0.31  13.07    0.89    45.47    9.58    210.39     5.55  36.68    2.74    21.96    0.09    377.20     0.00   0.00    0.95  4151.86    3.94    0.06    0.03   1.39
invalid line with not enough fields
sdb              1.00     50.00     0.20  10.00    1.00    50.00    5.00    100.00     2.00  20.00    3.00    20.00    0.00      0.00     0.00   0.00    0.00     0.00    2.00    0.05    0.02   2.00`

		_, err := ParseIOStat([]byte(content))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse device statistics")
		assert.Contains(t, err.Error(), "line 9")
	})

	t.Run("Parse content with single snapshot", func(t *testing.T) {
		content := `Linux 5.10.0-32-cloud-amd64 (test-system) 	09/04/24 	_x86_64_	(4 CPU)

09/04/24 15:30:45
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
          25.50   10.20    5.30    2.10    1.00   55.90

Device            r/s     rkB/s   rrqm/s  %rrqm r_await rareq-sz     w/s     wkB/s   wrqm/s  %wrqm w_await wareq-sz     d/s     dkB/s   drqm/s  %drqm d_await dareq-sz     f/s f_await  aqu-sz  %util
sda              5.00    250.00     1.00  20.00    2.00    50.00   10.00    500.00     3.00  30.00    4.00    50.00    0.50     25.00     0.10   5.00    3.00    50.00    1.00    0.10    0.05  15.50`

		data, err := ParseIOStat([]byte(content))
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Len(t, data.Snapshots, 1)

		snapshot := data.Snapshots[0]
		assert.Equal(t, 15, snapshot.Timestamp.Hour())
		assert.Equal(t, 30, snapshot.Timestamp.Minute())
		assert.Equal(t, 45, snapshot.Timestamp.Second())

		// Check CPU stats
		require.NotNil(t, snapshot.CPUStats)
		assert.Equal(t, 25.50, snapshot.CPUStats.User)
		assert.Equal(t, 10.20, snapshot.CPUStats.Nice)
		assert.Equal(t, 5.30, snapshot.CPUStats.System)
		assert.Equal(t, 2.10, snapshot.CPUStats.IOWait)
		assert.Equal(t, 1.00, snapshot.CPUStats.Steal)
		assert.Equal(t, 55.90, snapshot.CPUStats.Idle)

		// Check device stats
		assert.Len(t, snapshot.Devices, 1)
		device := snapshot.Devices[0]
		assert.Equal(t, "sda", device.Device)
		assert.Equal(t, 5.00, device.ReadsPerS)
		assert.Equal(t, 250.00, device.ReadKBPerS)
		assert.Equal(t, 10.00, device.WritesPerS)
		assert.Equal(t, 500.00, device.WriteKBPerS)
		assert.Equal(t, 15.50, device.Utilization)
	})
}

func TestParseIOStatTimestamp(t *testing.T) {
	t.Run("Valid timestamp parsing", func(t *testing.T) {
		line := "09/04/24 12:07:20"
		timestamp, err := parseIOStatTimestamp(line)
		require.NoError(t, err)

		assert.Equal(t, 2024, timestamp.Year())
		assert.Equal(t, time.September, timestamp.Month())
		assert.Equal(t, 4, timestamp.Day())
		assert.Equal(t, 12, timestamp.Hour())
		assert.Equal(t, 7, timestamp.Minute())
		assert.Equal(t, 20, timestamp.Second())
	})

	t.Run("Invalid timestamp format", func(t *testing.T) {
		line := "invalid timestamp"
		_, err := parseIOStatTimestamp(line)
		require.Error(t, err)
	})

	t.Run("Missing timestamp parts", func(t *testing.T) {
		line := "09/04/24"
		_, err := parseIOStatTimestamp(line)
		require.Error(t, err)
	})
}

func TestIsTimestampLine(t *testing.T) {
	t.Run("Valid timestamp line", func(t *testing.T) {
		line := "09/04/24 12:07:20"
		assert.True(t, isTimestampLine(line))
	})

	t.Run("Invalid timestamp line - no time", func(t *testing.T) {
		line := "09/04/24"
		assert.False(t, isTimestampLine(line))
	})

	t.Run("Invalid timestamp line - wrong format", func(t *testing.T) {
		line := "2024-09-04 12:07:20"
		assert.False(t, isTimestampLine(line))
	})

	t.Run("Invalid timestamp line - not a timestamp", func(t *testing.T) {
		line := "avg-cpu:  %user   %nice %system %iowait  %steal   %idle"
		assert.False(t, isTimestampLine(line))
	})
}

func TestParseCPUStatsLine(t *testing.T) {
	t.Run("Valid CPU stats line", func(t *testing.T) {
		line := "           2.36    0.00    0.40    0.04    0.01   97.20"

		stats, err := parseCPUStatsLine(line)
		require.NoError(t, err)
		require.NotNil(t, stats)

		assert.Equal(t, 2.36, stats.User)
		assert.Equal(t, 0.00, stats.Nice)
		assert.Equal(t, 0.40, stats.System)
		assert.Equal(t, 0.04, stats.IOWait)
		assert.Equal(t, 0.01, stats.Steal)
		assert.Equal(t, 97.20, stats.Idle)
	})

	t.Run("Invalid CPU stats line - insufficient fields", func(t *testing.T) {
		line := "2.36 0.00 0.40"
		_, err := parseCPUStatsLine(line)
		require.Error(t, err)
	})

	t.Run("Invalid CPU stats line - non-numeric values", func(t *testing.T) {
		line := "invalid 0.00 0.40 0.04 0.01 97.20"
		_, err := parseCPUStatsLine(line)
		require.Error(t, err)
	})
}

func TestParseDeviceStatsLine(t *testing.T) {
	t.Run("Valid device stats line", func(t *testing.T) {
		line := "sda              2.08     94.38     0.31  13.07    0.89    45.47    9.58    210.39     5.55  36.68    2.74    21.96    0.09    377.20     0.00   0.00    0.95  4151.86    3.94    0.06    0.03   1.39"

		device, err := parseDeviceStatsLine(line)
		require.NoError(t, err)

		assert.Equal(t, "sda", device.Device)
		assert.Equal(t, 2.08, device.ReadsPerS)
		assert.Equal(t, 94.38, device.ReadKBPerS)
		assert.Equal(t, 0.31, device.ReadReqMergedPerS)
		assert.Equal(t, 13.07, device.ReadReqMergedPct)
		assert.Equal(t, 0.89, device.ReadAwait)
		assert.Equal(t, 45.47, device.ReadReqSize)
		assert.Equal(t, 9.58, device.WritesPerS)
		assert.Equal(t, 210.39, device.WriteKBPerS)
		assert.Equal(t, 5.55, device.WriteReqMergedPerS)
		assert.Equal(t, 36.68, device.WriteReqMergedPct)
		assert.Equal(t, 2.74, device.WriteAwait)
		assert.Equal(t, 21.96, device.WriteReqSize)
		assert.Equal(t, 0.09, device.DiscardsPerS)
		assert.Equal(t, 377.20, device.DiscardKBPerS)
		assert.Equal(t, 0.00, device.DiscardReqMergedPerS)
		assert.Equal(t, 0.00, device.DiscardReqMergedPct)
		assert.Equal(t, 0.95, device.DiscardAwait)
		assert.Equal(t, 4151.86, device.DiscardReqSize)
		assert.Equal(t, 3.94, device.FlushesPerS)
		assert.Equal(t, 0.06, device.FlushAwait)
		assert.Equal(t, 0.03, device.AvgQueueSize)
		assert.Equal(t, 1.39, device.Utilization)
	})

	t.Run("Invalid device stats line - insufficient fields", func(t *testing.T) {
		line := "sda 2.08 94.38"
		_, err := parseDeviceStatsLine(line)
		require.Error(t, err)
	})

	t.Run("Invalid device stats line - non-numeric values", func(t *testing.T) {
		line := "sda invalid 94.38 0.31 13.07 0.89 45.47 9.58 210.39 5.55 36.68 2.74 21.96 0.09 377.20 0.00 0.00 0.95 4151.86 3.94 0.06 0.03 1.39"
		_, err := parseDeviceStatsLine(line)
		require.Error(t, err)
	})

	t.Run("Device stats with zero values", func(t *testing.T) {
		line := "sdb              0.00      0.00     0.00   0.00    0.00     0.00    0.00      0.00     0.00   0.00    0.00     0.00    0.00      0.00     0.00   0.00    0.00     0.00    0.00    0.00    0.00   0.00"

		device, err := parseDeviceStatsLine(line)
		require.NoError(t, err)

		assert.Equal(t, "sdb", device.Device)
		assert.Equal(t, 0.00, device.ReadsPerS)
		assert.Equal(t, 0.00, device.ReadKBPerS)
		assert.Equal(t, 0.00, device.WritesPerS)
		assert.Equal(t, 0.00, device.WriteKBPerS)
		assert.Equal(t, 0.00, device.Utilization)
	})
}

func TestIOStatReportDataStructure(t *testing.T) {
	t.Run("Data structure creation and access", func(t *testing.T) {
		// Create test data
		timestamp := time.Date(2024, 9, 4, 12, 7, 20, 0, time.UTC)
		cpuStats := &CPUStats{
			User:   25.5,
			Nice:   0.0,
			System: 10.2,
			IOWait: 2.1,
			Steal:  0.5,
			Idle:   61.7,
		}
		deviceStats := DeviceStats{
			Device:      "sda",
			ReadsPerS:   5.0,
			ReadKBPerS:  250.0,
			WritesPerS:  10.0,
			WriteKBPerS: 500.0,
			Utilization: 15.5,
		}

		snapshot := IOStatSnapshot{
			Timestamp: timestamp,
			CPUStats:  cpuStats,
			Devices:   []DeviceStats{deviceStats},
		}

		data := IOStatReportData{
			SystemInfo: "Linux 5.10.0-32-cloud-amd64 (test-system)",
			Snapshots:  []IOStatSnapshot{snapshot},
		}

		// Verify structure
		assert.Len(t, data.Snapshots, 1)
		assert.Equal(t, timestamp, data.Snapshots[0].Timestamp)
		assert.Equal(t, cpuStats, data.Snapshots[0].CPUStats)
		assert.Len(t, data.Snapshots[0].Devices, 1)
		assert.Equal(t, deviceStats, data.Snapshots[0].Devices[0])
		assert.Contains(t, data.SystemInfo, "Linux")
	})
}

func TestIOStatParserEdgeCases(t *testing.T) {
	t.Run("Parse content with missing CPU stats should error", func(t *testing.T) {
		content := `Linux 5.10.0-32-cloud-amd64 (test-system) 	09/04/24 	_x86_64_	(4 CPU)

09/04/24 12:07:20

Device            r/s     rkB/s   rrqm/s  %rrqm r_await rareq-sz     w/s     wkB/s   wrqm/s  %wrqm w_await wareq-sz     d/s     dkB/s   drqm/s  %drqm d_await dareq-sz     f/s f_await  aqu-sz  %util
sda              2.08     94.38     0.31  13.07    0.89    45.47    9.58    210.39     5.55  36.68    2.74    21.96    0.09    377.20     0.00   0.00    0.95  4151.86    3.94    0.06    0.03   1.39`

		_, err := ParseIOStat([]byte(content))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected CPU statistics")
		assert.Contains(t, err.Error(), "line")
	})

	t.Run("Parse content with missing device stats", func(t *testing.T) {
		content := `Linux 5.10.0-32-cloud-amd64 (test-system) 	09/04/24 	_x86_64_	(4 CPU)

09/04/24 12:07:20
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
           2.36    0.00    0.40    0.04    0.01   97.20`

		data, err := ParseIOStat([]byte(content))
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Len(t, data.Snapshots, 1)

		// CPU stats should be present
		require.NotNil(t, data.Snapshots[0].CPUStats)
		assert.Equal(t, 2.36, data.Snapshots[0].CPUStats.User)
		// But device stats should be empty
		assert.Empty(t, data.Snapshots[0].Devices)
	})

	t.Run("Parse content with multiple devices", func(t *testing.T) {
		content := `Linux 5.10.0-32-cloud-amd64 (test-system) 	09/04/24 	_x86_64_	(4 CPU)

09/04/24 12:07:20
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
           2.36    0.00    0.40    0.04    0.01   97.20

Device            r/s     rkB/s   rrqm/s  %rrqm r_await rareq-sz     w/s     wkB/s   wrqm/s  %wrqm w_await wareq-sz     d/s     dkB/s   drqm/s  %drqm d_await dareq-sz     f/s f_await  aqu-sz  %util
sda              2.08     94.38     0.31  13.07    0.89    45.47    9.58    210.39     5.55  36.68    2.74    21.96    0.09    377.20     0.00   0.00    0.95  4151.86    3.94    0.06    0.03   1.39
sdb              1.00     50.00     0.20  10.00    1.00    50.00    5.00    100.00     2.00  20.00    3.00    20.00    0.00      0.00     0.00   0.00    0.00     0.00    2.00    0.05    0.02   2.00
nvme0n1          5.50    275.00     1.50  25.00    1.50    50.00   15.00    750.00     7.50  33.33    5.00    50.00    0.25     12.50     0.05   2.50    2.50    50.00    3.00    0.08    0.10  25.75`

		data, err := ParseIOStat([]byte(content))
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Len(t, data.Snapshots, 1)

		// Should have 3 devices
		assert.Len(t, data.Snapshots[0].Devices, 3)
		assert.Equal(t, "sda", data.Snapshots[0].Devices[0].Device)
		assert.Equal(t, "sdb", data.Snapshots[0].Devices[1].Device)
		assert.Equal(t, "nvme0n1", data.Snapshots[0].Devices[2].Device)
	})

	t.Run("Parse content with malformed CPU stats should error with line number", func(t *testing.T) {
		content := `Linux 5.10.0-32-cloud-amd64 (test-system) 	09/04/24 	_x86_64_	(4 CPU)

09/04/24 12:07:20
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
           invalid    0.00    0.40    0.04    0.01   97.20`

		_, err := ParseIOStat([]byte(content))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse CPU statistics")
		assert.Contains(t, err.Error(), "line 5")
	})

	t.Run("Parse content with malformed device stats should error with line number", func(t *testing.T) {
		content := `Linux 5.10.0-32-cloud-amd64 (test-system) 	09/04/24 	_x86_64_	(4 CPU)

09/04/24 12:07:20
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
           2.36    0.00    0.40    0.04    0.01   97.20

Device            r/s     rkB/s   rrqm/s  %rrqm r_await rareq-sz     w/s     wkB/s   wrqm/s  %wrqm w_await wareq-sz     d/s     dkB/s   drqm/s  %drqm d_await dareq-sz     f/s f_await  aqu-sz  %util
sda              invalid     94.38     0.31  13.07    0.89    45.47    9.58    210.39     5.55  36.68    2.74    21.96    0.09    377.20     0.00   0.00    0.95  4151.86    3.94    0.06    0.03   1.39`

		_, err := ParseIOStat([]byte(content))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse device statistics")
		assert.Contains(t, err.Error(), "line 8")
	})

	t.Run("Parse content with wrong number of CPU fields should error with line number", func(t *testing.T) {
		content := `Linux 5.10.0-32-cloud-amd64 (test-system) 	09/04/24 	_x86_64_	(4 CPU)

09/04/24 12:07:20
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
           2.36    0.00    0.40    0.04`

		_, err := ParseIOStat([]byte(content))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected CPU statistics with 6 fields, got 4 fields")
		assert.Contains(t, err.Error(), "line 5")
	})

	t.Run("Parse content ending without CPU stats should error", func(t *testing.T) {
		content := `Linux 5.10.0-32-cloud-amd64 (test-system) 	09/04/24 	_x86_64_	(4 CPU)

09/04/24 12:07:20`

		_, err := ParseIOStat([]byte(content))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected CPU statistics after timestamp")
		assert.Contains(t, err.Error(), "reached end of file")
	})
}
