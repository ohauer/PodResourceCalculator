# Go parameters
GOCMD?=/usr/local/go/bin/go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=PodResourceCalculator
BINARY_UNIX=$(BINARY_NAME)-linux-amd64
BINARY_WINDOWS=$(BINARY_NAME)-windows-amd64.exe
BINARY_DARWIN=$(BINARY_NAME)-darwin-amd64
BINARY_FREEBSD=$(BINARY_NAME)-freebsd-amd64

# Build flags
LDFLAGS=-ldflags "-s -w"
BUILD_FLAGS=-trimpath $(LDFLAGS)

.PHONY: all build clean test deps help run

all: test build

build: ## Build binary for current platform
	$(GOBUILD) $(BUILD_FLAGS) -o ../$(BINARY_NAME) .

build-all: ## Build binaries for all platforms
	# Linux
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o ../$(BINARY_UNIX) .
	# Windows
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o ../$(BINARY_WINDOWS) .
	# macOS
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o ../$(BINARY_DARWIN) .
	# FreeBSD
	GOOS=freebsd GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o ../$(BINARY_FREEBSD) .

test: ## Run tests
	$(GOTEST) -v ./...

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f ../$(BINARY_NAME)
	rm -f ../$(BINARY_UNIX)
	rm -f ../$(BINARY_WINDOWS)
	rm -f ../$(BINARY_DARWIN)
	rm -f ../$(BINARY_FREEBSD)

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

run: build ## Build and run with default settings
	../$(BINARY_NAME) -verbose

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
