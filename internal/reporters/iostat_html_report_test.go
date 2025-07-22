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

func TestGenerateIOStatHTML(t *testing.T) {
	t.Run("Generate HTML with ECharts", func(t *testing.T) {
		// Create test data
		data := &IOStatReportData{
			SystemInfo: "Linux 5.10.0-32-cloud-amd64 (test-system)",
			Snapshots: []IOStatSnapshot{
				{
					Timestamp: time.Date(2024, 9, 4, 12, 7, 20, 0, time.UTC),
					CPUStats: &CPUStats{
						User:   25.5,
						Nice:   0.0,
						System: 10.2,
						IOWait: 2.1,
						Steal:  0.5,
						Idle:   61.7,
					},
					Devices: []DeviceStats{
						{
							Device:      "sda",
							ReadsPerS:   5.0,
							ReadKBPerS:  250.0,
							WritesPerS:  10.0,
							WriteKBPerS: 500.0,
							Utilization: 15.5,
						},
						{
							Device:      "sdb",
							ReadsPerS:   2.0,
							ReadKBPerS:  100.0,
							WritesPerS:  5.0,
							WriteKBPerS: 200.0,
							Utilization: 8.2,
						},
					},
				},
				{
					Timestamp: time.Date(2024, 9, 4, 12, 7, 21, 0, time.UTC),
					CPUStats: &CPUStats{
						User:   30.0,
						Nice:   1.0,
						System: 12.5,
						IOWait: 3.2,
						Steal:  0.8,
						Idle:   52.5,
					},
					Devices: []DeviceStats{
						{
							Device:      "sda",
							ReadsPerS:   8.0,
							ReadKBPerS:  400.0,
							WritesPerS:  15.0,
							WriteKBPerS: 750.0,
							Utilization: 22.3,
						},
						{
							Device:      "sdb",
							ReadsPerS:   3.0,
							ReadKBPerS:  150.0,
							WritesPerS:  7.0,
							WriteKBPerS: 350.0,
							Utilization: 12.1,
						},
					},
				},
			},
		}

		html, err := GenerateIOStatHTML(data)
		require.NoError(t, err)
		assert.NotEmpty(t, html)

		// Verify ECharts is included
		assert.Contains(t, html, "echarts.min.js")
		assert.NotContains(t, html, "chart.js")

		// Verify all chart containers are present
		assert.Contains(t, html, `id="cpuChart"`)
		assert.Contains(t, html, `id="ioThroughputChart"`)
		assert.Contains(t, html, `id="deviceAwaitChart"`)
		assert.Contains(t, html, `id="deviceQueueChart"`)
		assert.Contains(t, html, `id="deviceRequestsChart"`)
		assert.Contains(t, html, `id="deviceRequestSizeChart"`)

		// Verify chart titles
		assert.Contains(t, html, "CPU Utilization Over Time")
		assert.Contains(t, html, "Device I/O Throughput Over Time")
		assert.Contains(t, html, "Device I/O Await Times")
		assert.Contains(t, html, "Device Average Queue Size")
		assert.Contains(t, html, "Device I/O Requests Per Second")
		assert.Contains(t, html, "Device I/O Request Sizes")

		// Verify device utilization chart is NOT present
		assert.NotContains(t, html, `id="deviceUtilChart"`)
		assert.NotContains(t, html, "Device Utilization Over Time")
		assert.NotContains(t, html, "Performance Interpretation Guide")

		// Verify ECharts initialization
		assert.Contains(t, html, "echarts.init")
		assert.Contains(t, html, "setOption")
		assert.Contains(t, html, "resize()")

		// Verify statistics are displayed
		assert.Contains(t, html, "2") // snapshot count
		assert.Contains(t, html, "2") // device count (sda, sdb)

		// Verify data is embedded in charts
		assert.Contains(t, html, "25.5")  // CPU user value
		assert.Contains(t, html, "10.2")  // CPU system value
		assert.Contains(t, html, "350.0") // Read KB/s aggregated
		assert.Contains(t, html, "700.0") // Write KB/s aggregated
	})

	t.Run("Generate HTML with empty data", func(t *testing.T) {
		data := &IOStatReportData{
			SystemInfo: "",
			Snapshots:  []IOStatSnapshot{},
		}

		html, err := GenerateIOStatHTML(data)
		require.NoError(t, err)
		assert.NotEmpty(t, html)

		// Should generate empty state HTML
		assert.Contains(t, html, "No IOStat Data Available")
		assert.Contains(t, html, "empty or could not be parsed")
	})

	t.Run("Generate HTML with nil data", func(t *testing.T) {
		html, err := GenerateIOStatHTML(nil)
		require.NoError(t, err)
		assert.NotEmpty(t, html)

		// Should generate empty state HTML
		assert.Contains(t, html, "No IOStat Data Available")
	})

	t.Run("Generate HTML with single snapshot", func(t *testing.T) {
		data := &IOStatReportData{
			SystemInfo: "Linux 5.10.0-32-cloud-amd64 (test-system)",
			Snapshots: []IOStatSnapshot{
				{
					Timestamp: time.Date(2024, 9, 4, 12, 7, 20, 0, time.UTC),
					CPUStats: &CPUStats{
						User:   15.5,
						Nice:   0.0,
						System: 5.2,
						IOWait: 1.1,
						Steal:  0.2,
						Idle:   78.0,
					},
					Devices: []DeviceStats{
						{
							Device:      "sda",
							ReadsPerS:   3.0,
							ReadKBPerS:  150.0,
							WritesPerS:  6.0,
							WriteKBPerS: 300.0,
							Utilization: 10.5,
						},
					},
				},
			},
		}

		html, err := GenerateIOStatHTML(data)
		require.NoError(t, err)
		assert.NotEmpty(t, html)
		assert.Contains(t, html, "1") // snapshot count
		assert.Contains(t, html, "1") // device count
	})
}

