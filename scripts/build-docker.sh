#!/bin/bash

# Build script for Docker container
set -e

echo "Building Docker container for playback-backend..."

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed or not in PATH"
    echo "Please install Docker Desktop for macOS:"
    echo "https://docs.docker.com/desktop/install/mac-install/"
    exit 1
fi

# Build the container
docker build -f deployments/Dockerfile -t playback-backend:latest .

echo "✅ Docker container built successfully!"
echo "Run with: docker run -p 8080:8080 playback-backend:latest"