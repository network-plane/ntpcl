BINARY := ntpcl
BINDIR := builds

# Static binaries only. Go version comes from go.mod / local toolchain — not set here.
# Version is owned by the app (main.go); no -X ldflags injection.
export CGO_ENABLED := 0

LDFLAGS := -s -w

.DEFAULT_GOAL := build

.PHONY: help build all clean linux windows darwin-arm64 test fmt lint tidy run

help: ## Show available targets
	@echo "Usage: make [target]"
	@echo ""
	@echo "All build targets produce static binaries (CGO_ENABLED=0)."
	@echo "Go version is read from go.mod. App version is set in main.go."
	@echo ""
	@grep -E '^[a-zA-Z0-9_-]+:.*?##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

build: ## Static build for the current platform (-> ./ntpcl)
	@echo "Static build: native -> ./$(BINARY)"
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

all: linux windows darwin-arm64 ## Static release builds: linux/amd64, windows/amd64, darwin/arm64
	@echo "Static release builds complete in $(BINDIR)/"

$(BINDIR):
	mkdir -p $(BINDIR)

linux: $(BINDIR) ## Static linux/amd64 (-> builds/ntpcl)
	@echo "Static build: linux/amd64 -> $(BINDIR)/$(BINARY)"
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINDIR)/$(BINARY) .

windows: $(BINDIR) ## Static windows/amd64 (-> builds/ntpcl.exe)
	@echo "Static build: windows/amd64 -> $(BINDIR)/$(BINARY).exe"
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINDIR)/$(BINARY).exe .

darwin-arm64: $(BINDIR) ## Static darwin/arm64 (-> builds/ntpcl_darwin)
	@echo "Static build: darwin/arm64 -> $(BINDIR)/$(BINARY)_darwin"
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINDIR)/$(BINARY)_darwin .

test: ## Run tests with race detector (CGO enabled for -race only)
	CGO_ENABLED=1 go test -race ./...

fmt: ## Format Go source
	go fmt ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

tidy: ## Run go mod tidy
	go mod tidy

run: build ## Build and run ./ntpcl
	./$(BINARY)

clean: ## Remove build artifacts
	rm -rf $(BINDIR) $(BINARY) $(BINARY).exe $(BINARY)_darwin ntpcl-darwin
