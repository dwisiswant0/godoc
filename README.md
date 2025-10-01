# godoc

[![Go Reference](https://pkg.go.dev/badge/go.dw1.io/godoc.svg)](https://pkg.go.dev/go.dw1.io/godoc)

Package `godoc` provides functionality for extracting and outputting Go package documentation in multiple formats (JSON, text, HTML). It supports both local and remote packages with options for specific symbols, module versions, and cross-platform builds.

## Installation

```bash
go get go.dw1.io/godoc
```

## Quick Start

```go
package main

import (
    "fmt"

    "go.dw1.io/godoc"
)

func main() {
    g := godoc.New()
    
    // Load package documentation
    result, err := g.Load("fmt", "", "")
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Package: %s\n", result.Text())
}
```

## API Usage

### Creating a Godoc Instance

```go
// Basic instance
g := godoc.New()

// With configuration options
g := godoc.New(
    godoc.WithGOOS("linux"),
    godoc.WithGOARCH("amd64"),
    godoc.WithWorkdir("/tmp"),
)

// With a custom context (for cancellation/timeouts, etc.)
ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
defer cancel()

g := godoc.New(godoc.WithContext(ctx))
```

### Loading Documentation

The `Load` method signature:

```go
Load(importPath, symbol, version string) (Result, error)
```

**Parameters:**

- `importPath`: Package import path (e.g., "fmt", "net/http", "github.com/user/repo")
- `symbol`: Specific symbol name (empty for entire package)
- `version`: Module version (empty for latest/default)

**Returns:**

- `Result`: Documentation result implementing `Text()` and `HTML()` methods
- `error`: Any error that occurred

### Examples

#### Package Documentation

```go
g := godoc.New()

// Standard library package
result, err := g.Load("fmt", "", "")
if err != nil {
    panic(err)
}

// Remote package
result, err := g.Load("github.com/gorilla/mux", "", "")
if err != nil {
    panic(err)
}

// Current package
result, err := g.Load(".", "", "")
if err != nil {
    panic(err)
}
```

#### Symbol-Specific Documentation

```go
g := godoc.New()

// Function documentation
result, err := g.Load("fmt", "Printf", "")

// Type documentation
result, err := g.Load("fmt", "Stringer", "")

// Method documentation
result, err := g.Load("net/http", "Request.ParseForm", "")

// Constant documentation
result, err := g.Load("net/http", "StatusOK", "")

// Variable documentation
result, err := g.Load("os", "Args", "")
```

#### Version-Specific Documentation

```go
g := godoc.New()

// Specific version
result, err := g.Load("github.com/gorilla/mux", "", "v1.8.0")

// Latest version
result, err := g.Load("github.com/gorilla/mux", "", "latest")
```

#### Cross-Platform Documentation

```go
g := godoc.New(
    godoc.WithGOOS("windows"),
    godoc.WithGOARCH("amd64"),
)

result, err := g.Load("runtime", "", "")
```

## Output Formats

Each `Result` from `Load` implements `Text()`, `HTML()`, and `MarshalJSON()` so you can format documentation however you need.

### JSON Serialization

```go
result, err := g.Load("fmt", "Printf", "")
if err != nil {
    panic(err)
}

// Marshal to JSON
data, err := result.MarshalJSON()
if err != nil {
    panic(err)
}

fmt.Printf("JSON: %s\n", data)
```

### Text Output

```go
result, err := g.Load("fmt", "Printf", "")
if err != nil {
    panic(err)
}

// Get plain text documentation
text := result.Text()
fmt.Printf("Documentation: %s\n", text)
```

### HTML Output

```go
result, err := g.Load("fmt", "Printf", "")
if err != nil {
    panic(err)
}

// Get HTML-formatted documentation
html := result.HTML()
fmt.Printf("HTML: %s\n", html)
```

## Result Types

The library returns different result types based on what was loaded:

### `PackageDoc`

Returned when loading an entire package (empty `symbol` parameter):

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
```

### `SymbolDoc`

Returned when loading a specific symbol:

```go
type SymbolDoc struct {
    ImportPath string    `json:"import_path"`
    Package    string    `json:"package"`
    Kind       string    `json:"kind"`       // "func", "type", "method", "const", "var"
    Name       string    `json:"name"`
    Receiver   string    `json:"receiver"`   // For methods only
    Args       []ArgInfo `json:"args"`       // For functions/methods only
    DocText    string    `json:"doc"`
}
```

## License

`godoc` is licensed under the MIT License. See the [LICENSE](LICENSE) file.