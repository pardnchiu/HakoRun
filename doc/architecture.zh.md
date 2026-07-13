# go-faas - 架構

> 返回 [README](./README.zh.md)

## 概覽

```mermaid
graph TB
    Client[HTTP 客戶端] -->|POST /upload| Upload[Upload Handler]
    Client -->|POST /run/*path| Run[Run Handler]
    Client -->|POST /run-now| RunNow[RunNow Handler]
    Upload --> Redis[(Redis)]
    Run --> Redis
    Run --> Sandbox[沙箱]
    RunNow --> Sandbox
    Sandbox --> Response[JSON / SSE]
    Response --> Client
```

## 模組: Router

負責建立 Gin 引擎、註冊 HTTP 路由與綁定服務埠。

```mermaid
graph TB
    subgraph Router
        CreateServer[CreateServer] --> Gin[gin.Default]
        Gin --> Routes[POST /upload /run /run-now]
        CreateServer --> Port[HTTP_PORT]
    end
    Routes --> Handler[handler 套件]
```

## 模組: Handler

處理上傳、執行已存腳本、立即執行與 SSE 串流。

```mermaid
graph TB
    subgraph Handler
        Upload[Upload] --> DBAdd[database.Add]
        Run[Run] --> DBGet[database.Get]
        Run --> RunCore[run]
        RunNow[RunNow] --> RunCore
        RunCore -->|stream=false| RunScript[runScript]
        RunCore -->|stream=true| RunSSE[runScriptWithSSE]
        RunScript --> SandboxCmd[sandbox.SandboxCommand]
        RunSSE --> SandboxCmd
    end
```

## 模組: Database

以 Redis 儲存腳本中繼資料、程式碼與版本集合。

```mermaid
graph TB
    subgraph Database
        Init[Init] --> RDB[redis.Client]
        Add[Add] --> Meta[meta:hash]
        Add --> Code[code:hash:ts]
        Add --> Versions[meta:hash:version]
        Get[Get] --> Meta
        Get --> Code
    end
```

## 模組: Sandbox

依平台建立隔離執行環境：Linux 用 systemd-run + bwrap；darwin 用 sandbox-exec。

```mermaid
graph TB
    subgraph Sandbox
        NewSlice[NewSlice] --> SliceUnit[go-faas-slice]
        SandboxCommand[SandboxCommand]
        SandboxCommand -->|linux| SystemdRun[systemd-run]
        SystemdRun --> Bwrap[bwrap]
        Bwrap --> Wrapper[wrapper.py/js/ts]
        SandboxCommand -->|darwin| Seatbelt[sandbox-exec]
        Seatbelt --> Wrapper
    end
```

## 模組: Checker

啟動時檢查 Node、TypeScript、esbuild、Python 等相依，必要時透過套件管理器安裝。

```mermaid
graph TB
    subgraph Checker
        CheckPackage[CheckPackage] --> OSName[getOSName]
        CheckPackage --> Missing[isMissing]
        Missing --> Install[execCommand apt/dnf/pacman/apk]
    end
```

## 資料流

```mermaid
sequenceDiagram
    participant Client
    participant Handler
    participant Redis
    participant Sandbox
    participant Runtime
    Client->>Handler: POST /upload
    Handler->>Redis: Add meta + code + version
    Redis-->>Handler: version
    Handler-->>Client: path, language, version
    Client->>Handler: POST /run/*path
    Handler->>Redis: Get script
    Redis-->>Handler: code, language
    Handler->>Sandbox: SandboxCommand
    Sandbox->>Runtime: wrapper + user code
    Runtime-->>Handler: stdout / stderr
    Handler-->>Client: JSON 或 SSE
```

## 狀態機

```mermaid
stateDiagram-v2
    [*] --> Idle
    Idle --> Preparing: 收到 Run / RunNow
    Preparing --> Running: 啟動沙箱行程
    Running --> Streaming: stream=true
    Running --> Completed: 正常結束
    Streaming --> Completed: 送出 result
    Running --> Failed: 逾時 / stderr / 非零退出
    Streaming --> Failed: 客戶端中斷 / 錯誤
    Failed --> Idle
    Completed --> Idle
```

***

©️ 2025 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
