<!--
Copyright 2025 Ryan SVIHLA Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# DDD Testing Guide

This document describes the comprehensive testing strategy for the DDD (Data Diagnostics Dashboard) project.

## Overview

The DDD project uses a multi-layered testing approach:

1. **Unit Tests** - Fast, isolated tests for individual functions
2. **Integration Tests** - Tests that verify component interactions with real dependencies
3. **End-to-End Tests** - Full workflow tests using Playwright

## Test Structure

```
├── internal/
│   ├── config/
│   ├── database/
│   │   └── database_test.go      # Database integration tests
│   ├── detector/
│   │   └── detector_test.go      # File detection tests
│   ├── handlers/
│   │   └── handlers_test.go      # HTTP handler integration tests (includes httptest-based e2e tests)
│   ├── reporters/
│   │   └── reporters_test.go     # Report generation tests
│   ├── workers/
│   │   └── workers_test.go       # Background worker tests
│   └── testutil/
│       └── testutil.go           # Shared test utilities
├── Makefile                     # Test automation
└── TESTING.md                   # This file
```

## Dependencies

### Go Testing Dependencies
- `github.com/stretchr/testify` - Assertions and test utilities
- `net/http/httptest` - HTTP testing utilities (built-in)

### External Tools (Optional)
- `golangci-lint` - Code linting
- `gosec` - Security scanning
- `gofmt` - Code formatting

## Running Tests

### Quick Start

```bash
# Install dependencies
make install-test-deps

# Run all tests
make test-all

# Run specific test types
make test-unit        # Fast unit tests
make test-integration # Integration tests with real dependencies (includes httptest-based e2e tests)
```

### Test Commands

| Command | Description |
|---------|-------------|
| `make test` | Run quick unit tests |
| `make test-unit` | Run unit tests only |
| `make test-integration` | Run integration tests (includes httptest-based e2e tests) |
| `make test-all` | Run all test types |
| `make test-coverage` | Run tests with coverage report |
| `make test-verbose` | Run tests with verbose output |
| `make test-watch` | Run tests in watch mode (requires `entr`) |

### Coverage Reporting

```bash
# Generate coverage report
make test-coverage

# View coverage in browser
open coverage/coverage.html
```

## Test Categories

### 1. Integration Tests (Preferred)

Following the testing pyramid principle, we prefer integration tests over unit tests for better confidence and maintainability.

#### Database Tests (`internal/database/database_test.go`)
- Tests all CRUD operations with real SQLite database
- Verifies foreign key constraints and data integrity
- Tests pagination, filtering, and complex queries

#### File Detection Tests (`internal/detector/detector_test.go`)
- Tests file type detection with real file samples
- Verifies archive handling (ZIP, TAR.GZ)
- Tests edge cases and malformed files

#### Report Generation Tests (`internal/reporters/reporters_test.go`)
- Tests report generation with actual files
- Verifies JSON output structure and content
- Tests error handling and edge cases

#### Worker Tests (`internal/workers/workers_test.go`)
- Tests background report processing
- Tests file cleanup with real filesystem operations
- Verifies worker error handling and recovery

#### Handler Tests (`internal/handlers/handlers_test.go`)
- Tests HTTP endpoints with real requests
- Tests file upload with multipart forms
- Verifies JSON API responses and error handling
- **Integration Tests**: Comprehensive httptest-based tests that replace browser-based e2e testing:
  - Complete file upload and processing workflows
  - Report generation and viewing via API
  - Error handling and edge cases
  - Performance testing with concurrent requests
  - Duplicate file handling

### 2. HTTP Integration Testing Approach

Uses Go's built-in `net/http/httptest` package for comprehensive testing:

The integration tests use httptest to create HTTP requests and responses, testing the complete application stack without requiring a browser. This approach provides:

1. **Fast Execution**: No browser startup overhead
2. **Reliable Testing**: No browser-specific quirks or timing issues
3. **Complete Coverage**: Tests all API endpoints and workflows
4. **Easy Debugging**: Direct access to request/response data
5. **CI/CD Friendly**: No display or browser dependencies

#### Integration Test Categories

