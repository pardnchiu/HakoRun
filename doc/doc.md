# HakoRun - Documentation

Last updated: 2026-10-06

> Back to [README](../README.md)

## Prerequisites

- Go 1.25 or higher
- One of the following operating systems:
  - Linux: `bwrap` (Bubblewrap), `systemd-run`, and a working systemd user session
  - macOS: the built-in `sandbox-exec`
- Python 3 (`python3`)
- Node.js and npm
- TypeScript runtime: global `tsx`, `typescript`, `esbuild`, plus a local `esbuild` in the project root
- Redis (only when built with `-tags redis`)

## Installation

### From Source

```bash
git clone https://github.com/pardnchiu/HakoRun.git
cd HakoRun
npm install -g tsx typescript esbuild
npm install esbuild
make build
```

The binary lands at `bin/hako`.

### Redis Storage

```bash
make build redis
```

Equivalent to `go build -tags redis -o bin/hako ./cmd/api`.

> HakoRun resolves `internal/resource/wrapper.{py,js,ts}` against the working directory at startup, so start the binary from the project root; a standalone binary from `go install` cannot locate the wrappers.

### Linux Dependency Auto-install

On Linux, when `node`, `tsc`, `esbuild`, or `python3` is missing at startup, HakoRun installs `bubblewrap`, `nodejs`, `npm`, and `python3` through `sudo` and the system package manager:

| Distribution | Package Manager |
|--------------|-----------------|
| Ubuntu, Debian | `apt` |
| Rocky Linux, Alma Linux, Fedora, RedHat | `dnf` |
| Arch Linux | `pacman` |
| Alpine Linux | `apk` |

Auto-install does not cover `tsc` or `esbuild`; install them first with `npm install -g typescript esbuild` so startup does not re-trigger the install. macOS skips the dependency check.

## Configuration

### Environment Variables

The `Makefile` loads `.env` from the project root:

```bash
cp .env.example .env
```

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `HTTP_PORT` | No | `8080` | HTTP listen port |
| `CODE_MAX_SIZE` | No | `262144` (256 KiB) | Request body byte limit for `/run` and `/run-now` |
| `TIMEOUT_SCRIPT` | No | `30` | Script timeout in seconds; the effective deadline adds 5 seconds |
| `MAX_CPUS` | No | `1` | CPU quota of `hakorun.slice` in cores (`CPUQuota = N × 100%`), Linux only |
| `MAX_MEMORY` | No | `128M` | `MemoryMax` of `hakorun.slice` (swap fixed at 0), Linux only |
| `REDIS_HOST` | No | `localhost` | Redis host (`-tags redis` only) |
| `REDIS_PORT` | No | `6379` | Redis port (`-tags redis` only) |
| `REDIS_PASSWORD` | No | empty | Redis password (`-tags redis` only) |
| `REDIS_DB` | No | `0` | Redis database index (`-tags redis` only) |
| `REDIS_TIMEOUT_SECONDS` | No | `5` | Redis dial/read/write timeout in seconds (`-tags redis` only) |

### Storage Location

| Backend | Build | Location |
|---------|-------|----------|
| ToriiDB | default | `~/.config/pardnchiu/hakorun` |
| Redis | `-tags redis` | `REDIS_DB` on `REDIS_HOST:REDIS_PORT` |

### systemd Slice (Linux)

At startup HakoRun writes `~/.config/systemd/user/hakorun.slice`, then runs `systemctl --user daemon-reload` and `start`. A failure only logs a warning; the server still starts.

## Usage

### Start the Server

```bash
make run
```

Or run the built binary from the project root:

```bash
./bin/hako
```

On `SIGINT` or `SIGTERM` the server shuts down gracefully within 5 seconds.

### Basic: Upload a Script

```bash
curl --fail-with-body -X POST http://localhost:8080/upload \
  -H "Content-Type: application/json" \
  -d '{
    "path": "math/add",
    "language": "python",
    "code": "return event.get(\"a\", 0) + event.get(\"b\", 0)"
  }'
```

Response (`version` is the Unix timestamp in seconds at upload time):

```json
{
  "path": "math/add",
  "language": "python",
  "version": 1791273600
}
```

### Basic: Run a Stored Script

```bash
curl --fail-with-body -X POST http://localhost:8080/run/math/add \
  -H "Content-Type: application/json" \
  -d '{"input": "{\"a\": 3, \"b\": 5}"}'
```

Response:

```json
{
  "data": 8,
  "type": "number"
}
```

`/run` requires a JSON body; send `{}` when there is no input.

### Advanced: Pin a Version

```bash
curl --fail-with-body -X POST "http://localhost:8080/run/math/add?version=1791273600" \
  -H "Content-Type: application/json" \
  -d '{"input": "{\"a\": 3, \"b\": 5}"}'
```

A `version` that does not parse as an integer falls back to the latest version; a missing version returns `404`.

### Advanced: Run Now (No Storage)

```bash
curl --fail-with-body -X POST http://localhost:8080/run-now \
  -H "Content-Type: application/json" \
  -d '{
    "language": "javascript",
    "code": "return { sum: event.a + event.b }",
    "input": "{\"a\": 10, \"b\": 20}"
  }'
```

Response:

