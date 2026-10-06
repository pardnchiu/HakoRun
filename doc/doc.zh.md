# HakoRun - 技術文件

最後更新：2026-10-06

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.25 或更高版本
- 作業系統擇一：
  - Linux：`bwrap`（Bubblewrap）、`systemd-run` 與可用的 systemd user session
  - macOS：系統內建 `sandbox-exec`
- Python 3（`python3`）
- Node.js 與 npm
- TypeScript 執行需求：全域 `tsx`、`typescript`、`esbuild`，以及專案根目錄下的本地 `esbuild`
- Redis（僅在以 `-tags redis` 建置時需要）

## 安裝

### 從原始碼

```bash
git clone https://github.com/pardnchiu/HakoRun.git
cd HakoRun
npm install -g tsx typescript esbuild
npm install esbuild
make build
```

產出的執行檔為 `bin/hako`。

### 改用 Redis 儲存

```bash
make build redis
```

等同於 `go build -tags redis -o bin/hako ./cmd/api`。

> HakoRun 以啟動時的工作目錄解析 `internal/resource/wrapper.{py,js,ts}`，因此執行檔必須在專案根目錄啟動；`go install` 取得的獨立執行檔無法找到 wrapper。

### Linux 相依自動安裝

Linux 啟動時若偵測不到 `node`、`tsc`、`esbuild` 或 `python3`，會透過 `sudo` 以系統套件管理器安裝 `bubblewrap`、`nodejs`、`npm`、`python3`：

| 發行版 | 套件管理器 |
|--------|------------|
| Ubuntu、Debian | `apt` |
| Rocky Linux、Alma Linux、Fedora、RedHat | `dnf` |
| Arch Linux | `pacman` |
| Alpine Linux | `apk` |

自動安裝不包含 `tsc` 與 `esbuild`，請先以 `npm install -g typescript esbuild` 安裝，避免每次啟動都觸發安裝流程。macOS 不做相依檢查。

## 設定

### 環境變數

`Makefile` 會載入專案根目錄的 `.env`：

```bash
cp .env.example .env
```

| 變數 | 必要 | 預設值 | 說明 |
|------|------|--------|------|
| `HTTP_PORT` | 否 | `8080` | HTTP 服務埠號 |
| `CODE_MAX_SIZE` | 否 | `262144`（256 KiB） | `/run` 與 `/run-now` 請求 body 的位元組上限 |
| `TIMEOUT_SCRIPT` | 否 | `30` | 腳本執行逾時秒數；實際截止時間為此值再加 5 秒 |
| `MAX_CPUS` | 否 | `1` | `hakorun.slice` 的 CPU 配額（核心數，`CPUQuota = N × 100%`），僅 Linux |
| `MAX_MEMORY` | 否 | `128M` | `hakorun.slice` 的 `MemoryMax`（swap 固定為 0），僅 Linux |
| `REDIS_HOST` | 否 | `localhost` | Redis 主機（僅 `-tags redis`） |
| `REDIS_PORT` | 否 | `6379` | Redis 埠號（僅 `-tags redis`） |
| `REDIS_PASSWORD` | 否 | 空 | Redis 密碼（僅 `-tags redis`） |
| `REDIS_DB` | 否 | `0` | Redis 資料庫編號（僅 `-tags redis`） |
| `REDIS_TIMEOUT_SECONDS` | 否 | `5` | Redis 連線／讀寫逾時秒數（僅 `-tags redis`） |

### 儲存位置

| 後端 | 建置方式 | 位置 |
|------|----------|------|
| ToriiDB | 預設 | `~/.config/pardnchiu/hakorun` |
| Redis | `-tags redis` | `REDIS_HOST:REDIS_PORT` 的 `REDIS_DB` |

### systemd Slice（Linux）

啟動時寫入 `~/.config/systemd/user/hakorun.slice` 並執行 `systemctl --user daemon-reload` 與 `start`。建立失敗僅記錄警告，服務仍會啟動。

