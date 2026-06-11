# DiskScope

A portable web-based disk usage visualizer with interactive treemap drill-down.

## Features

- **Treemap visualization** — D3.js powered, click to drill down into directories
- **Breadcrumb navigation** — jump back to any parent level
- **One-shot scan with refresh** — no live watching, scan when you want
- **Exclude common dirs** — toggle off node_modules, .git, __pycache__, etc.
- **Full disk scan** — OS-level elevation (sudo on macOS/Linux, UAC on Windows)
- **Read-only** — no delete/edit, safe to use
- **Single binary** — Go backend with embedded static frontend

## Quick Start

```bash
# Build
make build

# Run (scans home directory by default)
./diskscope

# Or specify a port
./diskscope -port 8080
```

Then open http://localhost:8765 in your browser.

## Building for Other Platforms

```bash
# Windows
make build-windows

# Linux
make build-linux

# macOS ARM (Apple Silicon)
make build-macos-arm
```

## Project Structure

```
diskscope/
├── cmd/
│   └── diskscope.go          # Main entry point, HTTP server
├── internal/
│   └── scanner/
│       └── scanner.go        # Concurrent filesystem walker
├── web/
│   └── static/
│       └── index.html        # Frontend (D3.js treemap)
├── Makefile
├── go.mod
└── README.md
```

## API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/scan` | POST | Scan a directory. Body: `{ path, fullDisk, useExcludes, excludeList }` |
| `/api/elevate` | GET | Restart with sudo for full disk access |
| `/` | GET | Serves the static frontend |
