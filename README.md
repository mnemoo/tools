# Mnemoo Tools

A toolkit for analyzing game lookup tables (LUTs) with a Go backend and SvelteKit frontend.

Powered by Stake Engine. Respect whole Stake Engine team ❤️

---

## Launcher Setup (Recommended)

**One-click solution** - download, run, done. No dependencies required.

### 1. Download

Get the launcher for your platform from [Releases](https://github.com/mnemoo/tools/releases):

| Platform | File |
|----------|------|
| macOS Apple Silicon | `mtools-launcher-darwin-arm64.app.zip` |
| Windows x64 | `mtools-launcher-windows-amd64.exe` |
| Linux x64 | `mtools-launcher-linux-amd64` |

### 2. Run

- **macOS**: Unzip and open the `.app`
- **Windows**: Run the `.exe`
- **Linux**: `chmod +x mtools-launcher-linux-amd64 && ./mtools-launcher-linux-amd64`

### 3. Select library

Use the dir picker to select your library and click **Start**.

> The launcher bundles everything (backend + frontend) - no Go, Node.js, or other dependencies needed.

---

## Manual Setup

For developers who want to run services separately or modify the code.

### Prerequisites

| Dependency | Version | Download |
|------------|---------|----------|
| **Go** | 1.23+ | [go.dev/dl](https://go.dev/dl/) |
| **Node.js** | 22+ | [nodejs.org](https://nodejs.org/) |
| **pnpm** | 9+ | `npm install -g pnpm` |

<details>
<summary><b>macOS</b></summary>

```bash
brew install go node
npm install -g pnpm
```
</details>

<details>
<summary><b>Windows</b></summary>

1. Install [Go](https://go.dev/dl/)
2. Install [Node.js](https://nodejs.org/)
3. Run: `npm install -g pnpm`
</details>

<details>
<summary><b>Linux (Ubuntu/Debian)</b></summary>

```bash
sudo apt update && sudo apt install golang-go
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs
npm install -g pnpm
```
</details>

### Run

```bash
# Backend
# Note: the library directory you provide must contain a "publish_files" directory
# with all required published files (index.json, CSV files, .jsonl.zstd files).
cd backend
go run ./cmd -library /path/to/library
# Runs on http://localhost:7754

# Frontend (separate terminal)
cd frontend
pnpm install && pnpm dev --port 7750
# Runs on http://localhost:7750
```

---

## Project Structure

```
mtools/
├── backend/     # Go API server
├── frontend/    # SvelteKit web app
├── launcher/    # Wails desktop app
└── stakergs/    # Shared Go library
```
