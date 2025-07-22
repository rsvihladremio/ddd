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
	"strings"
)

// GenerateIOStatHTML generates a self-contained HTML report with three charts:
// 1. CPU Utilization Over Time
// 2. Device I/O Throughput Over Time
// 3. Device Utilization Over Time
func GenerateIOStatHTML(data *IOStatReportData) (string, error) {
	if data == nil || len(data.Snapshots) == 0 {
		return generateEmptyIOStatHTML(), nil
	}

	// Prepare data for charts
	labels := extractIOStatTimeLabels(data)
	cpuData := extractCPUSeriesData(data)
	ioThroughputData := extractIOThroughputSeriesData(data)

	// Generate HTML with embedded charts
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>IOStat Analysis Report</title>
    <script src="https://cdn.jsdelivr.net/npm/echarts@5.4.3/dist/echarts.min.js"></script>
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
            <h1>IOStat Analysis Report</h1>
            <p>System I/O Performance Analysis</p>
        </div>
        
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Snapshots</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Devices Monitored</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%.1f%%</div>
                <div class="stat-label">Peak CPU Usage</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%.1f</div>
                <div class="stat-label">Peak Device Avg. Queue Size</div>
            </div>
        </div>

        <div class="chart-container">
            <div class="chart-title">CPU Utilization Over Time</div>
            <div id="cpuChart" class="chart"></div>
        </div>

        <div class="chart-container">
            <div class="chart-title">Device I/O Throughput Over Time</div>
            <div id="ioThroughputChart" class="chart"></div>
        </div>



        <div class="chart-container">
            <div class="chart-title">Device I/O Await Times</div>
            <div id="deviceAwaitChart" class="chart"></div>
        </div>

        <div class="chart-container">
            <div class="chart-title">Device Average Queue Size</div>
            <div id="deviceQueueChart" class="chart"></div>
        </div>

        <div class="chart-container">
            <div class="chart-title">Device I/O Requests Per Second</div>
            <div id="deviceRequestsChart" class="chart"></div>
        </div>

        <div class="chart-container">
            <div class="chart-title">Device I/O Request Sizes</div>
            <div id="deviceRequestSizeChart" class="chart"></div>
        </div>


    </div>

    <script>
        try {
            // CPU Utilization Chart
            const cpuChart = echarts.init(document.getElementById('cpuChart'));
            const cpuOption = {
                tooltip: {
                    trigger: 'axis',
                    axisPointer: {
                        type: 'cross'
                    }
                },
                legend: {
                    data: ['User', 'System', 'IOWait', 'Idle']
                },
                grid: {
                    left: '3%%',
                    right: '4%%',
                    bottom: '3%%',
                    containLabel: true
                },
                xAxis: {
                    type: 'category',
                    boundaryGap: false,
                    data: %s
                },
                yAxis: {
                    type: 'value',
                    name: 'CPU %%',
                    min: 0,
                    max: 100
                },
                series: %s
            };
            cpuChart.setOption(cpuOption);

            // I/O Throughput Chart
            const ioThroughputChart = echarts.init(document.getElementById('ioThroughputChart'));
            const ioThroughputOption = {
                tooltip: {
                    trigger: 'axis',
                    axisPointer: {
                        type: 'cross'
                    }
                },
                legend: {
                    data: ['Read KB/s', 'Write KB/s']
                },
                grid: {
                    left: '3%%',
                    right: '4%%',
                    bottom: '3%%',
                    containLabel: true
                },
                xAxis: {
                    type: 'category',
                    boundaryGap: false,
                    data: %s
                },
                yAxis: {
                    type: 'value',
                    name: 'KB/s'
                },
                series: %s
            };
            ioThroughputChart.setOption(ioThroughputOption);



            // Device I/O Await Chart
            const deviceAwaitChart = echarts.init(document.getElementById('deviceAwaitChart'));
            const deviceAwaitOption = {
                tooltip: {
                    trigger: 'axis',
                    axisPointer: {
                        type: 'cross'
                    }
                },
                legend: {
                    data: %s
                },
                grid: {
                    left: '3%%',
                    right: '4%%',
                    bottom: '3%%',
                    containLabel: true
                },
                xAxis: {
                    type: 'category',
                    boundaryGap: false,
                    data: %s
                },
                yAxis: {
                    type: 'value',
                    name: 'Await Time (ms)'
                },
                series: %s
            };
            deviceAwaitChart.setOption(deviceAwaitOption);

            // Device Queue Size Chart
            const deviceQueueChart = echarts.init(document.getElementById('deviceQueueChart'));
            const deviceQueueOption = {
                tooltip: {
                    trigger: 'axis',
                    axisPointer: {
                        type: 'cross'
                    }
                },
                legend: {
                    data: %s
                },
                grid: {
                    left: '3%%',
                    right: '4%%',
                    bottom: '3%%',
                    containLabel: true
                },
                xAxis: {
                    type: 'category',
                    boundaryGap: false,
                    data: %s
                },
                yAxis: {
                    type: 'value',
                    name: 'Queue Size'
                },
                series: %s
            };
            deviceQueueChart.setOption(deviceQueueOption);

            // Device Requests Chart
            const deviceRequestsChart = echarts.init(document.getElementById('deviceRequestsChart'));
            const deviceRequestsOption = {
                tooltip: {
                    trigger: 'axis',
                    axisPointer: {
                        type: 'cross'
                    }
                },
                legend: {
                    data: %s
                },
                grid: {
                    left: '3%%',
                    right: '4%%',
                    bottom: '3%%',
                    containLabel: true
                },
                xAxis: {
                    type: 'category',
                    boundaryGap: false,
                    data: %s
                },
                yAxis: {
                    type: 'value',
                    name: 'Requests/sec'
                },
                series: %s
            };
            deviceRequestsChart.setOption(deviceRequestsOption);

            // Device Request Size Chart
            const deviceRequestSizeChart = echarts.init(document.getElementById('deviceRequestSizeChart'));
            const deviceRequestSizeOption = {
                tooltip: {
                    trigger: 'axis',
                    axisPointer: {
                        type: 'cross'
                    }
                },
                legend: {
                    data: %s
                },
                grid: {
                    left: '3%%',
                    right: '4%%',
                    bottom: '3%%',
                    containLabel: true
                },
                xAxis: {
                    type: 'category',
                    boundaryGap: false,
                    data: %s
                },
                yAxis: {
                    type: 'value',
                    name: 'Request Size (KB)'
                },
                series: %s
            };
            deviceRequestSizeChart.setOption(deviceRequestSizeOption);

            // Handle window resize
            window.addEventListener('resize', function() {
                cpuChart.resize();
                ioThroughputChart.resize();
                deviceAwaitChart.resize();
                deviceQueueChart.resize();
                deviceRequestsChart.resize();
                deviceRequestSizeChart.resize();
            });

        } catch (error) {
            console.error('Error initializing charts:', error);
            document.body.innerHTML += '<div style="color: red; padding: 20px; background: #ffe6e6; border: 1px solid red; margin: 20px;">Error initializing charts: ' + error.message + '</div>';
        }
    </script>
</body>
</html>`,
		len(data.Snapshots),
		countUniqueDevices(data),
		findPeakCPUUsage(data),
		findPeakDeviceQueueSize(data),
		labels,
		cpuData,
		labels,
		ioThroughputData,
		extractDeviceAwaitLegendData(data),
		labels,
		extractDeviceAwaitSeriesData(data),
		extractDeviceQueueLegendData(data),
		labels,
		extractDeviceQueueSeriesData(data),
		extractDeviceRequestsLegendData(data),
		labels,
		extractDeviceRequestsSeriesData(data),
		extractDeviceRequestSizeLegendData(data),
		labels,
		extractDeviceRequestSizeSeriesData(data))

	return html, nil
}

// generateEmptyIOStatHTML generates HTML for empty iostat data
func generateEmptyIOStatHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>IOStat Analysis Report</title>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
        }
        .empty-state {
            text-align: center;
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .empty-state h1 {
            color: #666;
            margin-bottom: 10px;
        }
        .empty-state p {
            color: #999;
        }
    </style>
</head>
<body>
    <div class="empty-state">
        <h1>No IOStat Data Available</h1>
        <p>The iostat file appears to be empty or could not be parsed.</p>
    </div>
</body>
</html>`
}

