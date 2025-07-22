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
	"fmt"
	"sort"
	"strings"
)

// GenerateTTopHTML generates a self-contained HTML report with three charts:
// 1. Threads by Name/ID CPU Usage Over Time
// 2. System Memory Usage Over Time (using global memory data from ttop header)
// 3. Thread States Over Time (using global thread counts from ttop header)
func GenerateTTopHTML(data *TTopReportData) (string, error) {
	if data == nil || len(data.Snapshots) == 0 {
		return generateEmptyHTML(), nil
	}

	// Prepare data for charts
	labels := extractTimeLabels(data)
	threadByCPUData := extractThreadByCPUSeriesData(data)
	memoryByTypeData := extractMemoryTypeSeriesData(data)
	threadsByTypeData := extractThreadTypeSeriesData(data)

	// Build the complete HTML document
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>TTop Analysis Report</title>
    <script src="/static/js/echarts.min.js"></script>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            background-color: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #06b6d4 0%%, #0891b2 100%%);
            color: white;
            padding: 30px;
            text-align: center;
        }
        .header h1 {
            margin: 0 0 10px 0;
            font-size: 2.5em;
            font-weight: 300;
        }
        .header p {
            margin: 0;
            font-size: 1.1em;
            opacity: 0.9;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            padding: 30px;
            background-color: #f8f9fa;
        }
        .stat-card {
            background: white;
            padding: 20px;
            border-radius: 8px;
            text-align: center;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .stat-value {
            font-size: 2em;
            font-weight: bold;
            color: #06b6d4;
            margin-bottom: 5px;
        }
        .stat-label {
            color: #666;
            font-size: 0.9em;
        }
        .chart-container {
            padding: 30px;
            border-bottom: 1px solid #eee;
        }
        .chart-container:last-child {
            border-bottom: none;
        }
        .chart-title {
            font-size: 1.5em;
            margin-bottom: 20px;
            color: #333;
            text-align: center;
        }
        .chart {
            width: 100%%;
            height: 400px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>TTop Analysis Report</h1>
            <p>Thread Activity Performance Analysis</p>
        </div>

        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Snapshots</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Unique Threads</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Peak Thread Count</div>
            </div>
        </div>

        <div class="chart-container">
            <div class="chart-title">Thread CPU Usage Over Time</div>
            <div id="threadByCpuChart" class="chart"></div>
        </div>

        <div class="chart-container">
            <div class="chart-title">System Memory Usage Over Time</div>
            <div id="memoryByTypeChart" class="chart"></div>
        </div>

        <div class="chart-container">
            <div class="chart-title">Thread States Over Time</div>
            <div id="threadsByTypeChart" class="chart"></div>
        </div>
    </div>

    <script>
        console.log('Initializing charts...');
        try {
                // Thread by CPU Chart
                const threadByCpuChart = echarts.init(document.getElementById('threadByCpuChart'));
                const threadByCpuOption = {
                    title: { text: 'Threads by Name/ID CPU Usage Over Time' },
                    tooltip: { trigger: 'axis' },
                    legend: { data: [] },
                    toolbox: {
                        show: true,
                        feature: {
                            saveAsImage: {
                                show: true,
                                title: 'Save as Image',
                                type: 'png',
                                name: 'thread_cpu_usage'
                            },
                            dataView: {
                                show: true,
                                title: 'Data View',
                                readOnly: false
                            },
                            dataZoom: {
                                show: true,
                                title: { zoom: 'Zoom', back: 'Reset Zoom' }
                            },
                            restore: {
                                show: true,
                                title: 'Restore'
                            },
                            magicType: {
                                show: true,
                                type: ['line', 'bar'],
                                title: { line: 'Line Chart', bar: 'Bar Chart' }
                            }
                        }
                    },
                    dataZoom: [
                        {
                            type: 'slider',
                            show: true,
                            xAxisIndex: [0],
                            start: 0,
                            end: 100
                        },
                        {
                            type: 'inside',
                            xAxisIndex: [0],
                            start: 0,
                            end: 100
                        }
                    ],
                    xAxis: { type: 'category', data: %s },
                    yAxis: { type: 'value', name: 'CPU Usage (%%)', min: 0 },
                    series: %s
                };
                threadByCpuChart.setOption(threadByCpuOption);

                // Memory by Type Chart
                const memoryByTypeChart = echarts.init(document.getElementById('memoryByTypeChart'));
                const memoryByTypeOption = {
                    title: { text: 'System Memory Usage Over Time' },
                    tooltip: {
                        trigger: 'axis',
                        formatter: function (params) {
                            let result = params[0].name + '<br/>';
                            params.forEach(function (item) {
                                result += item.marker + ' ' + item.seriesName + ': ' + item.value + ' MiB<br/>';
                            });
                            return result;
                        }
                    },
                    legend: { data: [] },
                    toolbox: {
                        show: true,
                        feature: {
                            saveAsImage: {
                                show: true,
                                title: 'Save as Image',
                                type: 'png',
                                name: 'system_memory_usage'
                            },
                            dataView: {
                                show: true,
                                title: 'Data View',
                                readOnly: false
                            },
                            dataZoom: {
                                show: true,
                                title: { zoom: 'Zoom', back: 'Reset Zoom' }
                            },
                            restore: {
                                show: true,
                                title: 'Restore'
                            },
                            magicType: {
                                show: true,
                                type: ['line', 'bar', 'stack'],
                                title: { line: 'Line Chart', bar: 'Bar Chart', stack: 'Stacked' }
                            }
                        }
                    },
                    dataZoom: [
                        {
                            type: 'slider',
                            show: true,
                            xAxisIndex: [0],
                            start: 0,
                            end: 100
                        },
                        {
                            type: 'inside',
                            xAxisIndex: [0],
                            start: 0,
                            end: 100
                        }
                    ],
                    xAxis: { type: 'category', data: %s },
                    yAxis: { type: 'value', name: 'Memory (MiB)', min: 0 },
                    series: %s
                };
                memoryByTypeChart.setOption(memoryByTypeOption);

                // Threads by Type Chart
                const threadsByTypeChart = echarts.init(document.getElementById('threadsByTypeChart'));
                const threadsByTypeOption = {
                    title: { text: 'Thread States Over Time' },
                    tooltip: {
                        trigger: 'axis',
                        formatter: function (params) {
                            let result = params[0].name + '<br/>';
                            params.forEach(function (item) {
                                result += item.marker + ' ' + item.seriesName + ': ' + item.value + '<br/>';
                            });
                            return result;
                        }
                    },
                    legend: { data: [] },
                    toolbox: {
                        show: true,
                        feature: {
                            saveAsImage: {
                                show: true,
                                title: 'Save as Image',
                                type: 'png',
                                name: 'thread_states'
                            },
                            dataView: {
                                show: true,
                                title: 'Data View',
                                readOnly: false
                            },
                            dataZoom: {
                                show: true,
                                title: { zoom: 'Zoom', back: 'Reset Zoom' }
                            },
                            restore: {
                                show: true,
                                title: 'Restore'
                            },
                            magicType: {
                                show: true,
                                type: ['line', 'bar'],
                                title: { line: 'Line Chart', bar: 'Bar Chart' }
                            }
                        }
                    },
                    dataZoom: [
                        {
                            type: 'slider',
                            show: true,
                            xAxisIndex: [0],
                            start: 0,
                            end: 100
                        },
                        {
                            type: 'inside',
                            xAxisIndex: [0],
                            start: 0,
                            end: 100
                        }
                    ],
                    xAxis: { type: 'category', data: %s },
                    yAxis: { type: 'value', name: 'Thread Count', min: 0 },
                    series: %s
                };
                threadsByTypeChart.setOption(threadsByTypeOption);

                // Handle window resize
                window.addEventListener('resize', function() {
                    threadByCpuChart.resize();
                    memoryByTypeChart.resize();
                    threadsByTypeChart.resize();
                });

        } catch (error) {
            console.error('Error initializing charts:', error);
            document.body.innerHTML += '<div style="color: red; padding: 20px; background: #ffe6e6; border: 1px solid red; margin: 20px;">Error initializing charts: ' + error.message + '</div>';
        }
    </script>
</body>
</html>`,
		len(data.Snapshots),
		countUniqueThreads(data),
		findPeakThreadCount(data),
		labels,
		threadByCPUData,
		labels,
		memoryByTypeData,
		labels,
		threadsByTypeData)

	return html, nil
}

// generateEmptyHTML returns HTML for when no data is available
func generateEmptyHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>TTop Analysis Report</title>
</head>
<body>
    <h1>TTop Analysis Report</h1>
    <p>No data available for analysis.</p>
</body>
</html>`
}

// extractTimeLabels creates time labels for the x-axis of charts
func extractTimeLabels(data *TTopReportData) string {
	var labels []string
	for _, snapshot := range data.Snapshots {
		labels = append(labels, fmt.Sprintf(`"%s"`, snapshot.Timestamp.Format("15:04:05")))
	}
	return fmt.Sprintf("[%s]", strings.Join(labels, ", "))
}

// countUniqueThreads counts the total number of unique threads across all snapshots
func countUniqueThreads(data *TTopReportData) int {
	threads := make(map[int]bool)
	for _, snapshot := range data.Snapshots {
		for _, thread := range snapshot.Threads {
			threads[thread.PID] = true
		}
	}
	return len(threads)
}

// findPeakThreadCount finds the maximum number of threads in any single snapshot
func findPeakThreadCount(data *TTopReportData) int {
	peak := 0
	for _, snapshot := range data.Snapshots {
		if len(snapshot.Threads) > peak {
			peak = len(snapshot.Threads)
		}
	}
	return peak
}

// escapeJSONString escapes special characters in strings for JSON output
func escapeJSONString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// extractCPULegendData extracts legend data for CPU chart (top 5 threads)
func extractCPULegendData(data *TTopReportData) []string {
	// Find the top 5 busiest threads across all snapshots
	threadCPU := make(map[string]float64)
	for _, snapshot := range data.Snapshots {
		for _, thread := range snapshot.Threads {
			key := fmt.Sprintf("%s (PID: %d)", thread.Command, thread.PID)
			if cpu, exists := threadCPU[key]; !exists || thread.CPU > cpu {
				threadCPU[key] = thread.CPU
			}
		}
	}

	// Sort threads by CPU usage and get top 5
	type threadCPUPair struct {
		key string
		cpu float64
	}

	var pairs []threadCPUPair
	for key, cpu := range threadCPU {
		pairs = append(pairs, threadCPUPair{key, cpu})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].cpu > pairs[j].cpu
	})

	// Limit to top 5
	if len(pairs) > 5 {
		pairs = pairs[:5]
	}

	var result []string
	for _, pair := range pairs {
		result = append(result, pair.key)
	}
	return result
}

