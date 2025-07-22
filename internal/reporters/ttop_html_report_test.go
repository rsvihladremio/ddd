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

func TestGenerateTTopHTML(t *testing.T) {
	t.Run("Generate HTML with ECharts", func(t *testing.T) {
		// Create test data
		data := &TTopReportData{
			Snapshots: []TTopSnapshot{
				{
					Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					ThreadCounts: &ThreadCounts{
						Total:    100,
						Running:  2,
						Sleeping: 98,
						Stopped:  0,
						Zombie:   0,
					},
					SystemMemory: &SystemMemory{
						MemTotal:     8000.0,
						MemFree:      4000.0,
						MemUsed:      3000.0,
						MemBuffCache: 1000.0,
						SwapTotal:    0.0,
						SwapFree:     0.0,
						SwapUsed:     0.0,
						MemAvail:     5000.0,
					},
					Threads: []ThreadInfo{
						{PID: 1234, User: "dremio", CPU: 25.5, MEM: 10.2, Command: "java"},
						{PID: 5678, User: "root", CPU: 15.0, MEM: 5.1, Command: "compiler"},
					},
				},
				{
					Timestamp: time.Date(2024, 1, 1, 12, 0, 1, 0, time.UTC),
					ThreadCounts: &ThreadCounts{
						Total:    105,
						Running:  3,
						Sleeping: 102,
						Stopped:  0,
						Zombie:   0,
					},
					SystemMemory: &SystemMemory{
						MemTotal:     8000.0,
						MemFree:      3500.0,
						MemUsed:      3500.0,
						MemBuffCache: 1000.0,
						SwapTotal:    0.0,
						SwapFree:     0.0,
						SwapUsed:     0.0,
						MemAvail:     4500.0,
					},
					Threads: []ThreadInfo{
						{PID: 1234, User: "dremio", CPU: 30.0, MEM: 12.0, Command: "java"},
						{PID: 5678, User: "root", CPU: 20.0, MEM: 6.0, Command: "compiler"},
						{PID: 9012, User: "user", CPU: 5.0, MEM: 2.0, Command: "system"},
					},
				},
			},
		}

		html, err := GenerateTTopHTML(data)
		require.NoError(t, err)
		assert.NotEmpty(t, html)

		// Verify ECharts is included
		assert.Contains(t, html, "echarts.min.js")
		assert.NotContains(t, html, "chart.js")

		// Verify all chart containers are present
		assert.Contains(t, html, `id="threadByCpuChart"`)
		assert.Contains(t, html, `id="memoryByTypeChart"`)
		assert.Contains(t, html, `id="threadsByTypeChart"`)

		// Verify chart titles
		assert.Contains(t, html, "Threads by Name/ID CPU Usage Over Time")
		assert.Contains(t, html, "System Memory Usage Over Time")
		assert.Contains(t, html, "Thread States Over Time")

		// Verify ECharts initialization
		assert.Contains(t, html, "echarts.init")
		assert.Contains(t, html, "setOption")
		assert.Contains(t, html, "resize()")
	})

	t.Run("Generate HTML with empty data", func(t *testing.T) {
		html, err := GenerateTTopHTML(nil)
		require.NoError(t, err)
		assert.NotEmpty(t, html)
		assert.Contains(t, html, "No data available")
	})

	t.Run("Generate HTML with single snapshot", func(t *testing.T) {
		data := &TTopReportData{
			Snapshots: []TTopSnapshot{
				{
					Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					ThreadCounts: &ThreadCounts{
						Total:    50,
						Running:  1,
						Sleeping: 49,
						Stopped:  0,
						Zombie:   0,
					},
					SystemMemory: &SystemMemory{
						MemTotal:     4000.0,
						MemFree:      2000.0,
						MemUsed:      1500.0,
						MemBuffCache: 500.0,
						SwapTotal:    0.0,
						SwapFree:     0.0,
						SwapUsed:     0.0,
						MemAvail:     2500.0,
					},
					Threads: []ThreadInfo{
						{PID: 1234, User: "dremio", CPU: 25.5, MEM: 10.2, Command: "java"},
					},
				},
			},
		}

		html, err := GenerateTTopHTML(data)
		require.NoError(t, err)
		assert.NotEmpty(t, html)
		assert.Contains(t, html, "Snapshots")
		assert.Contains(t, html, "<div class=\"stat-value\">1</div>")
	})
}

