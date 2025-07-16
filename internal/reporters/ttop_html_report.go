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
	"fmt"
	"sort"
	"strings"
)

// GenerateTTopHTML generates a self-contained HTML report with four charts:
// 1. Total thread count over time
// 2. Threads by Name/ID CPU Usage Over Time
// 3. Memory Usage by Memory Type Over Time
// 4. Total Threads by Type Over Time
func GenerateTTopHTML(data *TTopReportData) (string, error) {
	if data == nil || len(data.Snapshots) == 0 {
		return generateEmptyHTML(), nil
	}

	// Prepare data for charts
	labels := extractTimeLabels(data)
	threadCountData := extractThreadCountData(data)
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
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background-color: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1 {
            color: #333;
            text-align: center;
            margin-bottom: 30px;
        }
        .chart-container {
            margin-bottom: 40px;
            padding: 20px;
            border: 1px solid #ddd;
            border-radius: 6px;
            background-color: #fafafa;
        }
        .chart-title {
            font-size: 18px;
            font-weight: bold;
            margin-bottom: 15px;
            color: #555;
        }
        .chart {
            max-width: 100%%;
            height: 400px;
        }
        .summary {
            background-color: #e8f4f8;
            padding: 15px;
            border-radius: 6px;
            margin-bottom: 20px;
        }
        .summary h3 {
            margin-top: 0;
            color: #2c5282;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>TTop Analysis Report</h1>
        
        <div class="summary">
            <h3>Summary</h3>
            <p>Analysis of %d snapshots covering thread activity over time.</p>
            <p>Total unique threads observed: %d</p>
            <p>Peak thread count: %d</p>
        </div>

        <div class="chart-container">
            <div class="chart-title">Thread Count Over Time</div>
            <div id="threadCountChart" style="width: 100%%; height: 400px;"></div>
        </div>

        <div class="chart-container">
            <div class="chart-title">Threads by Name/ID CPU Usage Over Time</div>
            <div id="threadByCpuChart" style="width: 100%%; height: 400px;"></div>
        </div>

        <div class="chart-container">
            <div class="chart-title">Memory Usage by Memory Type Over Time</div>
            <div id="memoryByTypeChart" style="width: 100%%; height: 400px;"></div>
        </div>

        <div class="chart-container">
            <div class="chart-title">Total Threads by Type Over Time</div>
            <div id="threadsByTypeChart" style="width: 100%%; height: 400px;"></div>
        </div>
    </div>

    <script>
        console.log('Initializing charts...');
        try {
                // Thread Count Chart
                const threadCountChart = echarts.init(document.getElementById('threadCountChart'));
                const threadCountOption = {
                    title: { text: 'Thread Count Over Time' },
                    tooltip: { trigger: 'axis' },
                    legend: { data: ['Thread Count'] },
                    xAxis: { type: 'category', data: %s },
                    yAxis: { type: 'value' },
                    series: [{
                        name: 'Thread Count',
                        type: 'line',
                        data: %s
                    }]
                };
                threadCountChart.setOption(threadCountOption);

                // Thread by CPU Chart
                const threadByCpuChart = echarts.init(document.getElementById('threadByCpuChart'));
                const threadByCpuOption = {
                    title: { text: 'Threads by Name/ID CPU Usage Over Time' },
                    tooltip: { trigger: 'axis' },
                    legend: { data: [] },
                    xAxis: { type: 'category', data: %s },
                    yAxis: { type: 'value', name: 'CPU Usage (%%)', min: 0 },
                    series: %s
                };
                threadByCpuChart.setOption(threadByCpuOption);

                // Memory by Type Chart
                const memoryByTypeChart = echarts.init(document.getElementById('memoryByTypeChart'));
                const memoryByTypeOption = {
                    title: { text: 'Memory Usage by Memory Type Over Time' },
                    tooltip: { trigger: 'axis' },
                    legend: { data: [] },
                    xAxis: { type: 'category', data: %s },
                    yAxis: { type: 'value', name: 'Thread Count', min: 0 },
                    series: %s
                };
                memoryByTypeChart.setOption(memoryByTypeOption);

                // Threads by Type Chart
                const threadsByTypeChart = echarts.init(document.getElementById('threadsByTypeChart'));
                const threadsByTypeOption = {
                    title: { text: 'Total Threads by Type Over Time' },
                    tooltip: { trigger: 'axis' },
                    legend: { data: [] },
                    xAxis: { type: 'category', data: %s },
                    yAxis: { type: 'value', name: 'Thread Count', min: 0 },
                    series: %s
                };
                threadsByTypeChart.setOption(threadsByTypeOption);

                // Handle window resize
                window.addEventListener('resize', function() {
                    threadCountChart.resize();
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
		threadCountData,
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

// extractThreadCountData extracts thread count for each snapshot
func extractThreadCountData(data *TTopReportData) string {
	var counts []string
	for _, snapshot := range data.Snapshots {
		counts = append(counts, fmt.Sprintf("%d", len(snapshot.Threads)))
	}
	return fmt.Sprintf("[%s]", strings.Join(counts, ", "))
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

// extractMemoryTypeLegendData extracts legend data for memory type chart
func extractMemoryTypeLegendData(data *TTopReportData) []string {
	// Check if we have threads with different memory usage levels
	hasLow, hasMedium, hasHigh := false, false, false

	for _, snapshot := range data.Snapshots {
		for _, thread := range snapshot.Threads {
			if thread.MEM < 5.0 {
				hasLow = true
			} else if thread.MEM <= 15.0 {
				hasMedium = true
			} else {
				hasHigh = true
			}
		}
	}

	var result []string
	if hasLow {
		result = append(result, "Low Memory (<5%)")
	}
	if hasMedium {
		result = append(result, "Medium Memory (5-15%)")
	}
	if hasHigh {
		result = append(result, "High Memory (>15%)")
	}
	return result
}

// extractMemoryTypeSeriesData extracts series data for memory type chart
func extractMemoryTypeSeriesData(data *TTopReportData) string {
	// Count threads by memory type for each snapshot
	var lowMemorySeries, mediumMemorySeries, highMemorySeries []string

	for _, snapshot := range data.Snapshots {
		lowCount, mediumCount, highCount := 0, 0, 0

		for _, thread := range snapshot.Threads {
			if thread.MEM < 5.0 {
				lowCount++
			} else if thread.MEM <= 15.0 {
				mediumCount++
			} else {
				highCount++
			}
		}

		lowMemorySeries = append(lowMemorySeries, fmt.Sprintf("%d", lowCount))
		mediumMemorySeries = append(mediumMemorySeries, fmt.Sprintf("%d", mediumCount))
		highMemorySeries = append(highMemorySeries, fmt.Sprintf("%d", highCount))
	}

	var datasets []string

	if len(lowMemorySeries) > 0 {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "Low Memory (<5%%)",
			type: "bar",
			data: [%s]
		}`, strings.Join(lowMemorySeries, ", ")))
	}

	if len(mediumMemorySeries) > 0 {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "Medium Memory (5-15%%)",
			type: "bar",
			data: [%s]
		}`, strings.Join(mediumMemorySeries, ", ")))
	}

	if len(highMemorySeries) > 0 {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "High Memory (>15%%)",
			type: "bar",
			data: [%s]
		}`, strings.Join(highMemorySeries, ", ")))
	}

	return fmt.Sprintf("[%s]", strings.Join(datasets, ", "))
}

// extractThreadTypeLegendData extracts legend data for thread type chart
func extractThreadTypeLegendData(data *TTopReportData) []string {
	// Check what types of threads we have
	hasJava, hasCompiler, hasSystem, hasOther := false, false, false, false

	for _, snapshot := range data.Snapshots {
		for _, thread := range snapshot.Threads {
			command := strings.ToLower(thread.Command)
			if strings.Contains(command, "java") {
				hasJava = true
			} else if strings.Contains(command, "compiler") || strings.Contains(command, "compile") {
				hasCompiler = true
			} else if strings.Contains(command, "system") || strings.Contains(command, "kernel") {
				hasSystem = true
			} else {
				hasOther = true
			}
		}
	}

	var result []string
	if hasJava {
		result = append(result, "Java Threads")
	}
	if hasCompiler {
		result = append(result, "Compiler Threads")
	}
	if hasSystem {
		result = append(result, "System Threads")
	}
	if hasOther {
		result = append(result, "Other Threads")
	}
	return result
}

// extractThreadTypeSeriesData extracts series data for thread type chart
func extractThreadTypeSeriesData(data *TTopReportData) string {
	// Count threads by type for each snapshot
	var javaSeries, compilerSeries, systemSeries, otherSeries []string

	for _, snapshot := range data.Snapshots {
		javaCount, compilerCount, systemCount, otherCount := 0, 0, 0, 0

		for _, thread := range snapshot.Threads {
			command := strings.ToLower(thread.Command)
			if strings.Contains(command, "java") {
				javaCount++
			} else if strings.Contains(command, "compiler") || strings.Contains(command, "compile") {
				compilerCount++
			} else if strings.Contains(command, "system") || strings.Contains(command, "kernel") {
				systemCount++
			} else {
				otherCount++
			}
		}

		javaSeries = append(javaSeries, fmt.Sprintf("%d", javaCount))
		compilerSeries = append(compilerSeries, fmt.Sprintf("%d", compilerCount))
		systemSeries = append(systemSeries, fmt.Sprintf("%d", systemCount))
		otherSeries = append(otherSeries, fmt.Sprintf("%d", otherCount))
	}

	var datasets []string

	if len(javaSeries) > 0 {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "Java Threads",
			type: "bar",
			data: [%s]
		}`, strings.Join(javaSeries, ", ")))
	}

	if len(compilerSeries) > 0 {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "Compiler Threads",
			type: "bar",
			data: [%s]
		}`, strings.Join(compilerSeries, ", ")))
	}

	if len(systemSeries) > 0 {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "System Threads",
			type: "bar",
			data: [%s]
		}`, strings.Join(systemSeries, ", ")))
	}

	if len(otherSeries) > 0 {
		datasets = append(datasets, fmt.Sprintf(`{
			name: "Other Threads",
			type: "bar",
			data: [%s]
		}`, strings.Join(otherSeries, ", ")))
	}

	return fmt.Sprintf("[%s]", strings.Join(datasets, ", "))
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
