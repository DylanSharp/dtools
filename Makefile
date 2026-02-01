.PHONY: build install clean test

BINARY_NAME=worktree-dev
BUILD_DIR=bin
INSTALL_DIR=$(HOME)/.local/bin

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/worktree-dev

# Install to ~/.local/bin
install: build
	@echo "Installing to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Done! Make sure $(INSTALL_DIR) is in your PATH"

# Install dependencies
deps:
	go mod download
	go mod tidy

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)

# Run tests
test:
	go test -v ./...

# Development: build and run
run: build
	./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

# Show help
help:
	@echo "Available targets:"
	@echo "  build   - Build the binary"
	@echo "  install - Build and install to ~/.local/bin"
	@echo "  deps    - Download and tidy dependencies"
	@echo "  clean   - Remove build artifacts"
	@echo "  test    - Run tests"
	@echo "  run     - Build and run (use ARGS='create' to pass arguments)"