// extractMemoryTypeLegendData extracts legend data for memory type chart using system memory data
func extractMemoryTypeLegendData(data *TTopReportData) []string {
	// Check what types of memory data we have from the system memory information
	hasBuffCache, hasSwapUsed := false, false

	for _, snapshot := range data.Snapshots {
		if snapshot.SystemMemory != nil {
			if snapshot.SystemMemory.MemBuffCache > 0 {
				hasBuffCache = true
			}
			if snapshot.SystemMemory.SwapUsed > 0 {
				hasSwapUsed = true
			}
		}
	}

	var result []string
	// Always include basic memory types
	result = append(result, "Memory Used (MiB)")

	if hasBuffCache {
		result = append(result, "Buffer/Cache (MiB)")
	}

	result = append(result, "Memory Free (MiB)")

	if hasSwapUsed {
		result = append(result, "Swap Used (MiB)")
	}

	return result
}

// extractMemoryTypeSeriesData extracts series data for memory type chart using system memory information
func extractMemoryTypeSeriesData(data *TTopReportData) string {
	// Use system memory data from the "MiB Mem:" and "MiB Swap:" lines
	var memUsedSeries, memFreeSeries, memBuffCacheSeries, swapUsedSeries []string

	for _, snapshot := range data.Snapshots {
		// Use system memory data if available, otherwise default to 0
		if snapshot.SystemMemory != nil {
			memUsedSeries = append(memUsedSeries, fmt.Sprintf("%.1f", snapshot.SystemMemory.MemUsed))
			memFreeSeries = append(memFreeSeries, fmt.Sprintf("%.1f", snapshot.SystemMemory.MemFree))
			memBuffCacheSeries = append(memBuffCacheSeries, fmt.Sprintf("%.1f", snapshot.SystemMemory.MemBuffCache))
			swapUsedSeries = append(swapUsedSeries, fmt.Sprintf("%.1f", snapshot.SystemMemory.SwapUsed))
		} else {
			// Fallback to 0 if system memory data is not available
			memUsedSeries = append(memUsedSeries, "0.0")
			memFreeSeries = append(memFreeSeries, "0.0")
			memBuffCacheSeries = append(memBuffCacheSeries, "0.0")
			swapUsedSeries = append(swapUsedSeries, "0.0")
		}
	}

	var datasets []string

	// Always include memory used
	datasets = append(datasets, fmt.Sprintf(`{
		name: "Memory Used (MiB)",
		type: "bar",
		stack: "memory",
		data: [%s]
	}`, strings.Join(memUsedSeries, ", ")))

	// Include buffer/cache if there are any non-zero values
	if hasNonZeroFloatValues(memBuffCacheSeries) {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "Buffer/Cache (MiB)",
			type: "bar",
			stack: "memory",
			data: [%s]
		}`, strings.Join(memBuffCacheSeries, ", ")))
	}

	// Include memory free
	datasets = append(datasets, fmt.Sprintf(`{
		name: "Memory Free (MiB)",
		type: "bar",
		stack: "memory",
		data: [%s]
	}`, strings.Join(memFreeSeries, ", ")))

	// Include swap used if there are any non-zero values
	if hasNonZeroFloatValues(swapUsedSeries) {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "Swap Used (MiB)",
			type: "bar",
			stack: "swap",
			data: [%s]
		}`, strings.Join(swapUsedSeries, ", ")))
	}

	return fmt.Sprintf("[%s]", strings.Join(datasets, ", "))
}