func TestExtractIOStatTimeLabels(t *testing.T) {
	t.Run("Extract time labels", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{Timestamp: time.Date(2024, 9, 4, 12, 7, 20, 0, time.UTC)},
				{Timestamp: time.Date(2024, 9, 4, 12, 7, 21, 0, time.UTC)},
				{Timestamp: time.Date(2024, 9, 4, 12, 7, 22, 0, time.UTC)},
			},
		}

		result := extractIOStatTimeLabels(data)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "12:07:20")
		assert.Contains(t, result, "12:07:21")
		assert.Contains(t, result, "12:07:22")
		assert.Contains(t, result, "[")
		assert.Contains(t, result, "]")
	})
}

func TestExtractCPUSeriesData(t *testing.T) {
	t.Run("Extract CPU series data", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{
					CPUStats: &CPUStats{
						User:   25.5,
						System: 10.2,
						IOWait: 2.1,
						Idle:   62.2,
					},
				},
				{
					CPUStats: &CPUStats{
						User:   30.0,
						System: 12.5,
						IOWait: 3.2,
						Idle:   54.3,
					},
				},
			},
		}

		result := extractCPUSeriesData(data)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "User")
		assert.Contains(t, result, "System")
		assert.Contains(t, result, "IOWait")
		assert.Contains(t, result, "Idle")
		assert.Contains(t, result, "25.5, 30.0") // User values
		assert.Contains(t, result, "10.2, 12.5") // System values
		assert.Contains(t, result, "2.1, 3.2")   // IOWait values
		assert.Contains(t, result, "62.2, 54.3") // Idle values
	})

	t.Run("Extract CPU series data with missing stats", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{CPUStats: nil},
				{
					CPUStats: &CPUStats{
						User:   15.0,
						System: 5.0,
						IOWait: 1.0,
						Idle:   79.0,
					},
				},
			},
		}

		result := extractCPUSeriesData(data)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "0, 15.0") // User values (0 for missing, 15.0 for present)
		assert.Contains(t, result, "0, 5.0")  // System values
		assert.Contains(t, result, "0, 1.0")  // IOWait values
		assert.Contains(t, result, "0, 79.0") // Idle values
	})
}

func TestExtractIOThroughputSeriesData(t *testing.T) {
	t.Run("Extract I/O throughput data", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{
					Devices: []DeviceStats{
						{ReadKBPerS: 100.0, WriteKBPerS: 200.0},
						{ReadKBPerS: 50.0, WriteKBPerS: 100.0},
					},
				},
				{
					Devices: []DeviceStats{
						{ReadKBPerS: 150.0, WriteKBPerS: 300.0},
						{ReadKBPerS: 75.0, WriteKBPerS: 150.0},
					},
				},
			},
		}

		result := extractIOThroughputSeriesData(data)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Read KB/s")
		assert.Contains(t, result, "Write KB/s")
		assert.Contains(t, result, "150.0, 225.0") // Aggregated read values (100+50, 150+75)
		assert.Contains(t, result, "300.0, 450.0") // Aggregated write values (200+100, 300+150)
	})

	t.Run("Extract I/O throughput data with no devices", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{Devices: []DeviceStats{}},
				{Devices: []DeviceStats{}},
			},
		}

		result := extractIOThroughputSeriesData(data)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "0.0, 0.0") // Should have zero values
	})
}

