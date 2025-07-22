/**
 * Copyright 2025 Ryan SVIHLA Corporation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// DDD Application JavaScript

class DDDApp {
    constructor() {
        this.currentPage = 1;
        this.pageSize = 5;
        this.searchQuery = '';
        this.currentFileId = null;
        this.totalFiles = 0;
        this.totalPages = 1;
        this.currentFileType = null;
        this.pollingInterval = null;
        this.init();
    }

    init() {
        this.setupEventListeners();
        this.loadFiles();
        this.loadDiskUsage();
        this.loadSettings();
    }

    setupEventListeners() {
        // Upload area events
        const uploadArea = document.getElementById('upload-area');
        const fileInput = document.getElementById('file-input');

        uploadArea.addEventListener('click', () => fileInput.click());
        uploadArea.addEventListener('dragover', this.handleDragOver.bind(this));
        uploadArea.addEventListener('dragleave', this.handleDragLeave.bind(this));
        uploadArea.addEventListener('drop', this.handleDrop.bind(this));

        fileInput.addEventListener('change', this.handleFileSelect.bind(this));

        // Search events
        const searchButton = document.getElementById('search-button');
        const searchInput = document.getElementById('search-input');
        const clearSearchButton = document.getElementById('clear-search-button');

        searchButton.addEventListener('click', this.handleSearch.bind(this));
        clearSearchButton.addEventListener('click', this.clearSearch.bind(this));

        searchInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                this.handleSearch();
            }
        });

        searchInput.addEventListener('input', (e) => {
            this.updateSearchUI();
        });

        // Pagination events
        document.getElementById('prev-page').addEventListener('click', () => {
            if (this.currentPage > 1) {
                this.currentPage--;
                this.loadFiles();
            }
        });

        document.getElementById('next-page').addEventListener('click', () => {
            if (this.currentPage < this.totalPages) {
                this.currentPage++;
                this.loadFiles();
            }
        });

        // Dialog events
        const dialog = document.getElementById('report-dialog');
        const closeButton = dialog.querySelector('.close');
        closeButton.addEventListener('click', () => {
            this.stopPolling();
            this.currentFileId = null;
            this.currentFileType = null;
            dialog.close();
        });

        // Also handle dialog close events (ESC key, clicking outside)
        dialog.addEventListener('close', () => {
            this.stopPolling();
            this.currentFileId = null;
            this.currentFileType = null;
        });

        // Settings controls
        const setupEditableSetting = (id) => {
            const el = document.getElementById(id);
            if (!el) return;

            el.addEventListener('click', () => {
                if (el.contentEditable !== 'true') {
                    el.contentEditable = true;
                    el.focus();
                    const selection = window.getSelection();
                    const range = document.createRange();
                    range.selectNodeContents(el);
                    selection.removeAllRanges();
                    selection.addRange(range);
                }
            });

            el.addEventListener('blur', () => {
                el.contentEditable = false;
                if (el.textContent.trim()) {
                    this.saveSettings();
                } else {
                    this.loadSettings(); // Revert if empty
                }
            });

            el.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') {
                    e.preventDefault();
                    el.blur();
                }
            });

            el.addEventListener('keypress', (e) => {
                // Allow only numbers
                if (!/\d/.test(e.key)) {
                    e.preventDefault();
                }
            });
        };

        setupEditableSetting('max-disk-usage');
        setupEditableSetting('file-retention-days');
    }

    handleDragOver(e) {
        e.preventDefault();
        e.stopPropagation();
        document.getElementById('upload-area').classList.add('dragover');
    }

    handleDragLeave(e) {
        e.preventDefault();
        e.stopPropagation();
        document.getElementById('upload-area').classList.remove('dragover');
    }

    handleDrop(e) {
        e.preventDefault();
        e.stopPropagation();
        document.getElementById('upload-area').classList.remove('dragover');
        
        const files = e.dataTransfer.files;
        if (files.length > 0) {
            this.uploadFile(files[0]);
        }
    }

    handleFileSelect(e) {
        const files = e.target.files;
        if (files.length > 0) {
            this.uploadFile(files[0]);
        }
    }

    async uploadFile(file) {
        const progressBar = document.getElementById('upload-progress');
        const statusDiv = document.getElementById('upload-status');

        // Show progress
        progressBar.style.display = 'block';
        statusDiv.style.display = 'none';

        const formData = new FormData();
        formData.append('file', file);

        try {
            const response = await fetch('/api/upload', {
                method: 'POST',
                body: formData
            });

            const result = await response.json();

            if (result.success) {
                this.showStatus('File uploaded successfully!', 'success');
                this.loadFiles(); // Refresh file list
            } else {
                this.showStatus('Upload failed: ' + (result.message || 'Unknown error'), 'error');
            }
        } catch (error) {
            this.showStatus('Upload failed: ' + error.message, 'error');
        } finally {
            progressBar.style.display = 'none';
        }
    }

    showStatus(message, type) {
        const statusDiv = document.getElementById('upload-status');
        statusDiv.textContent = message;
        statusDiv.className = `upload-status ${type}`;
        statusDiv.style.display = 'block';

        // Hide after 5 seconds
        setTimeout(() => {
            statusDiv.style.display = 'none';
        }, 5000);
    }

    handleSearch() {
        this.searchQuery = document.getElementById('search-input').value;
        this.currentPage = 1;
        this.updateSearchUI();
        this.loadFiles();
    }

    clearSearch() {
        document.getElementById('search-input').value = '';
        this.searchQuery = '';
        this.currentPage = 1;
        this.updateSearchUI();
        this.loadFiles();
    }

    updateSearchUI() {
        const searchInput = document.getElementById('search-input');
        const clearButton = document.getElementById('clear-search-button');
        const hasSearch = searchInput.value.trim().length > 0;

        clearButton.style.display = hasSearch ? 'inline-block' : 'none';
    }

    updateSearchStatus(files, hasSearch) {
        const statusDiv = document.getElementById('search-status');
        const statusText = document.getElementById('search-status-text');

        if (hasSearch) {
            statusDiv.style.display = 'block';
            const searchTerm = this.searchQuery.trim();

            if (files.length === 0) {
                statusDiv.className = 'search-status no-results';
                statusText.textContent = `No files found for "${searchTerm}"`;
            } else {
                statusDiv.className = 'search-status';
                const fileCount = files.length;
                const pageInfo = this.pageSize < fileCount ? ` (showing ${fileCount} per page)` : '';
                statusText.textContent = `Found ${fileCount} file${fileCount !== 1 ? 's' : ''} matching "${searchTerm}"${pageInfo}`;
            }
        } else {
            statusDiv.style.display = 'none';
        }
    }

    async loadFiles() {
        const loadingDiv = document.getElementById('files-loading');
        const emptyDiv = document.getElementById('files-empty');
        const filesList = document.getElementById('files-list');

        loadingDiv.style.display = 'block';
        emptyDiv.style.display = 'none';

        try {
            const params = new URLSearchParams({
                limit: this.pageSize,
                offset: (this.currentPage - 1) * this.pageSize,
                include_deleted: 'true'
            });

            // Add search query if present
            if (this.searchQuery && this.searchQuery.trim()) {
                params.append('search', this.searchQuery.trim());
            }

            const response = await fetch(`/api/files?${params}`);
            const result = await response.json();

            if (result.success) {
                const files = result.files || [];
                const hasSearch = this.searchQuery && this.searchQuery.trim();
                this.totalFiles = result.total || 0;
                this.totalPages = result.total_pages || 1;
                this.updateSearchStatus(files, hasSearch);
                this.renderFiles(files);
                this.updatePagination();
            } else {
                throw new Error(result.message || 'Failed to load files');
            }
        } catch (error) {
            console.error('Error loading files:', error);
            filesList.innerHTML = '';
            emptyDiv.style.display = 'block';

            const hasSearch = this.searchQuery && this.searchQuery.trim();
            if (hasSearch) {
                emptyDiv.textContent = `Error searching for "${this.searchQuery.trim()}". Please try again.`;
            } else {
                emptyDiv.textContent = 'Error loading files. Please try again.';
            }

            // Also hide search status on error
            document.getElementById('search-status').style.display = 'none';
        } finally {
            loadingDiv.style.display = 'none';
        }
    }

    renderFiles(files) {
        const filesList = document.getElementById('files-list');
        const emptyDiv = document.getElementById('files-empty');

        if (!files || files.length === 0) {
            filesList.innerHTML = '';
            emptyDiv.style.display = 'block';

            const hasSearch = this.searchQuery && this.searchQuery.trim();
            if (hasSearch) {
                emptyDiv.textContent = `No files found matching "${this.searchQuery.trim()}". Try a different search term or clear the search to see all files.`;
            } else {
                emptyDiv.textContent = 'No files found. Upload a file to generate reports.';
            }
            return;
        }

        emptyDiv.style.display = 'none';

        filesList.innerHTML = files.map(file => `
            <tr ${file.deleted ? 'class="deleted-file"' : ''}>
                <td class="mdl-data-table__cell--non-numeric">
                    ${this.highlightSearchTerm(this.escapeHtml(file.original_name))}
                    ${file.deleted ? '<span class="deleted-indicator">(File Removed)</span>' : ''}
                </td>
                <td>
                    <span class="file-hash"
                          onclick="app.showHashVerification('${file.hash}', '${file.original_name}')"
                          title="Click to show verification command">
                        ${this.highlightSearchTerm(file.hash.substring(0, 6))}...
                    </span>
                </td>
                <td>
                    <span class="file-type-badge file-type-${file.file_type}">
                        ${file.file_type}
                    </span>
                </td>
                <td class="file-size">${this.formatFileSize(file.file_size)}</td>
                <td>${this.formatDate(file.upload_time)}</td>
                <td>
                    <div class="file-actions">
                        <button class="mdl-button mdl-js-button mdl-button--icon mdl-button--colored"
                                onclick="app.viewReports(${file.id}, '${file.file_type}', ${file.deleted})"
                                title="View Reports">
                            <i class="material-icons">assessment</i>
                        </button>

                        ${!file.deleted ? `
                            <button class="mdl-button mdl-js-button mdl-button--icon"
                                    onclick="app.redetectFileType(${file.id})"
                                    title="Redetect File Type">
                                <i class="material-icons">autorenew</i>
                            </button>
                            <button class="mdl-button mdl-js-button mdl-button--icon"
                                    onclick="app.deleteFile(${file.id})"
                                    title="Delete File">
                                <i class="material-icons">delete</i>
                            </button>
                        ` : ''}
                    </div>
                </td>
            </tr>
        `).join('');

        // Re-initialize MDL components
        componentHandler.upgradeDom();
    }

    updatePagination() {
        const pageNumbers = document.getElementById('page-numbers');
        const prevButton = document.getElementById('prev-page');
        const nextButton = document.getElementById('next-page');

        // Update prev/next button states
        prevButton.disabled = this.currentPage === 1;
        nextButton.disabled = this.currentPage >= this.totalPages;

        // Clear existing page numbers
        pageNumbers.innerHTML = '';

        if (this.totalPages <= 1) {
            // Hide pagination if only one page or no pages
            document.getElementById('pagination').style.display = 'none';
            return;
        }

        // Show pagination
        document.getElementById('pagination').style.display = 'flex';

        // Generate page numbers
        const pages = this.generatePageNumbers();

        pages.forEach(page => {
            if (page === '...') {
                const ellipsis = document.createElement('span');
                ellipsis.className = 'page-ellipsis';
                ellipsis.textContent = '...';
                pageNumbers.appendChild(ellipsis);
            } else {
                const pageButton = document.createElement('button');
                pageButton.className = `page-number ${page === this.currentPage ? 'current' : ''}`;
                pageButton.textContent = page;
                pageButton.addEventListener('click', () => this.goToPage(page));
                pageNumbers.appendChild(pageButton);
            }
        });
    }

    generatePageNumbers() {
        const pages = [];
        const current = this.currentPage;
        const total = this.totalPages;

        if (total <= 5) {
            // Show all pages if 5 or fewer
            for (let i = 1; i <= total; i++) {
                pages.push(i);
            }
        } else {
            // Always show first page
            pages.push(1);

            if (current <= 3) {
                // Near the beginning: 1 2 3 4 ... last
                for (let i = 2; i <= Math.min(4, total - 1); i++) {
                    pages.push(i);
                }
                if (total > 4) {
                    pages.push('...');
                    pages.push(total);
                }
            } else if (current >= total - 2) {
                // Near the end: 1 ... (total-3) (total-2) (total-1) total
                pages.push('...');
                for (let i = Math.max(2, total - 3); i <= total; i++) {
                    pages.push(i);
                }
            } else {
                // In the middle: 1 ... (current-1) current (current+1) ... last
                pages.push('...');
                pages.push(current - 1);
                pages.push(current);
                pages.push(current + 1);
                pages.push('...');
                pages.push(total);
            }
        }

        return pages;
    }

    goToPage(page) {
        if (page !== this.currentPage && page >= 1 && page <= this.totalPages) {
            this.currentPage = page;
            this.loadFiles();
        }
    }

    async viewReports(fileId, fileType, isDeleted = false) {
        try {
            const response = await fetch(`/api/reports/${fileId}`);

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const result = await response.json();

            if (result.success) {
                // Ensure reports is an array, default to empty array if not
                const reports = Array.isArray(result.reports) ? result.reports : [];
                this.showReportsDialog(reports, fileId, fileType, isDeleted);
            } else {
                throw new Error(result.message || 'Failed to load reports');
            }
        } catch (error) {
            console.error('Error loading reports:', error);
            // Show dialog with error state instead of just an alert
            this.showReportsDialog([], fileId, fileType, isDeleted);
            alert('Failed to load reports: ' + error.message);
        }
    }

    showReportsDialog(reports, fileId, fileType, isDeleted = false) {
        const dialog = document.getElementById('report-dialog');

        // Store current file info for polling
        this.currentFileId = fileId;
        this.currentFileType = fileType;
        this.currentFileDeleted = isDeleted;

        // Render the reports content
        this.renderReportsContent(reports, fileId, fileType, isDeleted);

        // Re-initialize MDL components
        componentHandler.upgradeDom();
        dialog.showModal();

        // Start polling for report status updates (only if file is not deleted)
        if (!isDeleted) {
            this.startPolling();
        }
    }



    copyReportLink(reportId) {
        const reportUrl = `${window.location.origin}/report/${reportId}`;

        // Try to use the modern clipboard API
        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard.writeText(reportUrl).then(() => {
                this.showToast('Report link copied to clipboard!');
            }).catch(err => {
                console.error('Failed to copy link:', err);
                this.fallbackCopyTextToClipboard(reportUrl);
            });
        } else {
            // Fallback for older browsers
            this.fallbackCopyTextToClipboard(reportUrl);
        }
    }

    fallbackCopyTextToClipboard(text) {
        const textArea = document.createElement('textarea');
        textArea.value = text;
        textArea.style.position = 'fixed';
        textArea.style.left = '-999999px';
        textArea.style.top = '-999999px';
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();

        try {
            document.execCommand('copy');
            this.showToast('Report link copied to clipboard!');
        } catch (err) {
            console.error('Fallback: Could not copy text:', err);
            this.showToast('Failed to copy link. Please copy manually: ' + text);
        }

        document.body.removeChild(textArea);
    }

    showToast(message, type = 'info') {
        // Create a simple toast notification
        const toast = document.createElement('div');
        toast.textContent = message;

        // Define colors for different types
        const colors = {
            info: '#06b6d4',
            success: '#10b981',
            warning: '#f59e0b',
            error: '#ef4444'
        };

        const borderColor = colors[type] || colors.info;

        toast.style.cssText = `
            position: fixed;
            bottom: 20px;
            left: 50%;
            transform: translateX(-50%);
            background-color: #1a202c;
            color: white;
            padding: 12px 24px;
            border-radius: 8px;
            z-index: 10000;
            font-size: 14px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
            border-left: 4px solid ${borderColor};
            max-width: 400px;
            text-align: center;
        `;

        document.body.appendChild(toast);

        // Remove toast after 4 seconds for file deletion messages, 3 seconds for others
        const duration = message.includes('completely removed') ? 4000 : 3000;
        setTimeout(() => {
            if (document.body.contains(toast)) {
                document.body.removeChild(toast);
            }
        }, duration);
    }

    async deleteReport(reportId) {
        if (!confirm('Are you sure you want to delete this report?')) {
            return;
        }

        try {
            const response = await fetch(`/api/reports/${reportId}`, {
                method: 'DELETE'
            });

            const result = await response.json();

            if (result.success) {
                // Remove the report item from the list
                const reportItem = document.querySelector(`[data-report-id="${reportId}"]`);
                if (reportItem) {
                    reportItem.remove();
                }

                // Show success message
                this.showToast(result.message, 'success');

                // If a file was completely deleted, refresh the file list
                if (result.file_deleted) {
                    console.log(`File ${result.deleted_file_name} was completely removed from database`);
                    // Refresh the file list to show the file is gone
                    await this.loadFiles();
                    // Show additional notification about file removal
                    this.showToast(`File "${result.deleted_file_name}" was completely removed (no reports remaining)`, 'warning');
                }

            } else {
                throw new Error(result.message || 'Failed to delete report');
            }
        } catch (error) {
            console.error('Error deleting report:', error);
            this.showToast('Failed to delete report: ' + error.message, 'error');
        }
    }

    renderReportData(reportDataStr) {
        try {
            const reportData = JSON.parse(reportDataStr);
            
            // If we have a full HTML report, use that
            if (reportData.html_report) {
                return `
                    <div class="html-report-container">
                        ${reportData.html_report}
                    </div>
                `;
            }
            
            // Fall back to basic report display
            return `
                <div class="report-content">
                    <h4>Report Summary</h4>
                    <p>${reportData.summary || 'No summary available'}</p>
                    <h4>Analysis</h4>
                    <p>${reportData.analysis || 'No analysis available'}</p>
                    ${reportData.charts ? this.renderCharts(reportData.charts) : ''}
                </div>
            `;
        } catch (error) {
            return `<pre class="report-raw-data">${this.escapeHtml(reportDataStr)}</pre>`;
        }
    }

    renderCharts(charts) {
        if (!charts || charts.length === 0) {
            return '<div class="chart-container">No charts available</div>';
        }
        
        let chartsHTML = '';
        charts.forEach((chart, index) => {
            chartsHTML += `
                <div class="chart-container">
                    <div class="chart-title">${chart.title}</div>
                    <div id="chart-${index}" style="width: 100%; height: 400px;"></div>
                </div>
            `;
        });
        
        // Add script to initialize ECharts
        chartsHTML += `
            <script>
                // Initialize ECharts for each chart
                ${charts.map((chart, index) => `
                    const chart${index} = echarts.init(document.getElementById('chart-${index}'));
                    chart${index}.setOption(${JSON.stringify(chart.options)});
                `).join('')}
                
                // Make charts responsive
                window.addEventListener('resize', function() {
                    ${charts.map((chart, index) => `chart${index}.resize();`).join('')}
                });
            </script>
<style>
    .editable-setting {
        display: inline-block;
        width: 40px;
        padding: 2px 4px;
        border-radius: 3px;
        border: 1px solid transparent;
        text-align: center;
        cursor: pointer;
        color: white;
        font-size: 12px;
    }
    .editable-setting:hover {
        background: rgba(255, 255, 255, 0.1);
    }
    .editable-setting[contenteditable="true"] {
        cursor: text;
        background: rgba(255, 255, 255, 0.2);
        border: 1px solid rgba(255, 255, 255, 0.3);
    }
    .editable-setting[contenteditable="true"]:focus {
        background: rgba(255, 255, 255, 0.3);
        outline: none;
    }
    /* Disk Usage Toolbar Styles */
    .disk-usage-toolbar {
        display: flex;
        align-items: center;
        margin-right: 20px;
        color: white;
        font-size: 14px;
        gap: 15px;
    }
    
    .settings-controls {
        display: flex;
        align-items: center;
        gap: 5px;
    }
    
    .setting-label {
        font-size: 12px;
        opacity: 0.9;
    }
    
    .setting-unit {
        font-size: 12px;
        opacity: 0.8;
        margin-right: 5px;
    }
    
    .mini-input {
        width: 40px;
        padding: 2px 4px;
        background: rgba(255, 255, 255, 0.2);
        border: 1px solid rgba(255, 255, 255, 0.3);
        border-radius: 3px;
        color: white;
        font-size: 12px;
        text-align: center;
    }
    
    .mini-input:focus {
        background: rgba(255, 255, 255, 0.3);
        outline: none;
    }
    
    .disk-usage-warning {
        color: #ff9800;
        font-weight: bold;
    }
    
    #disk-usage-display {
        display: inline-block;
    }
