# go-faas - Architecture

> Back to [README](../README.md)

## Overview

```mermaid
graph TB
    Client[HTTP Client] -->|POST /upload| Upload[Upload Handler]
    Client -->|POST /run/*path| Run[Run Handler]
    Client -->|POST /run-now| RunNow[RunNow Handler]
    Upload --> Redis[(Redis)]
    Run --> Redis
    Run --> Sandbox[Sandbox]
    RunNow --> Sandbox
    Sandbox --> Response[JSON / SSE]
    Response --> Client
```

## Module: Router

Creates the Gin engine, registers HTTP routes, and binds the listen port.

```mermaid
graph TB
    subgraph Router
        CreateServer[CreateServer] --> Gin[gin.Default]
        Gin --> Routes[POST /upload /run /run-now]
        CreateServer --> Port[HTTP_PORT]
    end
    Routes --> Handler[handler package]
```

## Module: Handler

Handles upload, stored-script execution, immediate run, and SSE streaming.

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

## Module: Database

Stores script metadata, code blobs, and version sets in Redis.

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

## Module: Sandbox

Builds the isolation environment per platform: Linux uses systemd-run + bwrap; darwin uses sandbox-exec.

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

## Module: Checker

Checks Node, TypeScript, esbuild, and Python at startup and installs missing packages via the OS package manager when needed.

```mermaid
graph TB
    subgraph Checker
        CheckPackage[CheckPackage] --> OSName[getOSName]
        CheckPackage --> Missing[isMissing]
        Missing --> Install[execCommand apt/dnf/pacman/apk]
    end
```

## Data Flow

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
    Handler-->>Client: JSON or SSE
```

## State Machine

```mermaid
stateDiagram-v2
    [*] --> Idle
    Idle --> Preparing: receive Run / RunNow
    Preparing --> Running: start sandbox process
    Running --> Streaming: stream=true
    Running --> Completed: clean exit
    Streaming --> Completed: emit result
    Running --> Failed: timeout / stderr / non-zero exit
    Streaming --> Failed: client disconnect / error
    Failed --> Idle
    Completed --> Idle
```

***

©️ 2025 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