func TestCountUniqueDevices(t *testing.T) {
	t.Run("Count unique devices across snapshots", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{
					Devices: []DeviceStats{
						{Device: "sda"},
						{Device: "sdb"},
					},
				},
				{
					Devices: []DeviceStats{
						{Device: "sda"},
						{Device: "sdb"},
						{Device: "nvme0n1"},
					},
				},
			},
		}

		count := countUniqueDevices(data)
		assert.Equal(t, 3, count) // sda, sdb, nvme0n1
	})

	t.Run("Count unique devices with no devices", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{Devices: []DeviceStats{}},
				{Devices: []DeviceStats{}},
			},
		}

		count := countUniqueDevices(data)
		assert.Equal(t, 0, count)
	})

	t.Run("Count unique devices with duplicate devices", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{
					Devices: []DeviceStats{
						{Device: "sda"},
						{Device: "sda"}, // Duplicate
					},
				},
			},
		}

		count := countUniqueDevices(data)
		assert.Equal(t, 1, count) // Only one unique device
	})
}

func TestFindPeakCPUUsage(t *testing.T) {
	t.Run("Find peak CPU usage", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{
					CPUStats: &CPUStats{Idle: 80.0}, // 20% usage
				},
				{
					CPUStats: &CPUStats{Idle: 60.0}, // 40% usage
				},
				{
					CPUStats: &CPUStats{Idle: 70.0}, // 30% usage
				},
			},
		}

		peak := findPeakCPUUsage(data)
		assert.Equal(t, 40.0, peak) // 100 - 60 = 40%
	})

	t.Run("Find peak CPU usage with nil stats", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{CPUStats: nil},
				{
					CPUStats: &CPUStats{Idle: 75.0}, // 25% usage
				},
			},
		}

		peak := findPeakCPUUsage(data)
		assert.Equal(t, 25.0, peak) // 100 - 75 = 25%
	})

	t.Run("Find peak CPU usage with no snapshots", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{},
		}

		peak := findPeakCPUUsage(data)
		assert.Equal(t, 0.0, peak)
	})
}

func TestFindPeakDeviceQueueSize(t *testing.T) {
	t.Run("Find peak device queue size", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{
					Devices: []DeviceStats{
						{AvgQueueSize: 1.5},
						{AvgQueueSize: 0.8},
					},
				},
				{
					Devices: []DeviceStats{
						{AvgQueueSize: 2.3},
						{AvgQueueSize: 1.2},
					},
				},
				{
					Devices: []DeviceStats{
						{AvgQueueSize: 1.8},
						{AvgQueueSize: 0.9},
					},
				},
			},
		}

		peak := findPeakDeviceQueueSize(data)
		assert.Equal(t, 2.3, peak) // Highest queue size across all devices and snapshots
	})

	t.Run("Find peak device queue size with no devices", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{Devices: []DeviceStats{}},
				{Devices: []DeviceStats{}},
			},
		}

		peak := findPeakDeviceQueueSize(data)
		assert.Equal(t, 0.0, peak)
	})

	t.Run("Find peak device queue size with no snapshots", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{},
		}

		peak := findPeakDeviceQueueSize(data)
		assert.Equal(t, 0.0, peak)
	})
}

func TestGenerateEmptyIOStatHTML(t *testing.T) {
	t.Run("Generate empty state HTML", func(t *testing.T) {
		html := generateEmptyIOStatHTML()
		assert.NotEmpty(t, html)
		assert.Contains(t, html, "No IOStat Data Available")
		assert.Contains(t, html, "empty or could not be parsed")
		assert.Contains(t, html, "<!DOCTYPE html>")
		assert.Contains(t, html, "</html>")
	})
}

