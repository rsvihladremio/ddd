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

// GenerateTTopHTML generates a self-contained HTML report with three charts:
// 1. Total thread count over time
// 2. CPU usage per thread (top 5 busiest threads) over time
// 3. Memory usage per user over time
func GenerateTTopHTML(data *TTopReportData) (string, error) {
	if data == nil || len(data.Snapshots) == 0 {
		return generateEmptyHTML(), nil
	}

	// Prepare data for charts
	labels := extractTimeLabels(data)
	threadCountData := extractThreadCountData(data)
	cpuData := extractTop5CPUData(data)
	memoryData := extractMemoryByUserData(data)

	// Build the complete HTML document
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>TTop Analysis Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
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
        canvas {
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
            <canvas id="threadCountChart"></canvas>
        </div>

        <div class="chart-container">
            <div class="chart-title">CPU Usage - Top 5 Busiest Threads</div>
            <canvas id="cpuChart"></canvas>
        </div>

        <div class="chart-container">
            <div class="chart-title">Memory Usage by User</div>
            <canvas id="memoryChart"></canvas>
        </div>
    </div>

    <script>
        // Chart.js configuration and data
        const chartOptions = {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'top',
                },
            },
            scales: {
                y: {
                    beginAtZero: true
                }
            }
        };

        // Thread Count Chart
        const threadCountCtx = document.getElementById('threadCountChart').getContext('2d');
        new Chart(threadCountCtx, {
            type: 'line',
            data: {
                labels: %s,
                datasets: [{
                    label: 'Thread Count',
                    data: %s,
                    borderColor: 'rgb(75, 192, 192)',
                    backgroundColor: 'rgba(75, 192, 192, 0.2)',
                    tension: 0.1
                }]
            },
            options: chartOptions
        });

        // CPU Usage Chart
        const cpuCtx = document.getElementById('cpuChart').getContext('2d');
        new Chart(cpuCtx, {
            type: 'line',
            data: {
                labels: %s,
                datasets: %s
            },
            options: {
                ...chartOptions,
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'CPU Usage (%%)'
                        }
                    }
                }
            }
        });

        // Memory Usage Chart
        const memoryCtx = document.getElementById('memoryChart').getContext('2d');
        new Chart(memoryCtx, {
            type: 'bar',
            data: {
                labels: %s,
                datasets: %s
            },
            options: {
                ...chartOptions,
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'Memory Usage (%%)'
                        }
                    }
                }
            }
        });
    </script>
</body>
</html>`, 
		len(data.Snapshots), 
		countUniqueThreads(data), 
		findPeakThreadCount(data),
		labels,
		threadCountData,
		labels,
		cpuData,
		labels,
		memoryData)

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

// extractTop5CPUData extracts CPU usage data for the top 5 busiest threads
func extractTop5CPUData(data *TTopReportData) string {
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
	
	// Generate Chart.js dataset for each of the top 5 threads
	var datasets []string
	colors := []string{
		"rgb(255, 99, 132)", "rgb(54, 162, 235)", "rgb(255, 205, 86)", 
		"rgb(75, 192, 192)", "rgb(153, 102, 255)",
	}
	
	for i, pair := range pairs {
		color := colors[i%len(colors)]
		
		// Extract data for this thread across all snapshots
		var threadData []string
		for _, snapshot := range data.Snapshots {
			cpu := 0.0
			for _, thread := range snapshot.Threads {
				threadKey := fmt.Sprintf("%s (PID: %d)", thread.Command, thread.PID)
				if threadKey == pair.key {
					cpu = thread.CPU
					break
				}
			}
			threadData = append(threadData, fmt.Sprintf("%.1f", cpu))
		}
		
		dataset := fmt.Sprintf(`{
			label: "%s",
			data: [%s],
			borderColor: "%s",
			backgroundColor: "%s",
			tension: 0.1
		}`, escapeJSONString(pair.key), strings.Join(threadData, ", "), color, color+"40")
		
		datasets = append(datasets, dataset)
	}
	
	return fmt.Sprintf("[%s]", strings.Join(datasets, ", "))
}

// extractMemoryByUserData extracts memory usage data grouped by user
func extractMemoryByUserData(data *TTopReportData) string {
	// Collect all unique users
	users := make(map[string]bool)
	for _, snapshot := range data.Snapshots {
		for _, thread := range snapshot.Threads {
			users[thread.User] = true
		}
	}
	
	// Generate Chart.js dataset for each user
	var datasets []string
	colors := []string{
		"rgb(255, 99, 132)", "rgb(54, 162, 235)", "rgb(255, 205, 86)", 
		"rgb(75, 192, 192)", "rgb(153, 102, 255)", "rgb(255, 159, 64)",
	}
	
	i := 0
	for user := range users {
		color := colors[i%len(colors)]
		
		// Extract memory data for this user across all snapshots
		var userData []string
		for _, snapshot := range data.Snapshots {
			totalMem := 0.0
			for _, thread := range snapshot.Threads {
				if thread.User == user {
					totalMem += thread.MEM
				}
			}
			userData = append(userData, fmt.Sprintf("%.1f", totalMem))
		}
		
		dataset := fmt.Sprintf(`{
			label: "%s",
			data: [%s],
			backgroundColor: "%s",
			borderColor: "%s",
			borderWidth: 1
		}`, escapeJSONString(user), strings.Join(userData, ", "), color+"80", color)
		
		datasets = append(datasets, dataset)
		i++
	}
	
	return fmt.Sprintf("[%s]", strings.Join(datasets, ", "))
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
