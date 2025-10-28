.PHONY: test clean docker-build docker-run

# Run tests (must be run in Linux docker container)
test:
	./test/test-unit.sh

# Clean build artifacts
clean:
	rm -f legion-router
	go clean

# Build Docker image
docker-build:
	docker build -t legion-router:latest .

# Run in Docker with example config
docker-run: docker-build
	docker run --rm \
		--cap-add=NET_ADMIN \
		--cap-add=NET_RAW \
		-v $(PWD)/examples/config.yaml:/etc/legion-router/config.yaml \
		legion-router:latest

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Download dependencies
deps:
	go mod download
	go mod tidy

# Show help
help:
	@echo "Available targets:"
	@echo "  test         - Run tests in Linux docker container"
	@echo "  clean        - Remove build artifacts"
	@echo "  docker-build - Build docker image"
	@echo "  docker-run   - Run in docker with example config"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  deps         - Download and tidy dependencies"