// extractIOStatTimeLabels extracts time labels for chart x-axis
func extractIOStatTimeLabels(data *IOStatReportData) string {
	var labels []string
	for _, snapshot := range data.Snapshots {
		labels = append(labels, fmt.Sprintf(`"%s"`, snapshot.Timestamp.Format("15:04:05")))
	}
	return fmt.Sprintf("[%s]", strings.Join(labels, ", "))
}

// extractCPUSeriesData extracts CPU utilization data for charts
func extractCPUSeriesData(data *IOStatReportData) string {
	userData := make([]string, len(data.Snapshots))
	systemData := make([]string, len(data.Snapshots))
	iowaitData := make([]string, len(data.Snapshots))
	idleData := make([]string, len(data.Snapshots))

	for i, snapshot := range data.Snapshots {
		if snapshot.CPUStats != nil {
			userData[i] = fmt.Sprintf("%.1f", snapshot.CPUStats.User)
			systemData[i] = fmt.Sprintf("%.1f", snapshot.CPUStats.System)
			iowaitData[i] = fmt.Sprintf("%.1f", snapshot.CPUStats.IOWait)
			idleData[i] = fmt.Sprintf("%.1f", snapshot.CPUStats.Idle)
		} else {
			userData[i] = "0"
			systemData[i] = "0"
			iowaitData[i] = "0"
			idleData[i] = "0"
		}
	}

	series := []string{
		fmt.Sprintf(`{
			name: "User",
			type: "line",
			data: [%s],
			smooth: true
		}`, strings.Join(userData, ", ")),
		fmt.Sprintf(`{
			name: "System",
			type: "line",
			data: [%s],
			smooth: true
		}`, strings.Join(systemData, ", ")),
		fmt.Sprintf(`{
			name: "IOWait",
			type: "line",
			data: [%s],
			smooth: true
		}`, strings.Join(iowaitData, ", ")),
		fmt.Sprintf(`{
			name: "Idle",
			type: "line",
			data: [%s],
			smooth: true
		}`, strings.Join(idleData, ", ")),
	}

	return fmt.Sprintf("[%s]", strings.Join(series, ", "))
}

