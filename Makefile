# =============================================================================
# MinerGate Dashboard v1.0.3 - Makefile
# =============================================================================
# أوامر البناء والتشغيل
# =============================================================================

# المتغيرات
APP_NAME := minergate
VERSION := 1.0.3
BUILD_DIR := ./build
BIN_DIR := $(BUILD_DIR)/bin
CMD_DIR := ./cmd/minergate

# Go
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

# Fyne
FYNE := fyne

# =============================================================================
# الأهداف الافتراضية
# =============================================================================

.PHONY: all build run clean install deps

all: deps build

# =============================================================================
# التبعيات
# =============================================================================

deps:
	@echo "📦 Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "✓ Dependencies installed"

# =============================================================================
# البناء
# =============================================================================

build:
	@echo "🔨 Building $(APP_NAME) v$(VERSION)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo "✓ Build complete: $(BIN_DIR)/$(APP_NAME)"

build-linux:
	@echo "🔨 Building for Linux..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-linux-amd64 $(CMD_DIR)
	@echo "✓ Linux build complete"

build-windows:
	@echo "🔨 Building for Windows..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-windows-amd64.exe $(CMD_DIR)
	@echo "✓ Windows build complete"

build-darwin:
	@echo "🔨 Building for macOS..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-darwin-amd64 $(CMD_DIR)
	@echo "✓ macOS build complete"

build-all: build-linux build-windows build-darwin
	@echo "✓ All builds complete"

# =============================================================================
# التشغيل
# =============================================================================

run:
	@echo "🚀 Running $(APP_NAME)..."
	$(GO) run $(CMD_DIR)/main.go

# =============================================================================
# الاختبار
# =============================================================================

test:
	@echo "🧪 Running tests..."
	$(GO) test -v ./...

test-coverage:
	@echo "📊 Running tests with coverage..."
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# =============================================================================
# التثبيت
# =============================================================================

install:
	@echo "📥 Installing $(APP_NAME)..."
	$(GO) install $(LDFLAGS) $(CMD_DIR)
	@echo "✓ Installed"

# =============================================================================
# التنظيف
# =============================================================================

clean:
	@echo "🧹 Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "✓ Cleaned"

# =============================================================================
# الحزم
# =============================================================================

package:
	@echo "📦 Creating package..."
	@mkdir -p $(BUILD_DIR)/$(APP_NAME)-$(VERSION)
	@cp -r $(BIN_DIR)/$(APP_NAME) $(BUILD_DIR)/$(APP_NAME)-$(VERSION)/
	@cp config.json $(BUILD_DIR)/$(APP_NAME)-$(VERSION)/
	@cp README.md $(BUILD_DIR)/$(APP_NAME)-$(VERSION)/ 2>/dev/null || true
	@cd $(BUILD_DIR) && zip -r $(APP_NAME)-$(VERSION)-linux-amd64.zip $(APP_NAME)-$(VERSION)
	@echo "✓ Package created: $(BUILD_DIR)/$(APP_NAME)-$(VERSION)-linux-amd64.zip"

# =============================================================================
# التطوير
# =============================================================================

lint:
	@echo "🔍 Running linter..."
	golangci-lint run ./...

fmt:
	@echo "📝 Formatting code..."
	$(GO) fmt ./...
	@echo "✓ Formatted"

vet:
	@echo "🔍 Running go vet..."
	$(GO) vet ./...

# =============================================================================
# المساعدة
# =============================================================================

help:
	@echo "MinerGate Dashboard v$(VERSION) - Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all          Install dependencies and build"
	@echo "  deps         Install dependencies"
	@echo "  build        Build the application"
	@echo "  build-all    Build for all platforms"
	@echo "  run          Run the application"
	@echo "  test         Run tests"
	@echo "  test-coverage Run tests with coverage"
	@echo "  install      Install the application"
	@echo "  clean        Clean build artifacts"
	@echo "  package      Create distribution package"
	@echo "  lint         Run linter"
	@echo "  fmt          Format code"
	@echo "  vet          Run go vet"
	@echo "  help         Show this help"
