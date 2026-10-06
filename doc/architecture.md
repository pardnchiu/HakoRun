# HakoRun - Architecture

Last updated: 2026-10-06

> Back to [README](../README.md)

## Overview

```mermaid
graph TB
    Main[cmd/api entry] --> Checker[Checker dependency check]
    Main --> DB[Database init]
    Main --> Slice[Sandbox slice]
    Main --> Router[Router]
    Client[Client] -->|POST /upload| Upload[Upload]
    Client -->|POST /run/*path| Run[Run]
    Client -->|POST /run-now| RunNow[RunNow]
    Router --> Upload
    Router --> Run
    Router --> RunNow
    Upload --> Store[(ToriiDB / Redis)]
    Run --> Store
    Run --> Sandbox[Sandbox]
    RunNow --> Sandbox
    Sandbox --> Out[JSON / SSE]
    Out --> Client
```

## Module: Entry

Runs the dependency check, storage init, slice setup, and HTTP server in order, then shuts down gracefully on signal.

```mermaid
graph TB
    subgraph Entry[cmd/api]
        Start[main] --> Check[checker.CheckPackage]
        Check -->|fail| Exit[exit process]
        Check --> Init[database.Init]
        Init -->|fail| Exit
        Init --> NewSlice[sandbox.NewSlice]
        NewSlice -->|fail warns only| Serve[CreateServer + ListenAndServe]
        NewSlice --> Serve
        Serve --> Signal[wait SIGINT / SIGTERM]
        Signal --> Shutdown[Shutdown 5s timeout]
        Shutdown --> Close[database.Close]
    end
```

## Module: Router

Builds the Gin engine, registers three routes, and binds `HTTP_PORT`.

```mermaid
graph TB
    subgraph Router
        CreateServer[CreateServer] --> Port[HTTP_PORT default 8080]
        CreateServer --> Gin[gin.Default]
        Gin --> R1[POST /upload]
        Gin --> R2[POST /run/*targetPath]
        Gin --> R3[POST /run-now]
    end
    R1 --> Upload[handler.Upload]
    R2 --> Run[handler.Run]
    R3 --> RunNow[handler.RunNow]
```

## Module: Handler

Validates requests, loads scripts, and routes by `stream` to one-shot or SSE execution.

```mermaid
graph TB
    subgraph Handler
        Upload[Upload] --> Validate[check .. and language]
        Validate --> Add[DB.Add]
        Run[Run] --> Body[getRunBody capped by CODE_MAX_SIZE]
        RunNow[RunNow] --> Body
        Run --> Get[DB.Get path + version]
        RunNow --> Check[check language and code]
        Get --> Core[run]
        Check --> Core
        Core -->|stream=false| RunScript[runScript]
        Core -->|stream=true| RunSSE[runScriptWithSSE]
        RunScript --> Pick[pick last valid JSON line]
        Pick --> Result[sendResult tags type]
        RunSSE --> Event[sendEvent log]
        RunSSE --> Done[sendDone result / error]
    end
    RunScript --> Cmd[sandbox.SandboxCommand]
    RunSSE --> Cmd
```

## Module: Database

A build tag selects the backend; both share the `backend` interface and the same key layout.

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
        default build
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
    subgraph Keys[Key layout hash = md5 path]
        Meta[meta:hash path, language, latest]
        Code[code:hash:timestamp source]
        Versions[version list in meta for ToriiDB / meta:hash:version set for Redis]
    end
    Add[Add] -->|write first| Code
    Add -->|write last| Meta
    Add --> Versions
    Get[Get] --> Meta
    Meta -->|version=0 uses latest| Code
```

## Module: Sandbox

Builds a confined child-process command per platform; the wrapper reads `{code, input}` from stdin.

```mermaid
graph TB
    subgraph Sandbox
        SandboxCommand[SandboxCommand] -->|linux| SystemdRun[systemd-run --user --scope --slice=hakorun.slice]
        SystemdRun --> Bwrap[bwrap unshare-all / cap-drop ALL / read-only /usr]
        SandboxCommand -->|darwin| Seatbelt[sandbox-exec deny default / write HOME only]
        Bwrap --> Runtime[python3 / node / tsx]
        Seatbelt --> Runtime
        Runtime --> Wrapper[internal/resource/wrapper.py / js / ts]
        NewSlice[NewSlice linux] --> SliceFile[hakorun.slice CPUQuota / MemoryMax]
    end
    SliceFile -.applies.-> SystemdRun
```

## Module: Checker

Detects the runtime toolchain on Linux startup and installs missing packages; a no-op on macOS.

```mermaid
graph TB
    subgraph Checker
        CheckPackage[CheckPackage] --> OS[getOSName reads /etc/os-release]
        CheckPackage --> Missing[isMissing node / tsc / esbuild / python]
        Missing -->|missing| Exec[execCommand]
        Exec --> Apt[apt Ubuntu / Debian]
        Exec --> Dnf[dnf Rocky / Alma / Fedora / RedHat]
        Exec --> Pacman[pacman Arch]
        Exec --> Apk[apk Alpine]
        Apt --> Install[sudo install bubblewrap nodejs npm python3]
        Dnf --> Install
        Pacman --> Install
        Apk --> Install
    end
```

## Data Flow

```mermaid
sequenceDiagram
    participant Client
    participant Handler
    participant Store as ToriiDB / Redis
    participant Sandbox
    participant Wrapper
    Client->>Handler: POST /upload
    Handler->>Store: Add code, then meta
    Store-->>Handler: version timestamp
    Handler-->>Client: path, language, version
    Client->>Handler: POST /run/*path?version=
    Handler->>Store: Get path, version
    Store-->>Handler: code, language
    Handler->>Sandbox: SandboxCommand(language)
    Sandbox->>Wrapper: stdin {code, input}
    Wrapper->>Wrapper: parse input as event and execute
    Wrapper-->>Handler: stdout / stderr
    alt stream=false
        Handler-->>Client: {data, type}
    else stream=true
        Handler-->>Client: log events ...
        Handler-->>Client: result or error, then close
    end
```

## State Machine

```mermaid
stateDiagram-v2
    [*] --> Received: Run / RunNow request
    Received --> Rejected: bad body / script or version missing
    Received --> Running: start sandbox process
    Running --> Streaming: stream=true
    Running --> Completed: process exits
    Running --> Failed: timeout / non-zero exit
    Streaming --> Completed: result sent
    Streaming --> Failed: stderr output / timeout / client disconnect
    Rejected --> [*]
    Completed --> [*]
    Failed --> [*]
```

***

©️ 2025 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
