.PHONY: build install clean test deps help

BUILD_DIR=bin
INSTALL_DIR=$(HOME)/.local/bin

# Build dtools (and dt alias)
build:
	@echo "Building dtools..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/dtools ./cmd/dtools
	@ln -sf dtools $(BUILD_DIR)/dt
	@echo "Built: $(BUILD_DIR)/dtools (also available as $(BUILD_DIR)/dt)"

# Install to ~/.local/bin
install: build
	@echo "Installing to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/dtools $(INSTALL_DIR)/dtools
	@ln -sf dtools $(INSTALL_DIR)/dt
	@echo "Done! Installed: dtools, dt"
	@echo "Make sure $(INSTALL_DIR) is in your PATH"

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
	./$(BUILD_DIR)/dtools $(ARGS)

# Show help
help:
	@echo "dtools - Dylan's DevTools Kit"
	@echo ""
	@echo "Build targets:"
	@echo "  build   - Build dtools binary (and dt symlink)"
	@echo "  install - Build and install to ~/.local/bin"
	@echo "  deps    - Download and tidy dependencies"
	@echo "  clean   - Remove build artifacts"
	@echo "  test    - Run tests"
	@echo ""
	@echo "Usage:"
	@echo "  dtools worktree create <branch>  - Create isolated worktree"
	@echo "  dtools review [pr-number]        - Review CodeRabbit comments"
	@echo "  dtools ralph run                 - Execute PRD stories"
	@echo ""
	@echo "Alias: dt = dtools (e.g., 'dt review 123')"