// extractIOThroughputSeriesData extracts I/O throughput data for charts
func extractIOThroughputSeriesData(data *IOStatReportData) string {
	// Aggregate read and write throughput across all devices
	readData := make([]string, len(data.Snapshots))
	writeData := make([]string, len(data.Snapshots))

	for i, snapshot := range data.Snapshots {
		totalRead := 0.0
		totalWrite := 0.0

		for _, device := range snapshot.Devices {
			totalRead += device.ReadKBPerS
			totalWrite += device.WriteKBPerS
		}

		readData[i] = fmt.Sprintf("%.1f", totalRead)
		writeData[i] = fmt.Sprintf("%.1f", totalWrite)
	}

	series := []string{
		fmt.Sprintf(`{
			name: "Read KB/s",
			type: "line",
			data: [%s],
			smooth: true
		}`, strings.Join(readData, ", ")),
		fmt.Sprintf(`{
			name: "Write KB/s",
			type: "line",
			data: [%s],
			smooth: true
		}`, strings.Join(writeData, ", ")),
	}

	return fmt.Sprintf("[%s]", strings.Join(series, ", "))
}

// countUniqueDevices counts the number of unique devices across all snapshots
func countUniqueDevices(data *IOStatReportData) int {
	deviceSet := make(map[string]bool)
	for _, snapshot := range data.Snapshots {
		for _, device := range snapshot.Devices {
			deviceSet[device.Device] = true
		}
	}
	return len(deviceSet)
}

// findPeakCPUUsage finds the peak CPU usage (100 - idle) across all snapshots
func findPeakCPUUsage(data *IOStatReportData) float64 {
	peak := 0.0
	for _, snapshot := range data.Snapshots {
		if snapshot.CPUStats != nil {
			usage := 100.0 - snapshot.CPUStats.Idle
			if usage > peak {
				peak = usage
			}
		}
	}
	return peak
}

