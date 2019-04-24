# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: gdx gdx-cross all test lint clean cleanall 
.PHONY: gdx-linux gdx-linux-386 gdx-linux-amd64
.PHONY: gdx-linux-arm gdx-linux-arm-7 gdx-linux-arm64
.PHONY: gdx-darwin gdx-darwin-amd64
.PHONY: gdx-windows gdx-windows-386 gdx-windows-amd64

BUILD_BIN = $(shell pwd)/build/bin
GO ?= latest


gdx:
	go run build/ci.go install ./cmd/gdx
	@echo "Done building."
	@echo "Run \"$(BUILD_BIN)/gdx\" to launch gdx."

all:
	go run build/ci.go install

test: all
	go run build/ci.go test

lint: ## Run linters.
	go run build/ci.go lint


clean:
	rm -fr $(BUILD_BIN)/*

cleanall: clean
	go clean -testcache -cache

# Cross Compilation Targets (xgo)
gdx-cross: gdx-linux gdx-darwin gdx-windows
	@echo "Full cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-*

gdx-linux: gdx-linux-386 gdx-linux-amd64 gdx-linux-arm
	@echo "Linux cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-linux-*

gdx-linux-386:
	go run build/ci.go xgo -- --go=$(GO) --targets=linux/386 -v ./cmd/gdx
	@echo "Linux 386 cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-linux-* | grep 386

gdx-linux-amd64:
	go run build/ci.go xgo -- --go=$(GO) --targets=linux/amd64 -v ./cmd/gdx
	@echo "Linux amd64 cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-linux-* | grep amd64

gdx-linux-arm: gdx-linux-arm-7 gdx-linux-arm64
	@echo "Linux ARM cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-linux-* | grep arm

gdx-linux-arm-7:
	go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-7 -v ./cmd/gdx
	@echo "Linux ARMv7 cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-linux-* | grep arm-7

gdx-linux-arm64:
	go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm64 -v ./cmd/gdx
	@echo "Linux ARM64 cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-linux-* | grep arm64

gdx-darwin: gdx-darwin-amd64
	@echo "Darwin cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-darwin-*

gdx-darwin-amd64:
	go run build/ci.go xgo -- --go=$(GO) --targets=darwin/amd64 -v ./cmd/gdx
	@echo "Darwin amd64 cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-darwin-* | grep amd64

gdx-windows: gdx-windows-386 gdx-windows-amd64
	@echo "Windows cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-windows-*

gdx-windows-386:
	go run build/ci.go xgo -- --go=$(GO) --targets=windows/386 -v ./cmd/gdx
	@echo "Windows 386 cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-windows-* | grep 386

gdx-windows-amd64:
	go run build/ci.go xgo -- --go=$(GO) --targets=windows/amd64 -v ./cmd/gdx
	@echo "Windows amd64 cross compilation done:"
	@ls -ld $(BUILD_BIN)/gdx-windows-* | grep amd64
