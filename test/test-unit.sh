#!/bin/bash
set -e

echo "=== Legion Router Unit Tests ==="
echo ""

# Build the docker image first
echo "Step 1: Building docker image..."
echo "-----------------------------------"
docker build -t legion-router:test .
echo ""

# Run tests inside the docker container (using builder stage with Go)
echo "Step 2: Running unit tests in Linux docker container..."
echo "------------------------------------------------"
docker build --target builder -t legion-router:test-builder .
docker run --rm \
  -v "$(pwd):/workspace" \
  -w /workspace \
  legion-router:test-builder \
  go test -v ./...

echo ""
echo "=== All tests completed ==="
