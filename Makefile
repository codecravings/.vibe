# Vibe DSL Interpreter Makefile

BINARY_NAME=.vibe
VERSION=1.0.0
BUILD_DIR=build

.PHONY: all build clean test install cross-compile

all: build

build:
	go build -o $(BINARY_NAME) .

run: build
	./$(BINARY_NAME) example.vibe --dry-run

test:
	go test -v ./...

clean:
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)

install: build
	cp $(BINARY_NAME) /usr/local/bin/

# Cross-compilation for multiple platforms
cross-compile: clean
	mkdir -p $(BUILD_DIR)
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .

# Build with version info
release:
	go build -ldflags "-X main.Version=$(VERSION)" -o $(BINARY_NAME) .

help:
	@echo "Vibe DSL Interpreter - Build Targets"
	@echo ""
	@echo "  make build         - Build for current platform"
	@echo "  make run           - Build and run with example.vibe"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make install       - Install to /usr/local/bin"
	@echo "  make cross-compile - Build for all platforms"
	@echo "  make release       - Build release version"
	@echo "  make help          - Show this help"
