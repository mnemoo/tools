# Mnemoo Tools

A toolkit for analyzing game lookup tables (LUTs) with a Go backend and SvelteKit frontend.

Powered by Stake Engine. Respect whole Stake Engine team ❤️

## Quick Start (Recommended)

**Download the Launcher** from [Releases](https://github.com/mnemoo/tools/releases) for your platform:

- `mtools-launcher-darwin-arm64` - macOS Apple Silicon
- `mtools-launcher-linux-amd64` - Linux x64
- `mtools-launcher-linux-arm64` - Linux ARM64
- `mtools-launcher-windows-amd64.exe` - Windows x64
- `mtools-launcher-windows-arm64.exe` - Windows ARM64

The Launcher will:
1. Check and show missing dependencies
2. Let you select the `index.json` file
3. Start/stop backend and frontend with one click
4. Open the app in your browser

## Prerequisites

Before running the Launcher, install these dependencies:

### Required

| Dependency | Version | Download |
|------------|---------|----------|
| **Go** | 1.24+ | [go.dev/dl](https://go.dev/dl/) |
| **Node.js** | 22+ | [nodejs.org](https://nodejs.org/) |
| **pnpm** | latest | `npm install -g pnpm` |

### Installation by OS

<details>
<summary><b>macOS</b></summary>

```bash
# Install Homebrew if not installed
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install dependencies
brew install go node
npm install -g pnpm
```
</details>

<details>
<summary><b>Windows</b></summary>

1. Download and install [Go](https://go.dev/dl/)
2. Download and install [Node.js](https://nodejs.org/)
3. Open terminal and run: `npm install -g pnpm`
</details>

<details>
<summary><b>Linux (Ubuntu/Debian)</b></summary>

```bash
# Install Go
sudo apt update && sudo apt install golang-go

# Install Node.js (via NodeSource)
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

# Install pnpm
npm install -g pnpm
```
</details>

---

## Manual Setup (Advanced)

> **Note:** Running components separately is not recommended. Use the Launcher instead.

<details>
<summary>Show manual instructions</summary>

### Start Backend

```bash
cd backend
go run ./cmd -index /path/to/your/index.json
```

Backend runs on:
- HTTP: http://localhost:7754
- HTTPS: https://localhost:7755

### Start Frontend

```bash
cd frontend
pnpm install
pnpm dev --port 7750
```

Frontend runs on http://localhost:7750

</details>

## Project Structure

```
mtools/
├── backend/          # Go API server
├── frontend/         # SvelteKit web application
├── launcher/         # Desktop launcher (Fyne GUI)
└── stakergs/         # Shared Go library
```

## Documentation

- [Backend README](./backend/README.md)
- [Frontend README](./frontend/README.md)
