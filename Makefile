.PHONY: build install clean test build-review install-review build-ralph install-ralph

BINARY_NAME=worktree-dev
REVIEW_BINARY=review
RALPH_BINARY=ralph
BUILD_DIR=bin
INSTALL_DIR=$(HOME)/.local/bin

# Build all binaries
build: build-worktree build-review build-ralph

# Build worktree-dev binary
build-worktree:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/worktree-dev

# Build review binary
build-review:
	@echo "Building $(REVIEW_BINARY)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(REVIEW_BINARY) ./cmd/review

# Build ralph binary
build-ralph:
	@echo "Building $(RALPH_BINARY)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(RALPH_BINARY) ./cmd/ralph

# Install all to ~/.local/bin
install: build
	@echo "Installing to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	cp $(BUILD_DIR)/$(REVIEW_BINARY) $(INSTALL_DIR)/$(REVIEW_BINARY)
	cp $(BUILD_DIR)/$(RALPH_BINARY) $(INSTALL_DIR)/$(RALPH_BINARY)
	@echo "Done! Make sure $(INSTALL_DIR) is in your PATH"

# Install just review binary
install-review: build-review
	@echo "Installing $(REVIEW_BINARY) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(REVIEW_BINARY) $(INSTALL_DIR)/$(REVIEW_BINARY)
	@echo "Done!"

# Install just ralph binary
install-ralph: build-ralph
	@echo "Installing $(RALPH_BINARY) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(RALPH_BINARY) $(INSTALL_DIR)/$(RALPH_BINARY)
	@echo "Done!"

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
