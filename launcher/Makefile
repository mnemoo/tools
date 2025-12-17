.PHONY: build run clean install

# Default target
all: build

# Build the launcher
build:
	go build -o mtools-launcher .

# Build optimized (smaller binary)
build-release:
	go build -ldflags="-s -w" -o mtools-launcher .

# Run the launcher
run: build
	./mtools-launcher

# Clean build artifacts
clean:
	rm -f mtools-launcher

# Install to /usr/local/bin
install: build-release
	cp mtools-launcher /usr/local/bin/

# Build for all platforms
build-all:
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o mtools-launcher-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o mtools-launcher-darwin-arm64 .
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o mtools-launcher-linux-amd64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o mtools-launcher-windows-amd64.exe .
