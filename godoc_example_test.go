package godoc_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.dw1.io/godoc"
)

// ExampleNew demonstrates creating a new Godoc instance.
func ExampleNew() {
	g := godoc.New()
	_ = g // Use the variable to avoid unused error
	fmt.Println("Godoc instance created successfully")
	// Output: Godoc instance created successfully
}

// ExampleNew_withOptions demonstrates creating a Godoc instance with options.
func ExampleNew_withOptions() {
	g := godoc.New(
		godoc.WithGOOS("linux"),
		godoc.WithGOARCH("amd64"),
		godoc.WithWorkdir("/tmp"),
	)
	_ = g // Use the variable to avoid unused error
	fmt.Println("Godoc instance created with options")
	// Output: Godoc instance created with options
}

// ExampleGodoc_Load_package demonstrates loading documentation for an entire package.
func ExampleGodoc_Load_package() {
	g := godoc.New()
	result, err := g.Load("fmt", "", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded package documentation successfully\n")
	// Output: Loaded package documentation successfully
}

// ExampleGodoc_Load_function demonstrates loading documentation for a specific function.
func ExampleGodoc_Load_function() {
	g := godoc.New()
	result, err := g.Load("fmt", "Printf", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded function documentation successfully\n")
	// Output: Loaded function documentation successfully
}

// ExampleGodoc_Load_type demonstrates loading documentation for a specific type.
func ExampleGodoc_Load_type() {
	g := godoc.New()
	result, err := g.Load("fmt", "Stringer", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded type documentation successfully\n")
	// Output: Loaded type documentation successfully
}

// ExampleGodoc_Load_method demonstrates loading documentation for a specific method.
func ExampleGodoc_Load_method() {
	g := godoc.New()
	result, err := g.Load("net/http", "Request.ParseForm", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded method documentation successfully\n")
	// Output: Loaded method documentation successfully
}

// ExampleGodoc_Load_constant demonstrates loading documentation for a specific constant.
func ExampleGodoc_Load_constant() {
	g := godoc.New()
	result, err := g.Load("net/http", "StatusOK", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded constant documentation successfully\n")
	// Output: Loaded constant documentation successfully
}

// ExampleGodoc_Load_variable demonstrates loading documentation for a specific variable.
func ExampleGodoc_Load_variable() {
	g := godoc.New()
	result, err := g.Load("os", "Args", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded variable documentation successfully\n")
	// Output: Loaded variable documentation successfully
}

// ExampleGodoc_Load_withVersion demonstrates loading documentation with a specific version.
func ExampleGodoc_Load_withVersion() {
	g := godoc.New()
	result, err := g.Load("fmt", "", "latest")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded documentation with version successfully\n")
	// Output: Loaded documentation with version successfully
}

// ExampleGodoc_Load_remotePackage demonstrates loading documentation for a remote package.
func ExampleGodoc_Load_remotePackage() {
	g := godoc.New()
	result, err := g.Load("github.com/stretchr/testify/assert", "", "")
	if err != nil {
		// Skip if remote package is not available
		fmt.Println("Remote package not available")
		return
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded remote package documentation successfully\n")
	// Output: Loaded remote package documentation successfully
}

// ExampleGodoc_Load_remoteSymbol demonstrates loading documentation for a symbol from a remote package.
func ExampleGodoc_Load_remoteSymbol() {
	g := godoc.New()
	result, err := g.Load("github.com/stretchr/testify/assert", "Equal", "")
	if err != nil {
		// Skip if remote package is not available
		fmt.Println("Remote symbol not available")
		return
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded remote symbol documentation successfully\n")
	// Output: Loaded remote symbol documentation successfully
}

// ExampleGodoc_Load_jsonOutput demonstrates outputting documentation as JSON.
func ExampleGodoc_Load_jsonOutput() {
	g := godoc.New()
	result, err := g.Load("fmt", "Printf", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	// The result implements json.Marshaler
	_, err = result.MarshalJSON()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("JSON marshaling successful\n")
	// Output: JSON marshaling successful
}

// ExampleGodoc_Load_textOutput demonstrates getting plain text documentation.
func ExampleGodoc_Load_textOutput() {
	g := godoc.New()
	result, err := g.Load("fmt", "Printf", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	text := result.Text()
	fmt.Printf("Text output successful, length: %d\n", len(text))
	// Output: Text output successful, length: 150
}

// ExampleGodoc_Load_htmlOutput demonstrates getting HTML documentation.
func ExampleGodoc_Load_htmlOutput() {
	g := godoc.New()
	result, err := g.Load("fmt", "Printf", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	html := result.HTML()
	fmt.Printf("HTML output successful, length: %d\n", len(html))
	// Output: HTML output successful, length: 153
}

// ExampleGodoc_Load_currentPackage demonstrates loading documentation for the current package.
func ExampleGodoc_Load_currentPackage() {
	g := godoc.New()
	result, err := g.Load(".", "", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded current package documentation successfully\n")
	// Output: Loaded current package documentation successfully
}

// ExampleGodoc_Load_complexSymbol demonstrates loading documentation for complex symbol names.
func ExampleGodoc_Load_complexSymbol() {
	g := godoc.New()
	result, err := g.Load("net/http", "Client.Do", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded complex symbol documentation successfully\n")
	// Output: Loaded complex symbol documentation successfully
}

// ExampleGodoc_Load_withGOOSGOARCH demonstrates loading documentation with specific GOOS/GOARCH.
func ExampleGodoc_Load_withGOOSGOARCH() {
	g := godoc.New(godoc.WithGOOS("linux"), godoc.WithGOARCH("amd64"))
	result, err := g.Load("runtime", "", "")
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("Result is nil")
		return
	}

	fmt.Printf("Loaded platform-specific documentation successfully\n")
	// Output: Loaded platform-specific documentation successfully
}

// ExampleGodoc_Load_errorHandling demonstrates error handling when loading documentation.
func ExampleGodoc_Load_errorHandling() {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	g := godoc.New(
		godoc.WithContext(ctx),
	)
	_, err := g.Load("nonexistent/package", "", "")
	if err != nil {
		fmt.Println("Error handled correctly:", err != nil)
	}
	// Output: Error handled correctly: true
}

// ExampleGodoc_Load_invalidSelector demonstrates handling invalid selectors.
func ExampleGodoc_Load_invalidSelector() {
	g := godoc.New()
	_, err := g.Load("fmt", "NonExistentFunction", "")
	if err != nil {
		fmt.Println("Invalid selector error handled:", err != nil)
	}
	// Output: Invalid selector error handled: true
}

// ExampleGodoc_Load_emptyImportPath demonstrates handling empty import paths.
func ExampleGodoc_Load_emptyImportPath() {
	g := godoc.New()
	_, err := g.Load("", "", "")
	if err != nil {
		fmt.Println("Empty import path error handled:", err != nil)
	}
	// Output: Empty import path error handled: true
}
