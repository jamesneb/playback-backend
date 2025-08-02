#!/bin/bash

# Test runner script for playback-backend
# Runs all unit tests with proper coverage and formatting

set -e

echo "🧪 Running playback-backend test suite..."
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

print_status "Go version: $(go version)"

# Clean any previous test artifacts
echo ""
echo "🧹 Cleaning previous test artifacts..."
rm -f coverage.out coverage.html

# Download dependencies
echo ""
echo "📦 Downloading dependencies..."
go mod download
go mod tidy

print_status "Dependencies updated"

# Run go vet for static analysis
echo ""
echo "🔍 Running static analysis (go vet)..."
if go vet ./...; then
    print_status "Static analysis passed"
else
    print_error "Static analysis failed"
    exit 1
fi

# Run tests with coverage
echo ""
echo "🏃 Running unit tests with coverage..."
if go test -v -race -coverprofile=coverage.out -covermode=atomic ./...; then
    print_status "All tests passed"
else
    print_error "Some tests failed"
    exit 1
fi

# Generate coverage report
echo ""
echo "📊 Generating coverage report..."
go tool cover -html=coverage.out -o coverage.html

# Display coverage summary
echo ""
echo "📈 Coverage summary:"
go tool cover -func=coverage.out | tail -1

# Check coverage threshold (optional - set to 70% minimum)
COVERAGE=$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}' | sed 's/%//')
THRESHOLD=70

if (( $(echo "$COVERAGE >= $THRESHOLD" | bc -l) )); then
    print_status "Coverage threshold met: ${COVERAGE}% >= ${THRESHOLD}%"
else
    print_warning "Coverage below threshold: ${COVERAGE}% < ${THRESHOLD}%"
fi

# Run benchmarks (optional)
if [[ "$1" == "--bench" ]]; then
    echo ""
    echo "🏎️  Running benchmarks..."
    go test -bench=. -benchmem ./...
fi

# Test specific packages if provided
if [[ "$1" == "--package" && -n "$2" ]]; then
    echo ""
    echo "🎯 Running tests for specific package: $2"
    go test -v -race "$2"
fi

echo ""
print_status "Test suite completed successfully!"
echo "📋 Coverage report generated: coverage.html"
echo "🔍 View coverage: open coverage.html"