# Backend

Go API server for LUT analysis.

## Prerequisites

- Go 1.24+

## Installation

```bash
cd backend
go mod download
```

## Running

```bash
go run ./cmd -index /path/to/index.json
```

### Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-index` | (required) | Path to index.json file |
| `-port` | 7754 | HTTP server port |
| `-https-port` | 7755 | HTTPS server port (0 to disable) |

### Example

```bash
go run ./cmd -index ./testdata/index.json -port 7755
```

## Building

```bash
go build -o lutexplorer ./cmd
```

## TLS Certificates

On first run, a self-signed certificate is generated and cached:
- `lutexplorer.crt`
- `lutexplorer.key`
