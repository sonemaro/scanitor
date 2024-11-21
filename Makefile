# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary name
BINARY_NAME=scanitor
BINARY_UNIX=$(BINARY_NAME)

# Build directory
BUILD_DIR=bin

# Get version information
VERSION := $(shell ./scripts/version.sh | grep VERSION | cut -d'=' -f2)
BUILD_NUM := $(shell ./scripts/version.sh | grep BUILD_NUM | cut -d'=' -f2)
COMMIT_HASH := $(shell ./scripts/version.sh | grep COMMIT_HASH | cut -d'=' -f2)
BRANCH := $(shell ./scripts/version.sh | grep BRANCH | cut -d'=' -f2)
BUILD_DATE := $(shell ./scripts/version.sh | grep BUILD_DATE | cut -d'=' -f2)

# Build flags
LD_FLAGS := -X github.com/sonemaro/scanitor/internal/version.Version=$(VERSION)
LD_FLAGS += -X github.com/sonemaro/scanitor/internal/version.BuildNumber=$(BUILD_NUM)
LD_FLAGS += -X github.com/sonemaro/scanitor/internal/version.BuildDate=$(BUILD_DATE)
LD_FLAGS += -X github.com/sonemaro/scanitor/internal/version.GitCommit=$(COMMIT_HASH)
LD_FLAGS += -X github.com/sonemaro/scanitor/internal/version.GitBranch=$(BRANCH)

.PHONY: all build clean test help

all: test build

build:
	@mkdir -p $(BUILD_DIR)
	@echo "Building..."
	@$(GOBUILD) -ldflags "$(LD_FLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) cmd/scanitor/main.go

clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR)

test:
	@echo "Testing..."
	@$(GOTEST) -v ./...

run: build
	@./$(BUILD_DIR)/$(BINARY_NAME)

deps:
	@echo "Downloading dependencies..."
	@$(GOMOD) download

tidy:
	@echo "Tidying dependencies..."
	@$(GOMOD) tidy

version:
	@echo "Version:     $(VERSION)"
	@echo "Build:       $(BUILD_NUM)"
	@echo "Commit:      $(COMMIT_HASH)"
	@echo "Branch:      $(BRANCH)"
	@echo "Build Date:  $(BUILD_DATE)"

help:
	@echo "Make targets:"
	@echo "  build    - Build the binary"
	@echo "  clean    - Clean build files"
	@echo "  test     - Run tests"
	@echo "  deps     - Download dependencies"
	@echo "  tidy     - Tidy go.mod"
	@echo "  version  - Show version info"
	@echo "  help     - Show this help"
