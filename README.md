# godoc

[![Tests](https://github.com/dwisiswant0/godoc/actions/workflows/tests.yaml/badge.svg?branch=master)](https://github.com/dwisiswant0/godoc/actions/workflows/tests.yaml)
[![Go Reference](https://pkg.go.dev/badge/go.dw1.io/godoc.svg)](https://pkg.go.dev/go.dw1.io/godoc)
[![Go Report Card](https://goreportcard.com/badge/go.dw1.io/godoc)](https://goreportcard.com/report/go.dw1.io/godoc)

A suite of tools for exploring Go API documentation.

<img src="https://github.com/user-attachments/assets/57337c20-b523-4088-a5a5-a595ff5b133a" href="#" height="480">

It ships with:

- **godoc-cli** – ✨ a richly rendered terminal viewer for Go docs.
- **godoc-mcp** – 🤖 an [MCP](https://modelcontextprotocol.io/) server that lets AI tooling fetch Go docs on demand.
- **`go.dw1.io/godoc`** – 📦 the library powering both binaries, so you can embed doc loading in your own apps.

## Installation

> [!NOTE]
> Requires Go 1.25 or newer.

Install the [godoc CLI](#cli) and [godoc MCP](#mcp) from source:

```bash
go install go.dw1.io/godoc/cmd/godoc-{cli,mcp}@latest
```

The binaries will be placed in `$GOBIN` (or `$GOPATH/bin` if unset).

For library, see [library](#library) section.

## CLI

`godoc-cli` renders Go documentation with [Glamour](https://github.com/charmbracelet/glamour) for syntax-aware Markdown output. It understands modules, respects `GOOS`/`GOARCH`, and can emit JSON when you need structured data.

> The binary is called `godoc-cli` to sidestep naming clashes with the `golang.org/x/tools/cmd/godoc` tool.

### Usage

```text
godoc-cli [options]
godoc-cli [options] <pkg>
godoc-cli [options] <sym>[.<methodOrField>]
godoc-cli [options] [<pkg>.]<sym>[.<methodOrField>]
godoc-cli [options] [<pkg>.][<sym>.]<methodOrField>
godoc-cli [options] <pkg> <sym>[.<methodOrField>]
```

**Options**

| Flag | Description |
| --- | --- |
| `-goos string` | Target operating system (`linux`, `darwin`, `windows`, …). |
| `-goarch string` | Target architecture (`amd64`, `arm64`, …). |
| `-workdir string` | Working directory for resolving relative import paths (default: current dir). |
| `-version string` | Module version to fetch (e.g., `v1.2.3`, `latest`). |
| `-style string` | Glamour theme: `auto` (default), `dark`, `light`, `notty`. |
| `-pager` | Render output inside an interactive pager UI (press `?` for controls, `c` to copy). |
| `-json` | Emit raw JSON instead of rendered Markdown. |
| `-help` | Print the usage guide. |

`godoc` detects terminal width and theme when `-style=auto`. When stdout isn’t a TTY (e.g., piping to a file), it falls back to a minimal renderer so you can feed the output into other tools.

> [!TIP]
> * Use `-style=notty` in environments without ANSI color support.
> * Use `-pager` to explore large packages.
>   * With `-pager`, press `?` to toggle inline help or `c` to copy the document to your clipboard.
> * The CLI shares caches and configuration with the library, so Go toolchain settings (`GOPROXY`, `GOCACHE`, etc.) apply automatically.

#### Examples

Use these commands to explore docs quickly.

**Docs for the current module**

```bash
godoc-cli
```

**Remote package browsing**

```bash
godoc-cli github.com/gorilla/mux
```

**Symbol lookup**

```bash
godoc-cli net/http Request.ParseForm
godoc-cli fmt.Println
godoc-cli net/http.StatusOK
```

**Version pinning**

```bash
godoc-cli -version v1.8.0 github.com/gorilla/mux
```

**JSON output**

```bash
godoc-cli -json fmt.Printf | jq
```

**Interactive pager**

```bash
godoc-cli -pager net/http Request
```

**Platform-specific APIs**

```bash
godoc-cli -goos windows -goarch arm64 runtime
```

## MCP

`godoc-mcp` wraps the same engine in an MCP server so assistants like Claude Desktop, Cursor, or any other AI apps can serve Go docs inside a conversation.

### Usage

Launches an MCP server over STDIO:

```bash
godoc-mcp
```

### Tool interface

The server registers one tool: `load`.

| Argument | Type | Required | Description |
| --- | --- | --- | --- |
| `import_path` | string | ✅ | Go import path (`fmt`, `net/http`, `github.com/user/repo`, `.`). |
| `selector` | string | ❌ | Symbol selector (`Printf`, `Request.ParseForm`, `StatusOK`, …). Empty loads the full package. |
| `version` | string | ❌ | Module version (`v1.2.3`, `latest`). Empty uses the default/installed version. |
| `goos` | string | ❌ | Target OS for cross-compilation (`linux`, `darwin`, `windows`, …). |
| `goarch` | string | ❌ | Target architecture (`amd64`, `arm64`, …). |
| `workdir` | string | ❌ | Directory used to resolve relative import paths (defaults to the host’s current working directory). |

Calls return the raw `godoc.Result` (either `PackageDoc` or `SymbolDoc`) plus the request metadata (`import_path`, `selector`, `version`). See the library section below for schema details.

#### Example MCP configuration

```json
{
  "mcpServers": {
    "memory": {
      "command": "/path/to/go/bin/godoc-mcp"
    }
  }
}
```

After saving, restart your MCP host.

## Library

[![Go Reference](https://pkg.go.dev/badge/go.dw1.io/godoc.svg)](https://pkg.go.dev/go.dw1.io/godoc)

The library powers both binaries and is available for embedding in servers, CLIs, or automation. It can load package docs, symbols, and methods from local modules or remote sources, with optional version pinning and cross-compilation.

### Installation

```bash
go get go.dw1.io/godoc
```

### Quick start

```go
package main

import (
    "context"
    "fmt"
    "time"

    "go.dw1.io/godoc"
)

func main() {
    g := godoc.New(
        godoc.WithGOOS("linux"),
        godoc.WithGOARCH("amd64"),
        godoc.WithContext(context.Background()),
    )

    result, err := g.Load("fmt", "Printf", "")
    if err != nil {
        panic(err)
    }

    fmt.Println(result.Text())

    ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel()

    g = godoc.New(godoc.WithContext(ctx))
    _, _ = g.Load("github.com/gorilla/mux", "", "latest")
}
```

### Loading documentation

```go
Load(importPath, sel, version string) (Result, error)
```

- `importPath`: package import path (e.g., `fmt`, `net/http`, `github.com/user/repo`, `.`)
- `sel`: symbol selector (`Printf`, `Request.ParseForm`, …); leave empty for the whole package
- `version`: module version (`v1.2.3`, `latest`); leave empty for the default version

The returned `Result` implements `Text()`, `HTML()`, and `MarshalJSON()`.

### Result types

```go
type PackageDoc struct {
    ImportPath string     `json:"import_path"`
    Name       string     `json:"name"`
    Synopsis   string     `json:"synopsis"`
    DocText    string     `json:"doc"`
    Consts     []ValueDoc `json:"consts"`
    Vars       []ValueDoc `json:"vars"`
    Funcs      []FuncDoc  `json:"funcs"`
    Types      []TypeDoc  `json:"types"`
}

type SymbolDoc struct {
    ImportPath string    `json:"import_path"`
    Package    string    `json:"package"`
    Kind       string    `json:"kind"`
    Name       string    `json:"name"`
    Receiver   string    `json:"receiver"`
    Args       []ArgInfo `json:"args"`
    DocText    string    `json:"doc"`
}
```

Use the higher-level convenience methods or marshal to JSON to feed docs into other systems.

### Status

> [!CAUTION]
> `godoc` has NOT reached 1.0 yet. Therefore, this library does not offer a stable API; use at your own risk.

There are no guarantees of stability for the APIs in this library.

## Contributions

Go ahead.

## License

The entire project (CLI, MCP server, and library) is licensed under the MIT License. See [LICENSE](/LICENSE).