# HakoRun - 架構

最後更新：2026-10-06

> 返回 [README](./README.zh.md)

## 概覽

```mermaid
graph TB
    Main[cmd/api 進入點] --> Checker[Checker 相依檢查]
    Main --> DB[Database 初始化]
    Main --> Slice[Sandbox Slice]
    Main --> Router[Router]
    Client[客戶端] -->|POST /upload| Upload[Upload]
    Client -->|POST /run/*path| Run[Run]
    Client -->|POST /run-now| RunNow[RunNow]
    Router --> Upload
    Router --> Run
    Router --> RunNow
    Upload --> Store[(ToriiDB / Redis)]
    Run --> Store
    Run --> Sandbox[沙箱]
    RunNow --> Sandbox
    Sandbox --> Out[JSON / SSE]
    Out --> Client
```

## 模組: Entry

依序執行相依檢查、儲存初始化、Slice 建立與 HTTP 服務，收到訊號後優雅關閉。

```mermaid
graph TB
    subgraph Entry[cmd/api]
        Start[main] --> Check[checker.CheckPackage]
        Check -->|失敗| Exit[結束行程]
        Check --> Init[database.Init]
        Init -->|失敗| Exit
        Init --> NewSlice[sandbox.NewSlice]
        NewSlice -->|失敗僅警告| Serve[CreateServer + ListenAndServe]
        NewSlice --> Serve
        Serve --> Signal[等待 SIGINT / SIGTERM]
        Signal --> Shutdown[Shutdown 5 秒逾時]
        Shutdown --> Close[database.Close]
    end
```

## 模組: Router

建立 Gin 引擎、註冊三條路由並綁定 `HTTP_PORT`。

```mermaid
graph TB
    subgraph Router
        CreateServer[CreateServer] --> Port[HTTP_PORT 預設 8080]
        CreateServer --> Gin[gin.Default]
        Gin --> R1[POST /upload]
        Gin --> R2[POST /run/*targetPath]
        Gin --> R3[POST /run-now]
    end
    R1 --> Upload[handler.Upload]
    R2 --> Run[handler.Run]
    R3 --> RunNow[handler.RunNow]
```

## 模組: Handler

驗證請求、讀取腳本，依 `stream` 分流至一次性執行或 SSE 串流執行。

```mermaid
graph TB
    subgraph Handler
        Upload[Upload] --> Validate[檢查 .. 與 language]
        Validate --> Add[DB.Add]
        Run[Run] --> Body[getRunBody 限制 CODE_MAX_SIZE]
        RunNow[RunNow] --> Body
        Run --> Get[DB.Get path + version]
        RunNow --> Check[檢查 language 與 code]
        Get --> Core[run]
        Check --> Core
        Core -->|stream=false| RunScript[runScript]
        Core -->|stream=true| RunSSE[runScriptWithSSE]
        RunScript --> Pick[取最後一行合法 JSON]
        Pick --> Result[sendResult 標註 type]
        RunSSE --> Event[sendEvent log]
        RunSSE --> Done[sendDone result / error]
    end
    RunScript --> Cmd[sandbox.SandboxCommand]
    RunSSE --> Cmd
```

## 模組: Database

以建置標籤選擇後端，兩者共用 `backend` 介面與相同的 key 配置。

```mermaid
classDiagram
    class backend {
        <<interface>>
        +Add(ctx, Script) int64, error
        +Get(ctx, path, version) Script, error
        +Close() error
    }
    class Script {
        +Path string
        +Code string
        +Language string
        +Timestamp int64
    }
    class toriiBackend {
        預設建置
        dir ~/.config/pardnchiu/hakorun
    }
    class redisBackend {
        build -tags redis
        REDIS_HOST:REDIS_PORT
    }
    backend <|.. toriiBackend
    backend <|.. redisBackend
    backend ..> Script
```

```mermaid
graph TB
    subgraph Keys[Key 配置 hash = md5 path]
        Meta[meta:hash 路徑、語言、最新版本]
        Code[code:hash:timestamp 程式碼]
        Versions[版本清單 ToriiDB 存於 meta / Redis 為 meta:hash:version 集合]
    end
    Add[Add] -->|先寫| Code
    Add -->|後寫| Meta
    Add --> Versions
    Get[Get] --> Meta
    Meta -->|version=0 取 latest| Code
```

## 模組: Sandbox

依平台組出受限的子行程指令，wrapper 由 stdin 讀取 `{code, input}`。

```mermaid
graph TB
    subgraph Sandbox
        SandboxCommand[SandboxCommand] -->|linux| SystemdRun[systemd-run --user --scope --slice=hakorun.slice]
        SystemdRun --> Bwrap[bwrap unshare-all / cap-drop ALL / 唯讀 /usr]
        SandboxCommand -->|darwin| Seatbelt[sandbox-exec deny default / 僅 HOME 可寫]
        Bwrap --> Runtime[python3 / node / tsx]
        Seatbelt --> Runtime
        Runtime --> Wrapper[internal/resource/wrapper.py / js / ts]
        NewSlice[NewSlice linux] --> SliceFile[hakorun.slice CPUQuota / MemoryMax]
    end
    SliceFile -.套用.-> SystemdRun
```

## 模組: Checker

Linux 啟動時偵測執行環境，缺少時以套件管理器安裝；macOS 為 no-op。

```mermaid
graph TB
    subgraph Checker
        CheckPackage[CheckPackage] --> OS[getOSName 讀 /etc/os-release]
        CheckPackage --> Missing[isMissing node / tsc / esbuild / python]
        Missing -->|有缺| Exec[execCommand]
        Exec --> Apt[apt Ubuntu / Debian]
        Exec --> Dnf[dnf Rocky / Alma / Fedora / RedHat]
        Exec --> Pacman[pacman Arch]
        Exec --> Apk[apk Alpine]
        Apt --> Install[sudo 安裝 bubblewrap nodejs npm python3]
        Dnf --> Install
        Pacman --> Install
        Apk --> Install
    end
```

## 資料流

```mermaid
sequenceDiagram
    participant Client as 客戶端
    participant Handler
    participant Store as ToriiDB / Redis
    participant Sandbox as 沙箱
    participant Wrapper
    Client->>Handler: POST /upload
    Handler->>Store: Add code 再寫 meta
    Store-->>Handler: version 時間戳
    Handler-->>Client: path, language, version
    Client->>Handler: POST /run/*path?version=
    Handler->>Store: Get path, version
    Store-->>Handler: code, language
    Handler->>Sandbox: SandboxCommand(language)
    Sandbox->>Wrapper: stdin {code, input}
    Wrapper->>Wrapper: 解析 input 為 event 並執行
    Wrapper-->>Handler: stdout / stderr
    alt stream=false
        Handler-->>Client: {data, type}
    else stream=true
        Handler-->>Client: log 事件 ...
        Handler-->>Client: result 或 error 後關閉連線
    end
```

## 狀態機

```mermaid
stateDiagram-v2
    [*] --> Received: 收到 Run / RunNow
    Received --> Rejected: body 錯誤 / 腳本或版本不存在
    Received --> Running: 啟動沙箱行程
    Running --> Streaming: stream=true
    Running --> Completed: 行程結束
    Running --> Failed: 逾時 / 非零退出
    Streaming --> Completed: 送出 result
    Streaming --> Failed: stderr 輸出 / 逾時 / 客戶端斷線
    Rejected --> [*]
    Completed --> [*]
    Failed --> [*]
```

***

©️ 2025 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
