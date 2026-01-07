.PHONY: build clean

# Load .env file
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Build flags for embedding credentials
LDFLAGS := -ldflags "-s -w \
	-X 'sendtg/internal/config.apiID=$(API_ID)' \
	-X 'sendtg/internal/config.apiHash=$(API_HASH)'"

# Output binary name
BINARY := sendtg

build:
	@echo "Building $(BINARY)..."
	@go build $(LDFLAGS) -o $(BINARY) cmd/main.go
	@echo "Build complete: $(BINARY)"

clean:
	@rm -f $(BINARY)
	@echo "Cleaned"

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 cmd/main.go

build-darwin:
	@echo "Building for macOS..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 cmd/main.go
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 cmd/main.go

build-windows:
	@echo "Building for Windows..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-windows-amd64.exe cmd/main.go