func TestExtractDeviceAwaitData(t *testing.T) {
	t.Run("Extract device await legend and series data", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{
					Devices: []DeviceStats{
						{Device: "sda", ReadAwait: 2.5, WriteAwait: 3.2},
						{Device: "sdb", ReadAwait: 1.8, WriteAwait: 2.1},
					},
				},
				{
					Devices: []DeviceStats{
						{Device: "sda", ReadAwait: 3.1, WriteAwait: 4.0},
						{Device: "sdb", ReadAwait: 2.2, WriteAwait: 2.8},
					},
				},
			},
		}

		legend := extractDeviceAwaitLegendData(data)
		assert.NotEmpty(t, legend)
		assert.Contains(t, legend, "Read Await")
		assert.Contains(t, legend, "Write Await")
		assert.Contains(t, legend, "sda")
		assert.Contains(t, legend, "sdb")

		series := extractDeviceAwaitSeriesData(data)
		assert.NotEmpty(t, series)
		assert.Contains(t, series, "2.50, 3.10") // sda read await values
		assert.Contains(t, series, "3.20, 4.00") // sda write await values
		assert.Contains(t, series, "1.80, 2.20") // sdb read await values
		assert.Contains(t, series, "2.10, 2.80") // sdb write await values
	})
}

func TestExtractDeviceQueueData(t *testing.T) {
	t.Run("Extract device queue legend and series data", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{
					Devices: []DeviceStats{
						{Device: "sda", AvgQueueSize: 1.5},
						{Device: "sdb", AvgQueueSize: 0.8},
					},
				},
				{
					Devices: []DeviceStats{
						{Device: "sda", AvgQueueSize: 2.1},
						{Device: "sdb", AvgQueueSize: 1.2},
					},
				},
			},
		}

		legend := extractDeviceQueueLegendData(data)
		assert.NotEmpty(t, legend)
		assert.Contains(t, legend, "Queue Size")
		assert.Contains(t, legend, "sda")
		assert.Contains(t, legend, "sdb")

		series := extractDeviceQueueSeriesData(data)
		assert.NotEmpty(t, series)
		assert.Contains(t, series, "1.50, 2.10") // sda queue size values
		assert.Contains(t, series, "0.80, 1.20") // sdb queue size values
		assert.Contains(t, series, "areaStyle")  // Should have area style
	})
}

func TestExtractDeviceRequestsData(t *testing.T) {
	t.Run("Extract device requests legend and series data", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{
					Devices: []DeviceStats{
						{Device: "sda", ReadsPerS: 10.5, WritesPerS: 15.2},
						{Device: "sdb", ReadsPerS: 5.8, WritesPerS: 8.1},
					},
				},
				{
					Devices: []DeviceStats{
						{Device: "sda", ReadsPerS: 12.3, WritesPerS: 18.7},
						{Device: "sdb", ReadsPerS: 6.9, WritesPerS: 9.4},
					},
				},
			},
		}

		legend := extractDeviceRequestsLegendData(data)
		assert.NotEmpty(t, legend)
		assert.Contains(t, legend, "Reads/sec")
		assert.Contains(t, legend, "Writes/sec")
		assert.Contains(t, legend, "sda")
		assert.Contains(t, legend, "sdb")

		series := extractDeviceRequestsSeriesData(data)
		assert.NotEmpty(t, series)
		assert.Contains(t, series, "10.50, 12.30") // sda reads/sec values
		assert.Contains(t, series, "15.20, 18.70") // sda writes/sec values
		assert.Contains(t, series, "5.80, 6.90")   // sdb reads/sec values
		assert.Contains(t, series, "8.10, 9.40")   // sdb writes/sec values
	})
}

func TestExtractDeviceRequestSizeData(t *testing.T) {
	t.Run("Extract device request size legend and series data", func(t *testing.T) {
		data := &IOStatReportData{
			Snapshots: []IOStatSnapshot{
				{
					Devices: []DeviceStats{
						{Device: "sda", ReadReqSize: 64.5, WriteReqSize: 128.2},
						{Device: "sdb", ReadReqSize: 32.8, WriteReqSize: 96.1},
					},
				},
				{
					Devices: []DeviceStats{
						{Device: "sda", ReadReqSize: 72.3, WriteReqSize: 140.7},
						{Device: "sdb", ReadReqSize: 38.9, WriteReqSize: 104.4},
					},
				},
			},
		}

		legend := extractDeviceRequestSizeLegendData(data)
		assert.NotEmpty(t, legend)
		assert.Contains(t, legend, "Read Size")
		assert.Contains(t, legend, "Write Size")
		assert.Contains(t, legend, "sda")
		assert.Contains(t, legend, "sdb")

		series := extractDeviceRequestSizeSeriesData(data)
		assert.NotEmpty(t, series)
		assert.Contains(t, series, "64.50, 72.30")   // sda read size values
		assert.Contains(t, series, "128.20, 140.70") // sda write size values
		assert.Contains(t, series, "32.80, 38.90")   // sdb read size values
		assert.Contains(t, series, "96.10, 104.40")  // sdb write size values
	})
}