## 使用方式

### 啟動伺服器

```bash
make run
```

或執行已建置的執行檔（需在專案根目錄）：

```bash
./bin/hako
```

收到 `SIGINT`／`SIGTERM` 時，伺服器會在 5 秒內優雅關閉。

### 基礎：上傳腳本

```bash
curl --fail-with-body -X POST http://localhost:8080/upload \
  -H "Content-Type: application/json" \
  -d '{
    "path": "math/add",
    "language": "python",
    "code": "return event.get(\"a\", 0) + event.get(\"b\", 0)"
  }'
```

回應（`version` 為上傳當下的 Unix 秒數）：

```json
{
  "path": "math/add",
  "language": "python",
  "version": 1791273600
}
```

### 基礎：執行已儲存腳本

```bash
curl --fail-with-body -X POST http://localhost:8080/run/math/add \
  -H "Content-Type: application/json" \
  -d '{"input": "{\"a\": 3, \"b\": 5}"}'
```

回應：

```json
{
  "data": 8,
  "type": "number"
}
```

`/run` 必須帶 JSON body；無輸入時傳 `{}`。

### 進階：釘選版本

```bash
curl --fail-with-body -X POST "http://localhost:8080/run/math/add?version=1791273600" \
  -H "Content-Type: application/json" \
  -d '{"input": "{\"a\": 3, \"b\": 5}"}'
```

`version` 無法解析為整數時會改用最新版本；版本不存在則回傳 `404`。

### 進階：立即執行（不儲存）

```bash
curl --fail-with-body -X POST http://localhost:8080/run-now \
  -H "Content-Type: application/json" \
  -d '{
    "language": "javascript",
    "code": "return { sum: event.a + event.b }",
    "input": "{\"a\": 10, \"b\": 20}"
  }'
```

回應：

```json
{
  "data": { "sum": 30 },
  "type": "json"
}
```

### 進階：SSE 串流

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

串流輸出：

```
data: {"event":"log","data":0,"type":"number"}

data: {"event":"log","data":1,"type":"number"}

data: {"event":"log","data":2,"type":"number"}

data: {"event":"result","data":"done","type":"string"}
```

stdout 的每一行在下一行出現時以 `log` 推送，最後一行作為 `result`；腳本寫入任何 stderr、逾時或客戶端斷線時，行程立即被終止並送出 `error` 事件。送出 `result`／`error` 後伺服器會關閉連線。

## API 參考

### 端點

| 方法 | 路徑 | 說明 |
|------|------|------|
| `POST` | `/upload` | 儲存腳本並產生新版本 |
| `POST` | `/run/*targetPath` | 執行已儲存腳本（預設最新版本） |
| `POST` | `/run-now` | 不儲存，直接在沙箱執行提交的程式碼 |

### POST /upload

| 欄位 | 型別 | 必要 | 說明 |
|------|------|------|------|
| `path` | `string` | 是 | 腳本路徑，不可包含 `..` |
| `code` | `string` | 是 | 程式碼內容 |
| `language` | `string` | 是 | `python`、`javascript`、`typescript` |

| 狀態碼 | 內容 | 情境 |
|--------|------|------|
| `200` | `{"path", "language", "version"}` | 儲存成功 |
| `400` | `Invalid request payload` | 缺少欄位或 JSON 格式錯誤 |
| `400` | `Invalid path` | `path` 含 `..` 或 `language` 不支援 |
| `500` | `Failed to save function` | 儲存後端寫入失敗 |

同一路徑在同一秒內重複上傳會產生相同版本號，後者覆蓋前者的程式碼。

### POST /run/*targetPath

| 參數 | 位置 | 型別 | 必要 | 說明 |
|------|------|------|------|------|
| `version` | query | `int64` | 否 | 目標版本；省略或非整數時取最新版本 |
| `input` | body | `string` | 否 | JSON 字串，腳本內以 `event`／`input` 存取 |
| `stream` | body | `bool` | 否 | `true` 時以 SSE 回傳 |