// extractThreadTypeLegendData extracts legend data for thread type chart using global thread counts
func extractThreadTypeLegendData(data *TTopReportData) []string {
	// Check what types of thread counts we have from the global thread counts
	hasRunning, hasSleeping, hasStopped, hasZombie := false, false, false, false

	for _, snapshot := range data.Snapshots {
		if snapshot.ThreadCounts != nil {
			if snapshot.ThreadCounts.Running > 0 {
				hasRunning = true
			}
			if snapshot.ThreadCounts.Sleeping > 0 {
				hasSleeping = true
			}
			if snapshot.ThreadCounts.Stopped > 0 {
				hasStopped = true
			}
			if snapshot.ThreadCounts.Zombie > 0 {
				hasZombie = true
			}
		}
	}

	var result []string
	// Always include total threads
	result = append(result, "Total Threads")

	if hasRunning {
		result = append(result, "Running Threads")
	}
	if hasSleeping {
		result = append(result, "Sleeping Threads")
	}
	if hasStopped {
		result = append(result, "Stopped Threads")
	}
	if hasZombie {
		result = append(result, "Zombie Threads")
	}
	return result
}

// extractThreadTypeSeriesData extracts series data for thread type chart using global thread counts
func extractThreadTypeSeriesData(data *TTopReportData) string {
	// Use global thread counts from the "Threads:" line instead of categorizing individual threads
	var totalSeries, runningSeries, sleepingSeries, stoppedSeries, zombieSeries []string

	for _, snapshot := range data.Snapshots {
		// Use global thread counts if available, otherwise default to 0
		if snapshot.ThreadCounts != nil {
			totalSeries = append(totalSeries, fmt.Sprintf("%d", snapshot.ThreadCounts.Total))
			runningSeries = append(runningSeries, fmt.Sprintf("%d", snapshot.ThreadCounts.Running))
			sleepingSeries = append(sleepingSeries, fmt.Sprintf("%d", snapshot.ThreadCounts.Sleeping))
			stoppedSeries = append(stoppedSeries, fmt.Sprintf("%d", snapshot.ThreadCounts.Stopped))
			zombieSeries = append(zombieSeries, fmt.Sprintf("%d", snapshot.ThreadCounts.Zombie))
		} else {
			// Fallback to 0 if thread counts are not available
			totalSeries = append(totalSeries, "0")
			runningSeries = append(runningSeries, "0")
			sleepingSeries = append(sleepingSeries, "0")
			stoppedSeries = append(stoppedSeries, "0")
			zombieSeries = append(zombieSeries, "0")
		}
	}

	var datasets []string

	// Always include total threads
	datasets = append(datasets, fmt.Sprintf(`{
		name: "Total Threads",
		type: "line",
		data: [%s]
	}`, strings.Join(totalSeries, ", ")))

	// Include running threads if there are any non-zero values
	if hasNonZeroValues(runningSeries) {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "Running Threads",
			type: "line",
			data: [%s]
		}`, strings.Join(runningSeries, ", ")))
	}

	// Include sleeping threads if there are any non-zero values
	if hasNonZeroValues(sleepingSeries) {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "Sleeping Threads",
			type: "line",
			data: [%s]
		}`, strings.Join(sleepingSeries, ", ")))
	}

	// Include stopped threads if there are any non-zero values
	if hasNonZeroValues(stoppedSeries) {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "Stopped Threads",
			type: "line",
			data: [%s]
		}`, strings.Join(stoppedSeries, ", ")))
	}

	// Include zombie threads if there are any non-zero values
	if hasNonZeroValues(zombieSeries) {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "Zombie Threads",
			type: "line",
			data: [%s]
		}`, strings.Join(zombieSeries, ", ")))
	}

	return fmt.Sprintf("[%s]", strings.Join(datasets, ", "))
}