func TestExtractCPULegendData(t *testing.T) {
	t.Run("Extract CPU legend data", func(t *testing.T) {
		data := &TTopReportData{
			Snapshots: []TTopSnapshot{
				{
					Threads: []ThreadInfo{
						{PID: 1234, Command: "java", CPU: 25.5},
						{PID: 5678, Command: "compiler", CPU: 15.0},
					},
				},
			},
		}

		result := extractCPULegendData(data)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "java (PID: 1234)")
		assert.Contains(t, result, "compiler (PID: 5678)")
	})

	t.Run("Extract CPU legend data with top 5 limit", func(t *testing.T) {
		data := &TTopReportData{
			Snapshots: []TTopSnapshot{
				{
					Threads: []ThreadInfo{
						{PID: 1, Command: "thread1", CPU: 90.0},
						{PID: 2, Command: "thread2", CPU: 80.0},
						{PID: 3, Command: "thread3", CPU: 70.0},
						{PID: 4, Command: "thread4", CPU: 60.0},
						{PID: 5, Command: "thread5", CPU: 50.0},
						{PID: 6, Command: "thread6", CPU: 40.0},
					},
				},
			},
		}

		result := extractCPULegendData(data)
		assert.NotEmpty(t, result)

		// Should contain top 5 threads
		assert.Contains(t, result, "thread1 (PID: 1)")
		assert.Contains(t, result, "thread2 (PID: 2)")
		assert.Contains(t, result, "thread3 (PID: 3)")
		assert.Contains(t, result, "thread4 (PID: 4)")
		assert.Contains(t, result, "thread5 (PID: 5)")

		// Should not contain the 6th thread
		assert.NotContains(t, result, "thread6 (PID: 6)")
	})
}

func TestExtractMemoryTypeLegendData(t *testing.T) {
	t.Run("Extract memory type legend data", func(t *testing.T) {
		data := &TTopReportData{
			Snapshots: []TTopSnapshot{
				{
					SystemMemory: &SystemMemory{
						MemTotal:     8000.0,
						MemFree:      4000.0,
						MemUsed:      3000.0,
						MemBuffCache: 1000.0,
						SwapTotal:    1024.0,
						SwapFree:     512.0,
						SwapUsed:     512.0,
						MemAvail:     5000.0,
					},
				},
			},
		}

		result := extractMemoryTypeLegendData(data)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Memory Used (MiB)")
		assert.Contains(t, result, "Buffer/Cache (MiB)")
		assert.Contains(t, result, "Memory Free (MiB)")
		assert.Contains(t, result, "Swap Used (MiB)")
	})
}

func TestExtractMemoryTypeSeriesData(t *testing.T) {
	t.Run("Extract memory type series data", func(t *testing.T) {
		data := &TTopReportData{
			Snapshots: []TTopSnapshot{
				{
					SystemMemory: &SystemMemory{
						MemTotal:     8000.0,
						MemFree:      4000.0,
						MemUsed:      3000.0,
						MemBuffCache: 1000.0,
						SwapTotal:    1024.0,
						SwapFree:     512.0,
						SwapUsed:     512.0,
						MemAvail:     5000.0,
					},
				},
			},
		}

		result := extractMemoryTypeSeriesData(data)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Memory Used (MiB)")
		assert.Contains(t, result, "Buffer/Cache (MiB)")
		assert.Contains(t, result, "Memory Free (MiB)")
		assert.Contains(t, result, "Swap Used (MiB)")
		assert.Contains(t, result, "data: [3000.0]") // Memory used
		assert.Contains(t, result, "data: [1000.0]") // Buffer/cache
		assert.Contains(t, result, "data: [4000.0]") // Memory free
		assert.Contains(t, result, "data: [512.0]")  // Swap used
	})
}