// findPeakDeviceQueueSize finds the peak device average queue size across all snapshots
func findPeakDeviceQueueSize(data *IOStatReportData) float64 {
	peak := 0.0
	for _, snapshot := range data.Snapshots {
		for _, device := range snapshot.Devices {
			if device.AvgQueueSize > peak {
				peak = device.AvgQueueSize
			}
		}
	}
	return peak
}

// extractDeviceAwaitLegendData extracts legend data for device await charts
func extractDeviceAwaitLegendData(data *IOStatReportData) string {
	deviceSet := make(map[string]bool)
	for _, snapshot := range data.Snapshots {
		for _, device := range snapshot.Devices {
			deviceSet[device.Device] = true
		}
	}

	var legends []string
	for device := range deviceSet {
		legends = append(legends, fmt.Sprintf(`"%s Read Await"`, device))
		legends = append(legends, fmt.Sprintf(`"%s Write Await"`, device))
	}

	return fmt.Sprintf("[%s]", strings.Join(legends, ", "))
}

// extractDeviceAwaitSeriesData extracts device await time data for charts
func extractDeviceAwaitSeriesData(data *IOStatReportData) string {
	deviceSet := make(map[string]bool)
	for _, snapshot := range data.Snapshots {
		for _, device := range snapshot.Devices {
			deviceSet[device.Device] = true
		}
	}

	var series []string
	for device := range deviceSet {
		readAwaitData := make([]string, len(data.Snapshots))
		writeAwaitData := make([]string, len(data.Snapshots))

		for i, snapshot := range data.Snapshots {
			readAwait := 0.0
			writeAwait := 0.0

			for _, d := range snapshot.Devices {
				if d.Device == device {
					readAwait = d.ReadAwait
					writeAwait = d.WriteAwait
					break
				}
			}

			readAwaitData[i] = fmt.Sprintf("%.2f", readAwait)
			writeAwaitData[i] = fmt.Sprintf("%.2f", writeAwait)
		}

		series = append(series, fmt.Sprintf(`{
			name: "%s Read Await",
			type: "line",
			data: [%s],
			smooth: true
		}`, device, strings.Join(readAwaitData, ", ")))

		series = append(series, fmt.Sprintf(`{
			name: "%s Write Await",
			type: "line",
			data: [%s],
			smooth: true
		}`, device, strings.Join(writeAwaitData, ", ")))
	}

	return fmt.Sprintf("[%s]", strings.Join(series, ", "))
}

// extractDeviceQueueLegendData extracts legend data for device queue charts
func extractDeviceQueueLegendData(data *IOStatReportData) string {
	deviceSet := make(map[string]bool)
	for _, snapshot := range data.Snapshots {
		for _, device := range snapshot.Devices {
			deviceSet[device.Device] = true
		}
	}

	var legends []string
	for device := range deviceSet {
		legends = append(legends, fmt.Sprintf(`"%s Queue Size"`, device))
	}

	return fmt.Sprintf("[%s]", strings.Join(legends, ", "))
}

// extractDeviceQueueSeriesData extracts device queue size data for charts
func extractDeviceQueueSeriesData(data *IOStatReportData) string {
	deviceSet := make(map[string]bool)
	for _, snapshot := range data.Snapshots {
		for _, device := range snapshot.Devices {
			deviceSet[device.Device] = true
		}
	}

	var series []string
	for device := range deviceSet {
		queueData := make([]string, len(data.Snapshots))

		for i, snapshot := range data.Snapshots {
			queueSize := 0.0

			for _, d := range snapshot.Devices {
				if d.Device == device {
					queueSize = d.AvgQueueSize
					break
				}
			}

			queueData[i] = fmt.Sprintf("%.2f", queueSize)
		}

		series = append(series, fmt.Sprintf(`{
			name: "%s Queue Size",
			type: "line",
			data: [%s],
			smooth: true,
			areaStyle: {}
		}`, device, strings.Join(queueData, ", ")))
	}

	return fmt.Sprintf("[%s]", strings.Join(series, ", "))
}