- **File Upload Workflow**: Complete multipart file upload and processing
- **Report Generation**: API-based report creation and retrieval
- **API Endpoints**: Comprehensive testing of all REST endpoints
- **Error Handling**: HTTP error codes and error response validation
- **Performance**: Response time measurement and concurrent request testing
- **Duplicate Handling**: File deduplication and conflict resolution

### 3. Test Utilities (`internal/testutil/testutil.go`)

Shared utilities for all tests:
- Database setup with temporary SQLite files
- Test file creation and management
- Sample file fixtures (ttop, iostat, JSON)
- HTTP test helpers and httptest utilities
- Assertion helpers

## Writing Tests

### Integration Test Example

```go
func TestDatabase_FileOperations(t *testing.T) {
    db := testutil.TestDB(t)  // Creates temporary test database
    
    t.Run("InsertFile", func(t *testing.T) {
        file := &File{
            Hash:         "test-hash",
            OriginalName: "test.txt",
            FileType:     "ttop",
            FileSize:     1024,
            UploadTime:   time.Now(),
            FilePath:     "/uploads/test-hash",
        }
        
        err := db.InsertFile(file)
        require.NoError(t, err)
        assert.NotZero(t, file.ID)
    })
}
```

### Handler Test Example

```go
func TestHandlers_HandleUpload(t *testing.T) {
    handler, db := setupTestHandler(t)
    
    // Create multipart form data
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    part, err := writer.CreateFormFile("file", "test.txt")
    require.NoError(t, err)
    
    _, err = part.Write(testutil.SampleFiles["ttop"].Content)
    require.NoError(t, err)
    err = writer.Close()
    require.NoError(t, err)
    
    // Test the handler
    req := httptest.NewRequest("POST", "/api/upload", body)
    req.Header.Set("Content-Type", writer.FormDataContentType())
    w := httptest.NewRecorder()
    
    handler.HandleUpload(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
}
```

## Test Data

### Sample Files

The test suite includes sample files for different types:

- **TTop files**: Process monitoring output
- **IOStat files**: I/O statistics
- **Queries JSON**: Database query logs
- **Archives**: ZIP and TAR.GZ files containing multiple file types

### Test Database

Each test gets a fresh SQLite database in a temporary directory, ensuring test isolation.

## Continuous Integration

### CI-Friendly Commands

```bash
# Run tests suitable for CI (no GUI required)
make test-ci

# Generate coverage for CI
make test-coverage
```

### GitHub Actions Example

```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v4
      with:
        go-version: '1.24.5'
    - run: make install-test-deps
    - run: make test-ci
    - run: make test-coverage
```

## Performance Testing

```bash
# Run benchmark tests
make test-bench

# Run performance profiling
make test-perf
```

## Debugging Tests

### Verbose Output
```bash
make test-verbose
```

### Run Specific Tests
```bash
# Test specific package
make test-package PKG=./internal/database

# Test specific pattern
make test-file PATTERN=TestDatabase_FileOperations
```

### Debug Integration Tests
```bash
# Run integration tests with verbose output
go test -v ./internal/handlers -run TestIntegration

# Debug specific integration test
go test -v ./internal/handlers -run TestIntegration_FileUploadWorkflow
```

## Best Practices

1. **Prefer Integration Tests**: Test real interactions over mocked dependencies
2. **Use Test Utilities**: Leverage `testutil` package for common setup
3. **Clean Test Data**: Each test should clean up after itself
4. **Descriptive Names**: Use clear, descriptive test names
5. **Test Error Cases**: Include negative test cases and error handling
6. **Parallel Safe**: Ensure tests can run in parallel safely

## Troubleshooting

### Common Issues

1. **Test Dependencies**: Run `make install-test-deps` to install required tools
2. **Database Locks**: Integration tests use temporary SQLite databases
3. **File Permissions**: Ensure test directories are writable
4. **Race Conditions**: Use `-race` flag to detect race conditions

### Getting Help

- Check test output for specific error messages
- Use `make test-verbose` for detailed output
- Review test logs in temporary directories
- Check that all dependencies are installed

## Future Improvements

- Add mutation testing
- Implement property-based testing
- Add load testing scenarios
- Enhance cross-platform testing
- Add API contract testing
- Implement chaos engineering tests