</style>
        `;
        
        return chartsHTML;
    }

    async createReport(fileId, fileType) {
        try {
            const response = await fetch(`/api/reports/${fileId}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    report_type: fileType
                })
            });

            const result = await response.json();

            if (result.success) {
                alert('Report queued for processing!');
                // Immediately refresh the reports to show the new pending report
                this.refreshReports();
                // Restart polling in case it was stopped
                if (!this.pollingInterval) {
                    this.startPolling();
                }
            } else {
                throw new Error(result.message || 'Failed to create report');
            }
        } catch (error) {
            console.error('Error creating report:', error);
            alert('Failed to create report: ' + error.message);
        }
    }

    startPolling() {
        // Stop any existing polling
        this.stopPolling();

        console.log('Starting polling for file:', this.currentFileId);
        // Start polling every 2 seconds
        this.pollingInterval = setInterval(() => {
            this.refreshReports();
        }, 2000);
    }

    stopPolling() {
        if (this.pollingInterval) {
            console.log('Stopping polling');
            clearInterval(this.pollingInterval);
            this.pollingInterval = null;
        }
    }

    async refreshReports() {
        // Only refresh if dialog is open and we have file info
        const dialog = document.getElementById('report-dialog');
        if (!this.currentFileId || !dialog || !dialog.hasAttribute('open')) {
            this.stopPolling();
            return;
        }

        try {
            const response = await fetch(`/api/reports/${this.currentFileId}`);

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const result = await response.json();

            if (result.success) {
                const reports = Array.isArray(result.reports) ? result.reports : [];
                this.updateReportsInDialog(reports);

                // Stop polling if no reports are pending or running
                const hasActiveReports = reports.some(report =>
                    report.status === 'pending' || report.status === 'running'
                );
                console.log('Active reports found:', hasActiveReports, 'Total reports:', reports.length);

                if (!hasActiveReports && reports.length > 0) {
                    console.log('No active reports, stopping polling');
                    this.stopPolling();
                }
            }
        } catch (error) {
            console.error('Error refreshing reports:', error);
            // Don't show alerts during polling to avoid spam
        }
    }

    updateReportsInDialog(reports) {
        const content = document.getElementById('report-content');
        if (!content) return;

        // Check if we're in the no-reports state
        if (reports.length === 0) {
            // Keep the no-reports view if there are still no reports
            if (content.querySelector('.no-reports-container')) {
                return;
            }
        }

        // Re-render the entire dialog content with updated reports
        this.renderReportsContent(reports, this.currentFileId, this.currentFileType, this.currentFileDeleted);
    }

    renderReportsContent(reports, fileId, fileType, isDeleted = false) {
        const content = document.getElementById('report-content');

        // Check if there are any active reports (pending or running)
        const hasActiveReports = reports && reports.some(report =>
            report.status === 'pending' || report.status === 'running'
        );

        if (!reports || reports.length === 0) {
            if (isDeleted) {
                content.innerHTML = `
                    <div class="no-reports-container">
                        <p>No reports found for this file.</p>
                        <div class="deleted-file-message">
                            <p><strong>File has been removed from disk.</strong></p>
                            <p>To generate new reports, please upload the file again.</p>
                        </div>
                    </div>
                `;
            } else {
                content.innerHTML = `
                    <div class="no-reports-container">
                        <p>No reports found for this file.</p>
                        <button class="mdl-button mdl-js-button mdl-button--raised mdl-button--colored"
                                onclick="app.createReport(${fileId}, '${fileType}')">
                            <i class="material-icons">play_arrow</i>
                            Generate New Report
                        </button>
                    </div>
                `;
            }
        } else {
            content.innerHTML = `
                <div class="reports-container">
                    <div class="reports-list">
                        <div class="reports-header">
                            <h4>Reports ${hasActiveReports ? '<span class="polling-indicator" title="Auto-refreshing every 2 seconds"></span>' : ''}</h4>
                            ${!isDeleted ? `
                                <button class="mdl-button mdl-js-button mdl-button--raised mdl-button--colored"
                                        onclick="app.createReport(${fileId}, '${fileType}')">
                                    <i class="material-icons">play_arrow</i>
                                    Generate New Report
                                </button>
                            ` : `
                                <div class="deleted-file-message">
                                    <p><strong>File removed from disk.</strong> Upload file again to generate new reports.</p>
                                </div>
                            `}
                        </div>
                        ${reports.map(report => `
                            <div class="report-item" data-report-id="${report.id}">
                                <div class="report-info">
                                    <div>
                                        <strong>${report.report_type}</strong>
                                        <span class="status-badge status-${report.status}">${report.status}</span>
                                    </div>
                                    <div>
                                        <small>Created: ${this.formatDate(report.created_time)}</small>
                                    </div>
                                    <div>
                                        <small>Version: ${report.ddd_version}</small>
                                    </div>
                                    <div>
                                        ${report.completed_time ? `<small>Completed: ${this.formatDate(report.completed_time)}</small>` : ''}
                                        ${report.error_message ? `<small style="color: #d32f2f;">Error: ${report.error_message}</small>` : ''}
                                    </div>
                                </div>
                                <div class="report-actions">
                                    ${report.status === 'completed' ? `
                                        <button class="mdl-button mdl-js-button mdl-button--icon"
                                                onclick="app.copyReportLink(${report.id})" title="Copy Report Link">
                                            <i class="material-icons">link</i>
                                        </button>
                                        <a href="/report/${report.id}" target="_blank"
                                           class="mdl-button mdl-js-button mdl-button--icon" title="Open Report in New Tab">
                                            <i class="material-icons">open_in_new</i>
                                        </a>
                                    ` : ''}
                                    <button class="mdl-button mdl-js-button mdl-button--icon"
                                            onclick="app.deleteReport(${report.id})" title="Delete Report">
                                        <i class="material-icons">delete</i>
                                    </button>
                                </div>
                            </div>
                        `).join('')}
                    </div>
                </div>
            `;
        }

        // Re-initialize MDL components for any new buttons
        componentHandler.upgradeDom();
    }

    async redetectFileType(fileId) {
        try {
            const response = await fetch(`/api/files/${fileId}/redetect`, {
                method: 'POST'
            });

            const result = await response.json();

            if (result.success) {
                this.showToast('File type redetection started!');
                // Refresh file list after a short delay to show updated type
                setTimeout(() => this.loadFiles(), 1000);
            } else {
                throw new Error(result.message || 'Failed to redetect file type');
            }
        } catch (error) {
            console.error('Error redetecting file type:', error);
            this.showToast('Failed to redetect file type: ' + error.message);
        }
    }

    async deleteFile(fileId) {
        if (!confirm('Are you sure you want to delete this file?')) {
            return;
        }

        try {
            const response = await fetch(`/api/files/${fileId}`, {
                method: 'DELETE'
            });

            const result = await response.json();

            if (result.success) {
                this.loadFiles(); // Refresh file list
            } else {
                throw new Error(result.message || 'Failed to delete file');
            }
        } catch (error) {
            console.error('Error deleting file:', error);
            alert('Failed to delete file: ' + error.message);
        }
    }

    formatFileSize(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    showHashVerification(hash, filename) {
        const command = `echo "${hash}  ${filename}" | shasum -a 256 -c`;

        const instructions = `
<strong>Verification Command:</strong><br>
<code>${command}</code>
<br><br>
<em>This command will verify that your local file matches the uploaded file's SHA256 hash.</em>
<br><em>Run this command in the directory containing your local copy of the file.</em>
        `;

        // Create a dialog to show the verification command
        const dialog = document.createElement('div');
        dialog.className = 'hash-verification-dialog';
        dialog.innerHTML = `
            <div class="hash-verification-content">
                <div class="hash-verification-header">
                    <h3>File Hash Verification</h3>
                    <button class="close-button" onclick="this.closest('.hash-verification-dialog').remove()">×</button>
                </div>
                <div class="hash-verification-body">
                    <p><strong>File:</strong> ${this.escapeHtml(filename)}</p>
                    <p><strong>Full Hash:</strong> <code>${hash}</code></p>
                    ${instructions}
                    <button class="mdl-button mdl-js-button mdl-button--raised mdl-button--colored copy-command-btn">
                        Copy Command
                    </button>
                </div>
            </div>
        `;

        // Add click handler for copy button
        dialog.querySelector('.copy-command-btn').addEventListener('click', () => {
            this.copyToClipboard(command);
        });

        // Add to body and show
        document.body.appendChild(dialog);

        // Close on background click
        dialog.addEventListener('click', (e) => {
            if (e.target === dialog) {
                dialog.remove();
            }
        });
    }

    copyToClipboard(text) {
        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard.writeText(text).then(() => {
                this.showToast('Command copied to clipboard!');
            }).catch(err => {
                console.error('Could not copy text: ', err);
                this.fallbackCopyTextToClipboard(text);
            });
        } else {
            this.fallbackCopyTextToClipboard(text);
        }
    }

    fallbackCopyTextToClipboard(text) {
        const textArea = document.createElement("textarea");
        textArea.value = text;
        textArea.style.top = "0";
        textArea.style.left = "0";
        textArea.style.position = "fixed";

        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();

        try {
            document.execCommand('copy');
            this.showToast('Command copied to clipboard!');
        } catch (err) {
            console.error('Fallback: Could not copy text:', err);
            this.showToast('Could not copy to clipboard');
        }

        document.body.removeChild(textArea);
    }



    async loadDiskUsage() {
        try {
            const response = await fetch('/api/disk-usage');
            const result = await response.json();
            
            if (result.success) {
                this.updateDiskUsageUI(result.uploads, result.database);
            } else {
                console.error('Failed to load disk usage:', result.message);
            }
        } catch (error) {
            console.error('Error loading disk usage:', error);
        }
    }

    updateDiskUsageUI(uploads, database) {
        const diskUsageDisplay = document.getElementById('disk-usage-display');
        const currentUsageDisplay = document.getElementById('current-disk-usage');
        let displayText = '';
        let currentPercent = 0;

        // Check if they're on the same filesystem
        const sameFilesystem = uploads.path === database.path;

        if (sameFilesystem) {
            // Just show one set of stats if on same filesystem
            const totalUsed = uploads.used_bytes;
            const totalSize = uploads.total_bytes;
            const totalPercent = uploads.used_percent.toFixed(1);
            currentPercent = uploads.used_percent;
            displayText = `Disk: ${this.formatFileSize(totalUsed)} / ${this.formatFileSize(totalSize)} (${totalPercent}%)`;
        } else {
            // Show combined stats if on different filesystems
            const totalUsed = uploads.used_bytes + database.used_bytes;
            const totalSize = uploads.total_bytes + database.total_bytes;
            const totalPercent = (totalUsed / totalSize * 100).toFixed(1);
            currentPercent = totalUsed / totalSize * 100;
            displayText = `Uploads: ${this.formatFileSize(uploads.used_bytes)} / ${this.formatFileSize(uploads.total_bytes)} | `;
            displayText += `DB: ${this.formatFileSize(database.used_bytes)} / ${this.formatFileSize(database.total_bytes)} | `;
            displayText += `Total: ${totalPercent}%`;
        }

        // Update displays
        diskUsageDisplay.textContent = displayText;
        currentUsageDisplay.textContent = currentPercent.toFixed(1);

        // Add warning class if usage is high (>80%)
        const highUsage = sameFilesystem ?
            uploads.used_percent > 80 :
            (uploads.used_percent > 80 || database.used_percent > 80);

        if (highUsage) {
            diskUsageDisplay.classList.add('disk-usage-warning');
            currentUsageDisplay.classList.add('disk-usage-warning');
        } else {
            diskUsageDisplay.classList.remove('disk-usage-warning');
            currentUsageDisplay.classList.remove('disk-usage-warning');
        }
    }

    async loadSettings() {
        try {
            const response = await fetch('/api/disk-usage');
            const result = await response.json();
            
            if (result.success) {
                // Get max disk usage from config and convert from decimal to percentage
                const maxDiskUsage = result.max_disk_usage ? Math.round(result.max_disk_usage * 100) : 50;
                const retentionDays = result.file_retention_days || 14;
                
                // Update input fields with current values
                document.getElementById('max-disk-usage').textContent = maxDiskUsage;
                document.getElementById('file-retention-days').textContent = retentionDays;
            } else {
                console.error('Failed to load settings:', result.message);
                // Set defaults if loading fails
                document.getElementById('max-disk-usage').textContent = 50;
                document.getElementById('file-retention-days').textContent = 14;
            }
        } catch (error) {
            console.error('Error loading settings:', error);
            // Set defaults if loading fails
            document.getElementById('max-disk-usage').value = 50;
            document.getElementById('file-retention-days').value = 14;
        }
    }

    async saveSettings() {
        const maxUsage = document.getElementById('max-disk-usage').textContent.trim();
        const retentionDays = document.getElementById('file-retention-days').textContent.trim();
        
        try {
            const response = await fetch('/api/settings', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    max_disk_usage: maxUsage,
                    file_retention_days: retentionDays
                })
            });
            
            const result = await response.json();
            if (result.success) {
                this.showToast('Settings saved successfully!');
            } else {
                throw new Error(result.message || 'Failed to save settings');
            }
        } catch (error) {
            console.error('Error saving settings:', error);
            this.showToast('Failed to save settings: ' + error.message);
        }
    }

    formatDate(dateStr) {
        const date = new Date(dateStr);
        return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    highlightSearchTerm(text) {
        if (!this.searchQuery || !this.searchQuery.trim()) {
            return text;
        }

        const searchTerm = this.searchQuery.trim();
        const regex = new RegExp(`(${this.escapeRegex(searchTerm)})`, 'gi');
        return text.replace(regex, '<span class="search-highlight">$1</span>');
    }

    escapeRegex(string) {
        return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }
}

// Initialize the application
const app = new DDDApp();
