# go-faas - 技術文件

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.23 或更高版本
- Linux（Ubuntu、Debian、Fedora、Arch Linux、Alpine Linux；生產沙箱以 Linux 為主）
- Redis 伺服器
- Bubblewrap（`bwrap`）
- Node.js（含 npm）與 TypeScript 執行環境（`tsx`）
- Python 3
- systemd（Slice 資源管控）

## 安裝

### 從原始碼

```bash
git clone https://github.com/pardnchiu/go-faas.git
cd go-faas
go build -o go-faas cmd/api/main.go
```

### 使用 go install

```bash
go install github.com/pardnchiu/go-faas/cmd/api@latest
```

### 安裝 TypeScript 相依

```bash
npm install
```

> 首次啟動時，程式會檢查 `bwrap`、`node`、`python3` 等相依；若缺失，會嘗試透過系統套件管理器安裝。

## 設定

### 環境變數

複製 `.env.example` 並填入值：

```bash
cp .env.example .env
```

| 變數 | 必要 | 預設值 | 說明 |
|------|------|--------|------|
| `HTTP_PORT` | 否 | `8080` | HTTP 服務埠號 |
| `MAX_CPUS` | 否 | `1` | 沙箱 CPU 配額（核心數） |
| `MAX_MEMORY` | 否 | `128M` | 沙箱記憶體上限 |
| `CODE_MAX_SIZE` | 否 | `262144`（256KB） | 允許的最大程式碼位元組數 |
| `TIMEOUT_SCRIPT` | 否 | `30` | 腳本執行逾時（秒） |
| `REDIS_HOST` | 否 | `localhost` | Redis 主機 |
| `REDIS_PORT` | 否 | `6379` | Redis 埠號 |
| `REDIS_PASSWORD` | 否 | 空 | Redis 密碼 |
| `REDIS_DB` | 否 | `0` | Redis 資料庫編號 |
| `REDIS_TIMEOUT_SECONDS` | 否 | `5` | Redis 連線逾時（秒） |

## 使用方式

### 啟動伺服器

```bash
./go-faas
```

### 基礎：上傳腳本

將腳本存入 Redis 並取得版本號：

```bash
curl -X POST http://localhost:8080/upload \
  -H "Content-Type: application/json" \
  -d '{
    "path": "math/add",
    "language": "python",
    "code": "return event.get(\"a\", 0) + event.get(\"b\", 0)"
  }'
```

回應：

```json
{
  "path": "math/add",
  "language": "python",
  "version": 1739000000
}
```

### 基礎：執行已儲存腳本

執行路徑的最新版本：

```bash
curl -X POST http://localhost:8080/run/math/add \
  -H "Content-Type: application/json" \
  -d '{
    "input": "{\"a\": 3, \"b\": 5}"
  }'
```

指定版本：

```bash
curl -X POST "http://localhost:8080/run/math/add?version=1739000000" \
  -H "Content-Type: application/json" \
  -d '{
    "input": "{\"a\": 3, \"b\": 5}"
  }'
```

回應：

```json
{
  "data": 8,
  "type": "number"
}
```

### 進階：立即執行（不儲存）

```bash
curl -X POST http://localhost:8080/run-now \
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

### 進階：SSE 串流模式

設定 `stream: true` 啟用 Server-Sent Events：

```bash
curl -X POST http://localhost:8080/run-now \
  -H "Content-Type: application/json" \
  -d '{
    "language": "python",
    "code": "import time\nfor i in range(5):\n    print(i)\n    time.sleep(0.5)\nreturn \"done\"",
    "input": "{}",
    "stream": true
  }'
```

串流回應格式：

```
data: {"event":"log","data":"0","type":"text"}

data: {"event":"log","data":"1","type":"text"}

data: {"event":"result","data":"done","type":"string"}
```

## API 參考

### 端點

| 方法 | 路徑 | 說明 |
|------|------|------|
| `POST` | `/upload` | 上傳腳本至 Redis |
| `POST` | `/run/*targetPath` | 執行已儲存腳本 |
| `POST` | `/run-now` | 立即執行提交的程式碼 |

### POST /upload

上傳並儲存腳本，回傳版本號。

**Request Body：**

| 欄位 | 型別 | 必要 | 說明 |
|------|------|------|------|
| `path` | `string` | 是 | 腳本存取路徑（不得含 `..`） |
| `code` | `string` | 是 | 程式碼內容 |
| `language` | `string` | 是 | 語言（`python`、`javascript`、`typescript`） |

**Response：**

```json
{
  "path": "string",
  "language": "string",
  "version": 1739000000
}
```

### POST /run/*targetPath

從 Redis 取得腳本並在沙箱中執行。

**Query Parameters：**

| 參數 | 型別 | 必要 | 說明 |
|------|------|------|------|
| `version` | `int64` | 否 | 目標版本；預設最新 |

**Request Body：**

| 欄位 | 型別 | 必要 | 說明 |
|------|------|------|------|
| `input` | `string` | 否 | JSON 字串輸入，腳本內以 `event` 存取 |
| `stream` | `bool` | 否 | 啟用 SSE 串流輸出 |

### POST /run-now

不經 Redis 直接提交程式碼至沙箱執行。

**Request Body：**

| 欄位 | 型別 | 必要 | 說明 |
|------|------|------|------|
| `code` | `string` | 是 | 程式碼內容 |
| `language` | `string` | 是 | 語言（`python`、`javascript`、`typescript`） |
| `input` | `string` | 否 | JSON 字串輸入 |
| `stream` | `bool` | 否 | 啟用 SSE 串流輸出 |

### 回應格式

標準回應會自動判斷回傳資料型別：

| `type` | 說明 |
|--------|------|
| `string` | 字串 |
| `number` | 數值 |
| `json` | JSON 物件或陣列 |
| `text` | 純文字（非合法 JSON） |

### SSE 事件格式

| `event` | 說明 |
|---------|------|
| `log` | 腳本中間輸出（`print` / `console.log`） |
| `result` | 最終執行結果 |
| `error` | 執行錯誤訊息 |

### 支援語言

| 語言 | Runtime | 副檔名 | 腳本內全域變數 |
|------|---------|--------|----------------|
| Python | `python3` | `.py` | `event`、`input` |
| JavaScript | `node` | `.js` | `event`、`input` |
| TypeScript | `tsx` | `.ts` | `event`、`input` |

***

©️ 2025 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