```json
{
  "data": { "sum": 30 },
  "type": "json"
}
```

### Advanced: SSE Streaming

```bash
curl -N -X POST http://localhost:8080/run-now \
  -H "Content-Type: application/json" \
  -d '{
    "language": "python",
    "code": "import time\nfor i in range(3):\n    print(i)\n    time.sleep(0.5)\nreturn \"done\"",
    "input": "{}",
    "stream": true
  }'
```

Stream output:

```
data: {"event":"log","data":0,"type":"number"}

data: {"event":"log","data":1,"type":"number"}

data: {"event":"log","data":2,"type":"number"}

data: {"event":"result","data":"done","type":"string"}
```

Each stdout line is pushed as `log` once the next line arrives, and the final line becomes `result`; any stderr write, timeout, or client disconnect kills the process and emits an `error` event. The server closes the connection after sending `result` or `error`.

## API Reference

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/upload` | Store a script as a new version |
| `POST` | `/run/*targetPath` | Run a stored script (latest version by default) |
| `POST` | `/run-now` | Run submitted code in the sandbox without storing it |

### POST /upload

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | `string` | Yes | Script path; must not contain `..` |
| `code` | `string` | Yes | Source code |
| `language` | `string` | Yes | `python`, `javascript`, or `typescript` |

| Status | Body | Case |
|--------|------|------|
| `200` | `{"path", "language", "version"}` | Stored |
| `400` | `Invalid request payload` | Missing field or malformed JSON |
| `400` | `Invalid path` | `path` contains `..` or `language` is unsupported |
| `500` | `Failed to save function` | Storage backend write failed |

Two uploads to the same path within the same second share a version number, and the later code overwrites the earlier one.

### POST /run/*targetPath

| Parameter | In | Type | Required | Description |
|-----------|----|------|----------|-------------|
| `version` | query | `int64` | No | Target version; omitted or non-integer means latest |
| `input` | body | `string` | No | JSON string, exposed to the script as `event` / `input` |
| `stream` | body | `bool` | No | `true` responds over SSE |

| Status | Case |
|--------|------|
| `200` | Success (JSON or SSE) |
| `400` | Body is not valid JSON or exceeds `CODE_MAX_SIZE` |
| `404` | `script not found` or `assign version not found` |
| `500` | Non-stream execution failed or timed out (`failed to run: ...`) |

### POST /run-now

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `code` | `string` | Yes | Source code; must not be blank |
| `language` | `string` | Yes | `python`, `javascript`, or `typescript` |
| `input` | `string` | No | JSON string input |
| `stream` | `bool` | No | `true` responds over SSE |

| Status | Case |
|--------|------|
| `200` | Success (JSON or SSE) |
| `400` | Bad body, `unsupported language`, or `code is required` |
| `500` | Non-stream execution failed or timed out |

### Response Format

Non-stream mode takes the last valid JSON line on stdout as the result; without one it returns the full output with Node.js warnings filtered out.

| `type` | Condition |
|--------|-----------|
| `string` | Result is a JSON string |
| `number` | Result is a JSON number |
| `json` | Result is a JSON object, array, boolean, or `null` |
| `text` | Result is not valid JSON |

### SSE Events

Each event is `data: {"event", "data", "type"}`, with `type` following the rules above.

| `event` | Description |
|---------|-------------|
| `log` | Intermediate stdout line |
| `result` | Final stdout line (newlines replaced with spaces) |
| `error` | Termination reason: stderr content, timeout, client disconnect, or non-zero exit |

### Script Runtime Contract

| Language | Runtime | Script Globals | Result |
|----------|---------|----------------|--------|
| Python | `python3 -u` | `event`, `input` (both the parsed `input`) | Top-level `return` value, printed with `json.dumps` |
| JavaScript | `node` (`vm`, wrapped in an async function) | `event`, `input` | Top-level `return` value (supports `await`), printed with `JSON.stringify` |
| TypeScript | `tsx` (`esbuild` transpiles to CJS, then `vm` runs it) | `event`, `input` | Top-level `return` value or global `result` |

### Sandbox Boundaries

| Aspect | Linux (`systemd-run` + `bwrap`) | macOS (`sandbox-exec`) |
|--------|----------------------------------|------------------------|
| Filesystem | Read-only `/usr`, `/lib`, `/lib64`, and the wrapper; tmpfs `/tmp` and `/home/sandbox` | Read everywhere, write only under `$HOME` |
| Network | Disabled (`--unshare-net`) | Allowed |
| Privileges | `--unshare-all`, `--cap-drop ALL`, `--new-session`, `--die-with-parent` | Seatbelt profile based on `deny default` |
| Resource caps | `hakorun.slice` (`MAX_CPUS`, `MAX_MEMORY`) | None |
| Environment | Resets `HOME`, `PATH`, `TMPDIR`, `LANG`; unsets `LD_PRELOAD`, `LD_LIBRARY_PATH` | Inherits the server environment |

Inside the Linux sandbox `PATH` is `/usr/local/bin:/usr/bin:/bin` and only `/usr` is mounted, so the global `tsx` must live under `/usr` (npm's default `/usr/local` prefix works). TypeScript runs additionally mount the project root read-only and use its `node_modules` as `NODE_PATH`.

***

©️ 2025 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