| 狀態碼 | 情境 |
|--------|------|
| `200` | 執行成功（JSON 或 SSE） |
| `400` | body 不是合法 JSON 或超過 `CODE_MAX_SIZE` |
| `404` | `script not found` 或 `assign version not found` |
| `500` | 非串流模式下執行失敗或逾時（`failed to run: ...`） |

### POST /run-now

| 欄位 | 型別 | 必要 | 說明 |
|------|------|------|------|
| `code` | `string` | 是 | 程式碼內容，不可為空白 |
| `language` | `string` | 是 | `python`、`javascript`、`typescript` |
| `input` | `string` | 否 | JSON 字串輸入 |
| `stream` | `bool` | 否 | `true` 時以 SSE 回傳 |

| 狀態碼 | 情境 |
|--------|------|
| `200` | 執行成功（JSON 或 SSE） |
| `400` | body 錯誤、`unsupported language`、`code is required` |
| `500` | 非串流模式下執行失敗或逾時 |

### 回應格式

非串流模式取 stdout 中最後一行合法 JSON 作為結果；沒有合法 JSON 時回傳過濾掉 Node.js 警告後的完整輸出。

| `type` | 判斷條件 |
|--------|----------|
| `string` | 結果為 JSON 字串 |
| `number` | 結果為 JSON 數值 |
| `json` | 結果為 JSON 物件、陣列、布林或 `null` |
| `text` | 結果不是合法 JSON |

### SSE 事件

每則事件格式為 `data: {"event", "data", "type"}`，`type` 判斷規則同上。

| `event` | 說明 |
|---------|------|
| `log` | stdout 的中間輸出行 |
| `result` | stdout 最後一行（換行字元替換為空白） |
| `error` | 終止原因：stderr 內容、逾時、客戶端斷線或非零退出碼 |

### 腳本執行契約

| 語言 | Runtime | 腳本可用變數 | 取得結果的方式 |
|------|---------|--------------|----------------|
| Python | `python3 -u` | `event`、`input`（皆為解析後的 `input`） | 頂層 `return` 值，以 `json.dumps` 輸出 |
| JavaScript | `node`（`vm`，包在 async function） | `event`、`input` | 頂層 `return` 值（可 `await`），以 `JSON.stringify` 輸出 |
| TypeScript | `tsx`（`esbuild` 轉譯為 CJS 後以 `vm` 執行） | `event`、`input` | 頂層 `return` 值或全域 `result` |

### 沙箱限制

| 項目 | Linux（`systemd-run` + `bwrap`） | macOS（`sandbox-exec`） |
|------|----------------------------------|-------------------------|
| 檔案系統 | 唯讀掛載 `/usr`、`/lib`、`/lib64` 與 wrapper；`/tmp`、`/home/sandbox` 為 tmpfs | 全域可讀，僅 `$HOME` 可寫 |
| 網路 | 停用（`--unshare-net`） | 允許 |
| 權限 | `--unshare-all`、`--cap-drop ALL`、`--new-session`、`--die-with-parent` | `deny default` 為基底的 seatbelt profile |
| 資源上限 | `hakorun.slice`（`MAX_CPUS`、`MAX_MEMORY`） | 無 |
| 環境變數 | 重設 `HOME`、`PATH`、`TMPDIR`、`LANG`，移除 `LD_PRELOAD`、`LD_LIBRARY_PATH` | 繼承伺服器環境 |

Linux 沙箱內的 `PATH` 為 `/usr/local/bin:/usr/bin:/bin`，且只掛載 `/usr`，因此全域 `tsx` 必須安裝在 `/usr` 底下（npm 預設 prefix `/usr/local` 即可）。TypeScript 執行時會額外唯讀掛載專案根目錄，並以其 `node_modules` 作為 `NODE_PATH`。

***

©️ 2025 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