// extractDeviceRequestsLegendData extracts legend data for device requests charts
func extractDeviceRequestsLegendData(data *IOStatReportData) string {
	deviceSet := make(map[string]bool)
	for _, snapshot := range data.Snapshots {
		for _, device := range snapshot.Devices {
			deviceSet[device.Device] = true
		}
	}

	var legends []string
	for device := range deviceSet {
		legends = append(legends, fmt.Sprintf(`"%s Reads/sec"`, device))
		legends = append(legends, fmt.Sprintf(`"%s Writes/sec"`, device))
	}

	return fmt.Sprintf("[%s]", strings.Join(legends, ", "))
}

// extractDeviceRequestsSeriesData extracts device requests per second data for charts
func extractDeviceRequestsSeriesData(data *IOStatReportData) string {
	deviceSet := make(map[string]bool)
	for _, snapshot := range data.Snapshots {
		for _, device := range snapshot.Devices {
			deviceSet[device.Device] = true
		}
	}

	var series []string
	for device := range deviceSet {
		readsData := make([]string, len(data.Snapshots))
		writesData := make([]string, len(data.Snapshots))

		for i, snapshot := range data.Snapshots {
			reads := 0.0
			writes := 0.0

			for _, d := range snapshot.Devices {
				if d.Device == device {
					reads = d.ReadsPerS
					writes = d.WritesPerS
					break
				}
			}

			readsData[i] = fmt.Sprintf("%.2f", reads)
			writesData[i] = fmt.Sprintf("%.2f", writes)
		}

		series = append(series, fmt.Sprintf(`{
			name: "%s Reads/sec",
			type: "line",
			data: [%s],
			smooth: true
		}`, device, strings.Join(readsData, ", ")))

		series = append(series, fmt.Sprintf(`{
			name: "%s Writes/sec",
			type: "line",
			data: [%s],
			smooth: true
		}`, device, strings.Join(writesData, ", ")))
	}

	return fmt.Sprintf("[%s]", strings.Join(series, ", "))
}

// extractDeviceRequestSizeLegendData extracts legend data for device request size charts
func extractDeviceRequestSizeLegendData(data *IOStatReportData) string {
	deviceSet := make(map[string]bool)
	for _, snapshot := range data.Snapshots {
		for _, device := range snapshot.Devices {
			deviceSet[device.Device] = true
		}
	}

	var legends []string
	for device := range deviceSet {
		legends = append(legends, fmt.Sprintf(`"%s Read Size"`, device))
		legends = append(legends, fmt.Sprintf(`"%s Write Size"`, device))
	}

	return fmt.Sprintf("[%s]", strings.Join(legends, ", "))
}

// extractDeviceRequestSizeSeriesData extracts device request size data for charts
func extractDeviceRequestSizeSeriesData(data *IOStatReportData) string {
	deviceSet := make(map[string]bool)
	for _, snapshot := range data.Snapshots {
		for _, device := range snapshot.Devices {
			deviceSet[device.Device] = true
		}
	}

	var series []string
	for device := range deviceSet {
		readSizeData := make([]string, len(data.Snapshots))
		writeSizeData := make([]string, len(data.Snapshots))

		for i, snapshot := range data.Snapshots {
			readSize := 0.0
			writeSize := 0.0

			for _, d := range snapshot.Devices {
				if d.Device == device {
					readSize = d.ReadReqSize
					writeSize = d.WriteReqSize
					break
				}
			}

			readSizeData[i] = fmt.Sprintf("%.2f", readSize)
			writeSizeData[i] = fmt.Sprintf("%.2f", writeSize)
		}

		series = append(series, fmt.Sprintf(`{
			name: "%s Read Size",
			type: "line",
			data: [%s],
			smooth: true
		}`, device, strings.Join(readSizeData, ", ")))

		series = append(series, fmt.Sprintf(`{
			name: "%s Write Size",
			type: "line",
			data: [%s],
			smooth: true
		}`, device, strings.Join(writeSizeData, ", ")))
	}

	return fmt.Sprintf("[%s]", strings.Join(series, ", "))
}