// hasNonZeroValues checks if a series contains any non-zero values
func hasNonZeroValues(series []string) bool {
	for _, value := range series {
		if value != "0" {
			return true
		}
	}
	return false
}

// hasNonZeroFloatValues checks if a series contains any non-zero float values
func hasNonZeroFloatValues(series []string) bool {
	for _, value := range series {
		if value != "0.0" && value != "0" {
			return true
		}
	}
	return false
}

// extractThreadByCPULegendData extracts legend data for thread by CPU chart
func extractThreadByCPULegendData(data *TTopReportData) []string {
	// Find the top 5 busiest threads across all snapshots
	threadCPU := make(map[string]float64)
	for _, snapshot := range data.Snapshots {
		for _, thread := range snapshot.Threads {
			key := fmt.Sprintf("%s-%d", thread.Command, thread.PID)
			if cpu, exists := threadCPU[key]; !exists || thread.CPU > cpu {
				threadCPU[key] = thread.CPU
			}
		}
	}

	// Sort threads by CPU usage and get top 5
	type threadCPUPair struct {
		key string
		cpu float64
	}

	var pairs []threadCPUPair
	for key, cpu := range threadCPU {
		pairs = append(pairs, threadCPUPair{key, cpu})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].cpu > pairs[j].cpu
	})

	// Limit to top 5
	if len(pairs) > 5 {
		pairs = pairs[:5]
	}

	var result []string
	for _, pair := range pairs {
		result = append(result, pair.key)
	}
	return result
}

