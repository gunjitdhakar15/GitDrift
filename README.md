# GitDrift 🚀

> Blazing-fast CLI and analysis engine written in Go that scans monorepos for architectural drift, zombie files, git hotspots, circular imports, and technical debt.

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-blue.svg)](https://golang.org)
[![CI](https://github.com/yourusername/gitdrift/actions/workflows/ci.yml/badge.svg)](https://github.com/yourusername/gitdrift/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/gitdrift)](https://goreportcard.com/report/github.com/yourusername/gitdrift)

---

## 🌟 Overview

Modern monorepos accumulate silent rot: unreferenced files, high-churn files prone to merge conflicts, circular dependencies, and forgotten TODOs. **GitDrift** combines deep git history parsing with static analysis to surface actionable architectural health insights in milliseconds.

Designed in the style of high-performance developer tooling (like `tokei` or `ripgrep`), GitDrift processes thousands of files concurrently with zero runtime bloat.

---

## ⚙️ Key Architecture & Engineering Features

- **Web Console (`server/`):** Paste any GitHub URL → the server shallow-clones in the background, runs every analyzer, and serves the report — zero local setup. Async job store with TTL reaper, injectable analysis engine for testability, embedded single-file UI via `go:embed`.
- **Concurrent Analysis Engine (`pkg/analyzer/scan.go`):** Worker-pool based file traversal using goroutines (bounded by `runtime.NumCPU`), enabling sub-second scans of 10k+ file monorepos.
- **Circular Import Detection (`cycles.go`):** Full Go import-graph construction and depth-first cycle detection across internal packages, resolving imports via the module path.
- **Git Churn Hotspots (`hotspots.go`):** Parses `git log --name-only` streams to rank files by modification frequency — the classic code-smell indicator for refactoring targets.
- **Zombie / Stale File Detection (`stale.go`):** Cross-references `git ls-files` with per-file commit history to flag abandoned code untouched for configurable thresholds.
- **Tech Debt Scanner (`todos.go`):** Word-boundary regex matching for `TODO` / `FIXME` / `HACK` / `OPTIMIZE` markers with precise file:line reporting — no false positives on identifiers like `TodoItem`.
- **Self-Contained HTML Dashboard (`report.go`):** Single-file, dependency-free HTML report with dark GitHub-style theming, stats cards, language share bars, and full drill-down tables. No CDN, no build step — open it anywhere.
- **Machine-Readable JSON:** Every command and API endpoint supports `--json` for CI/CD quality gates and PR comment automation.
- **ANSI Terminal UI:** Color-coded sections, alignment columns, and status indicators with automatic `NO_COLOR` / dumb-terminal fallback.

---

## 🚀 Installation & Quick Start

### From Source
```bash
git clone https://github.com/yourusername/gitdrift.git
cd gitdrift
go build -o gitdrift ./cmd/gitdrift
```

### Commands

| Command | Description |
|---|---|
| `scan` | Run every analyzer and print the full health report |
| `stale` | Find zombie files untouched by git for N days (`--idle-days`) |
| `todos` | Find TODO / FIXME / HACK / OPTIMIZE markers (`--limit`) |
| `hotspots` | Find high-churn files from git history (`--limit`) |
| `deps` | Detect circular Go package imports and print the package graph |
| `report` | Generate a self-contained HTML dashboard (`--out report.html`) |
| `version` | Print version |

### Examples

```bash
# Full health scan of any repository
./gitdrift scan --dir /path/to/repo

# Generate an HTML dashboard
./gitdrift report --dir . --out health.html

# CI-friendly JSON output
./gitdrift scan --dir . --json > health-report.json

# Find files untouched for 6+ months
./gitdrift stale --idle-days 180
```

---

## 🌐 Web Console — analyze any GitHub repo without cloning

Paste a GitHub URL, get a full report. The server shallow-clones the repo in the background, runs every analyzer, and serves the dashboard.

```bash
go build -o gitdrift-server ./server
./gitdrift-server -addr :8080
```

Then open http://localhost:8080 — enter any public GitHub URL (or pick a sample repo) and watch the live progress.

### API

| Endpoint | Description |
|---|---|
| `POST /api/analyze` | Body: `{"url": "...", "depth": 100}` → `202` + job id |
| `GET /api/status?id=<id>` | Poll job: `queued` → `cloning` → `done` / `failed` |
| `GET /api/report?id=<id>` | Full HTML dashboard (self-contained file) |
| `GET /api/report?id=<id>&json=1` | Machine-readable report JSON |

Flags: `-addr` (default `:8080`), `-max-jobs`, `-ttl` (job retention), `-clone-timeout`. Set `GITDRIFT_GITHUB_TOKEN` for private repos or higher clone rate limits.

```text
┌────────────────────────────────────────────┐
│  🚀 GitDrift — Monorepo Drift Analyzer    │
└────────────────────────────────────────────┘
  repo:   /path/to/repo
  engine: Go go1.22.0 / linux / 16 workers

📊 CODEBASE METRICS
  files      142
  lines      28,450
  packages   18
  dirs       24
  languages:
    .go 85 files
    .ts 42 files

🔥 HIGH-CHURN HOTSPOTS
  [ 42 commits] backend/auth/session.go
  [ 38 commits] frontend/components/Navbar.tsx

🧟 STALE / ZOMBIE FILES
  [ 184 days] legacy/v1/deprecated_api.go

🛠 TECH DEBT
  TODO   backend/auth/session.go:45  // TODO: implement rate limiting here
  FIXME  frontend/components/Navbar.tsx:112 // FIXME: mobile view overflow bug

⭕ CIRCULAR IMPORTS
  ✗ gitdrift/pkg/a → gitdrift/pkg/b → gitdrift/pkg/a

✔ Analysis complete in 405ms (gitdrift v0.2.0)
```

---

## 🧪 Testing & Performance

```bash
go test -race -cover ./...
go test -bench=. -benchmem -run=^$ ./pkg/analyzer
```

Benchmark suite runs against a synthetic 2,000-file monorepo fixture:

| Benchmark | Description |
|---|---|
| `BenchmarkScanTodos` | Concurrent TODO scan over 2,000 files |
| `BenchmarkAnalyzeRepo` | Concurrent line/file/language metrics |
| `BenchmarkFindImportCycles` | Import-graph build + DFS cycle detection (100 packages) |

## 🛣️ Roadmap

- [ ] Multi-language AST parsing (tree-sitter) for dead-code and boundary violation detection
- [ ] `gitdrift fix` — automated remediation for stale file archiving and cycle refactors
- [ ] GitHub Action for PR comments with drift summaries
- [ ] WASM-in-browser mode for scanning without cloning