func TestExtractThreadTypeLegendData(t *testing.T) {
	t.Run("Extract thread type legend data", func(t *testing.T) {
		data := &TTopReportData{
			Snapshots: []TTopSnapshot{
				{
					ThreadCounts: &ThreadCounts{
						Total:    10,
						Running:  2,
						Sleeping: 8,
						Stopped:  0,
						Zombie:   0,
					},
				},
			},
		}

		result := extractThreadTypeLegendData(data)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Total Threads")
		assert.Contains(t, result, "Running Threads")
		assert.Contains(t, result, "Sleeping Threads")
		// Should not contain stopped or zombie since they are 0
		assert.NotContains(t, result, "Stopped Threads")
		assert.NotContains(t, result, "Zombie Threads")
	})
}

func TestExtractThreadTypeSeriesData(t *testing.T) {
	t.Run("Extract thread type series data", func(t *testing.T) {
		data := &TTopReportData{
			Snapshots: []TTopSnapshot{
				{
					ThreadCounts: &ThreadCounts{
						Total:    10,
						Running:  2,
						Sleeping: 8,
						Stopped:  0,
						Zombie:   0,
					},
				},
			},
		}

		result := extractThreadTypeSeriesData(data)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Total Threads")
		assert.Contains(t, result, "Running Threads")
		assert.Contains(t, result, "Sleeping Threads")
		assert.Contains(t, result, "data: [10]") // Total threads
		assert.Contains(t, result, "data: [2]")  // Running threads
		assert.Contains(t, result, "data: [8]")  // Sleeping threads
		// Should not contain stopped or zombie since they are 0
		assert.NotContains(t, result, "Stopped Threads")
		assert.NotContains(t, result, "Zombie Threads")
	})
}

func TestExtractThreadByCPUData(t *testing.T) {
	t.Run("Extract thread by CPU data", func(t *testing.T) {
		data := &TTopReportData{
			Snapshots: []TTopSnapshot{
				{
					Threads: []ThreadInfo{
						{PID: 1234, Command: "java", CPU: 25.5},
						{PID: 5678, Command: "compiler", CPU: 15.0},
					},
				},
				{
					Threads: []ThreadInfo{
						{PID: 1234, Command: "java", CPU: 30.0},
						{PID: 5678, Command: "compiler", CPU: 20.0},
					},
				},
			},
		}

		legendResult := extractThreadByCPULegendData(data)
		assert.NotEmpty(t, legendResult)
		assert.Contains(t, legendResult, "java-1234")
		assert.Contains(t, legendResult, "compiler-5678")

		seriesResult := extractThreadByCPUSeriesData(data)
		assert.NotEmpty(t, seriesResult)
		assert.Contains(t, seriesResult, "java-1234")
		assert.Contains(t, seriesResult, "compiler-5678")
		assert.Contains(t, seriesResult, "25.5, 30.0") // CPU values across snapshots
		assert.Contains(t, seriesResult, "15.0, 20.0") // CPU values across snapshots
	})
}

func TestEscapeJSONString(t *testing.T) {
	t.Run("Escape JSON string", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"simple", "simple"},
			{"with\"quotes", "with\\\"quotes"},
			{"with\\backslash", "with\\\\backslash"},
			{"with\nnewline", "with\\nnewline"},
			{"with\ttab", "with\\ttab"},
			{"with\rcarriage", "with\\rcarriage"},
		}

		for _, test := range tests {
			result := escapeJSONString(test.input)
			assert.Equal(t, test.expected, result)
		}
	})
}