// extractThreadByCPUSeriesData extracts series data for thread by CPU chart
func extractThreadByCPUSeriesData(data *TTopReportData) string {
	// Find the top 5 busiest threads across all snapshots
	threadCPU := make(map[string]float64)
	for _, snapshot := range data.Snapshots {
		for _, thread := range snapshot.Threads {
			key := fmt.Sprintf("%s-%d", thread.Command, thread.PID)
			if cpu, exists := threadCPU[key]; !exists || thread.CPU > cpu {
				threadCPU[key] = thread.CPU
			}
		}
	}

	// Sort threads by CPU usage and get top 5
	type threadCPUPair struct {
		key string
		cpu float64
	}

	var pairs []threadCPUPair
	for key, cpu := range threadCPU {
		pairs = append(pairs, threadCPUPair{key, cpu})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].cpu > pairs[j].cpu
	})

	// Limit to top 5
	if len(pairs) > 5 {
		pairs = pairs[:5]
	}

	// Generate series data for each thread
	var datasets []string
	for _, pair := range pairs {
		var threadData []string
		for _, snapshot := range data.Snapshots {
			cpu := 0.0
			for _, thread := range snapshot.Threads {
				threadKey := fmt.Sprintf("%s-%d", thread.Command, thread.PID)
				if threadKey == pair.key {
					cpu = thread.CPU
					break
				}
			}
			threadData = append(threadData, fmt.Sprintf("%.1f", cpu))
		}

		datasets = append(datasets, fmt.Sprintf(`{
			name: "%s",
			type: "line",
			data: [%s]
		}`, escapeJSONString(pair.key), strings.Join(threadData, ", ")))
	}

	return fmt.Sprintf("[%s]", strings.Join(datasets, ", "))
}
