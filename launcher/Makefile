.PHONY: dev build build-release clean install generate setup bundle-dev

# Setup all dependencies
setup:
	cd ui && pnpm install
	go mod tidy

# Development mode with hot reload
dev:
	@cp appicon.png build/appicon.png
	~/go/bin/wails dev

# Build production binary (requires bundled assets)
build:
	@cp appicon.png build/appicon.png
	~/go/bin/wails build -tags prod

# Build optimized production binary
build-release:
	@cp appicon.png build/appicon.png
	~/go/bin/wails build -ldflags="-s -w" -tags prod

# Generate TypeScript bindings
generate:
	~/go/bin/wails generate module

# Install UI dependencies
install-ui:
	cd ui && pnpm install

# Build UI only
build-ui:
	cd ui && pnpm run build

# Bundle assets for local production build
bundle-dev:
	@echo "Building and bundling assets for local production build..."
	@mkdir -p bundled/frontend
	@# Build frontend
	cd ../frontend && pnpm install && pnpm run build
	@cp -r ../frontend/build/* bundled/frontend/
	@# Build backend
	cd ../backend && go build -ldflags="-s -w" -o ../launcher/bundled/backend ./cmd
	@chmod +x bundled/backend
	@echo "Assets bundled. Run 'make build-release' to create production binary."

# Clean build artifacts
clean:
	rm -rf build/bin
	rm -rf ui/dist
	rm -rf bundled

# Clean everything including node_modules
clean-all: clean
	rm -rf ui/node_modules

# Run Go mod tidy
tidy:
	go mod tidy

# Build for all platforms (local, requires bundled assets)
build-all: bundle-dev build-release
	@echo "Production binary created at build/bin/"
