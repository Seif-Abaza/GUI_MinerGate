# MinerGate Dashboard – Makefile
# ──────────────────────────────────────────────────────────────────────────────
# Targets:
#   make deps     – install / tidy Go dependencies
#   make build    – build the binary to build/bin/minergate
#   make run      – run from source
#   make test     – run all tests
#   make clean    – remove build artefacts
#   make package  – create a distributable tar.gz
# ──────────────────────────────────────────────────────────────────────────────

BINARY   := minergate
BUILD_DIR := build/bin
CMD_PKG  := ./cmd/minergate

.PHONY: all deps build run test clean package

all: deps build

deps:
	go mod tidy
	go mod download

build:
	mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY) $(CMD_PKG)
	@echo "Built: $(BUILD_DIR)/$(BINARY)"

run:
	go run $(CMD_PKG)

test:
	go test ./...

clean:
	rm -rf build/

package: build
	tar -czf minergate-$(shell date +%Y%m%d).tar.gz \
		$(BUILD_DIR)/$(BINARY) config.json README.md
	@echo "Package created."
