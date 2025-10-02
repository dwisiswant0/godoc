package godoc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.dw1.io/godoc"
)

type testGodoc struct {
	godoc.Godoc
}

func newTestGodoc(opts ...godoc.Option) testGodoc {
	return testGodoc{Godoc: godoc.New(opts...)}
}

func (g testGodoc) Load(importPath, symbol, version string) (godoc.Result, error) {
	return g.Godoc.Load(importPath, symbol, version)
}

func TestNew(t *testing.T) {
	g := godoc.New(godoc.WithGOOS("linux"), godoc.WithGOARCH("amd64"), godoc.WithWorkdir("/tmp"))
	// Fields are unexported, so we can't check them directly
	// But we can verify the instance was created
	_ = g
}

func TestNewDefaultDir(t *testing.T) {
	g := godoc.New()
	// Fields are unexported, so we can't check workdir directly
	// But we can verify the instance was created with default
	_ = g
}

func TestLoadPackage(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "", "")
	if err != nil {
		t.Fatalf("Failed to load fmt: %v", err)
	}
	pkgDoc, ok := result.(godoc.PackageDoc)
	if !ok {
		t.Errorf("Expected PackageDoc, got %T", result)
	}

	if pkgDoc.ImportPath != "fmt" {
		t.Errorf("Expected ImportPath fmt, got %s", pkgDoc.ImportPath)
	}
	if pkgDoc.Name != "fmt" {
		t.Errorf("Expected Name fmt, got %s", pkgDoc.Name)
	}
	if pkgDoc.DocText == "" {
		t.Errorf("Expected non-empty DocText")
	}
}

func TestLoadSymbolFunction(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "Printf", "")
	if err != nil {
		t.Fatalf("Failed to load fmt.Printf: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	if symDoc.Kind != "func" {
		t.Errorf("Expected Kind func, got %s", symDoc.Kind)
	}
	if symDoc.Name != "Printf" {
		t.Errorf("Expected Name Printf, got %s", symDoc.Name)
	}
	if symDoc.Package != "fmt" {
		t.Errorf("Expected Package fmt, got %s", symDoc.Package)
	}
}

func TestLoadSymbolType(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "Stringer", "")
	if err != nil {
		t.Fatalf("Failed to load fmt.Stringer: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	if symDoc.Kind != "type" {
		t.Errorf("Expected Kind type, got %s", symDoc.Kind)
	}
	if symDoc.Name != "Stringer" {
		t.Errorf("Expected Name Stringer, got %s", symDoc.Name)
	}
}

func TestLoadSymbolMethod(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("net/http", "Request.ParseForm", "")
	if err != nil {
		t.Fatalf("Failed to load net/http.Request.ParseForm: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	if symDoc.Kind != "method" {
		t.Errorf("Expected Kind method, got %s", symDoc.Kind)
	}
	if symDoc.Name != "ParseForm" {
		t.Errorf("Expected Name ParseForm, got %s", symDoc.Name)
	}
	if symDoc.Receiver != "Request" {
		t.Errorf("Expected Receiver Request, got %s", symDoc.Receiver)
	}
}

func TestLoadSymbolConst(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("net/http", "StatusOK", "")
	if err != nil {
		t.Fatalf("Failed to load net/http.StatusOK: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	if symDoc.Kind != "const" {
		t.Errorf("Expected Kind const, got %s", symDoc.Kind)
	}
	if symDoc.Name != "StatusOK" {
		t.Errorf("Expected Name StatusOK, got %s", symDoc.Name)
	}
}

func TestLoadInvalidPackage(t *testing.T) {
	g := godoc.New()
	_, err := g.Load("invalid/package", "", "")
	if err == nil {
		t.Errorf("Expected error for invalid package")
	}
}

func TestLoadNonExistentSymbol(t *testing.T) {
	g := godoc.New()
	_, err := g.Load("fmt", "NonExistent", "")
	if err == nil {
		t.Errorf("Expected error for non-existent symbol")
	}
}

func TestLoadEmptyImportPath(t *testing.T) {
	g := godoc.New()
	_, err := g.Load("", "", "")
	if err == nil {
		t.Errorf("Expected error for empty import path")
	}
}

func TestLoadSymbolWithDot(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("os", "File.Read", "")
	if err != nil {
		t.Fatalf("Failed to load os.File.Read: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	if symDoc.Kind != "method" {
		t.Errorf("Expected Kind method, got %s", symDoc.Kind)
	}
	if symDoc.Name != "Read" {
		t.Errorf("Expected Name Read, got %s", symDoc.Name)
	}
	if symDoc.Receiver != "File" {
		t.Errorf("Expected Receiver File, got %s", symDoc.Receiver)
	}
}

func TestStructFieldsIncluded(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("net/http", "", "")
	if err != nil {
		t.Fatalf("Failed to load net/http: %v", err)
	}

	pkgDoc, ok := result.(godoc.PackageDoc)
	if !ok {
		t.Fatalf("Expected PackageDoc, got %T", result)
	}

	var requestDoc *godoc.TypeDoc
	for i := range pkgDoc.Types {
		if pkgDoc.Types[i].Name == "Request" {
			requestDoc = &pkgDoc.Types[i]
			break
		}
	}

	if requestDoc == nil {
		t.Fatalf("Expected Request type in net/http")
	}

	if len(requestDoc.Fields) == 0 {
		t.Fatalf("Expected struct fields for net/http.Request")
	}

	var methodField *godoc.FieldDoc
	for i := range requestDoc.Fields {
		if requestDoc.Fields[i].Name == "Method" {
			methodField = &requestDoc.Fields[i]
			break
		}
	}

	if methodField == nil {
		t.Fatalf("Expected Method field on net/http.Request")
	}

	if methodField.Type != "string" {
		t.Fatalf("Expected Method field type string, got %s", methodField.Type)
	}
}

func TestExtractArgs(t *testing.T) {
	// This is internal, but we can test via Load
	g := newTestGodoc()
	result, err := g.Load("fmt", "Sprintf", "")
	if err != nil {
		t.Fatalf("Failed to load fmt.Sprintf: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	if len(symDoc.Args) < 1 {
		t.Errorf("Expected at least one arg for Sprintf")
	}
	// Check first arg is format string
	if symDoc.Args[0].Type != "string" {
		t.Errorf("Expected first arg type string, got %s", symDoc.Args[0].Type)
	}
}

func TestPackageDocJSON(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "", "")
	if err != nil {
		t.Fatalf("Failed to load fmt: %v", err)
	}
	if _, ok := result.(godoc.PackageDoc); !ok {
		t.Errorf("Expected PackageDoc, got %T", result)
	}
	data, err := result.MarshalJSON()
	if err != nil {
		t.Errorf("Failed to marshal PackageDoc via Result.MarshalJSON: %v", err)
	}
	if len(data) == 0 {
		t.Errorf("Expected non-empty JSON")
	}
}

func TestSymbolDocJSON(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "Printf", "")
	if err != nil {
		t.Fatalf("Failed to load fmt.Printf: %v", err)
	}
	if _, ok := result.(godoc.SymbolDoc); !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	data, err := result.MarshalJSON()
	if err != nil {
		t.Errorf("Failed to marshal SymbolDoc via Result.MarshalJSON: %v", err)
	}
	if len(data) == 0 {
		t.Errorf("Expected non-empty JSON")
	}
}

func TestTextAndHTML(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "", "")
	if err != nil {
		t.Fatalf("Failed to load fmt: %v", err)
	}
	text := result.Text()
	html := result.HTML()
	if text == "" {
		t.Errorf("Expected non-empty Text")
	}
	if html == "" {
		t.Errorf("Expected non-empty HTML")
	}
}

func TestLoadWithVersion(t *testing.T) {
	g := newTestGodoc()
	// This might require a remote package, but for test, use local
	result, err := g.Load("fmt", "", "v1.21.0")
	if err != nil {
		t.Fatalf("Failed to load fmt with version: %v", err)
	}
	// Just check it loads
	_ = result
}

func TestLoadRemotePackage(t *testing.T) {
	g := newTestGodoc()
	// Use a small remote package
	result, err := g.Load("github.com/stretchr/testify/assert", "", "")
	if err != nil {
		t.Skipf("Skipping remote test: %v", err)
	}
	pkgDoc, ok := result.(godoc.PackageDoc)
	if !ok {
		t.Errorf("Expected PackageDoc, got %T", result)
	}
	if !strings.Contains(pkgDoc.ImportPath, "testify") {
		t.Errorf("Expected ImportPath to contain testify, got %s", pkgDoc.ImportPath)
	}
}

func TestLoadSymbolFromRemote(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("github.com/stretchr/testify/assert", "Equal", "")
	if err != nil {
		t.Skipf("Skipping remote test: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	if symDoc.Name != "Equal" {
		t.Errorf("Expected Name Equal, got %s", symDoc.Name)
	}
}

func TestValidateInputs(t *testing.T) {
	// Internal function, test via Load
	g := godoc.New()
	_, err := g.Load("", "symbol", "")
	if err == nil {
		t.Errorf("Expected error for empty import path")
	}
}

func TestLoadCurrentPackage(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load(".", "", "")
	if err != nil {
		t.Fatalf("Failed to load current package: %v", err)
	}
	pkgDoc, ok := result.(godoc.PackageDoc)
	if !ok {
		t.Errorf("Expected PackageDoc, got %T", result)
	}
	if pkgDoc.Name == "" {
		t.Errorf("Expected non-empty package name")
	}
}

func TestCacheFunctionality(t *testing.T) {
	g := newTestGodoc()

	// First load
	startFirst := time.Now()
	result1, err1 := g.Load("fmt", "", "")
	firstDuration := time.Since(startFirst)
	if err1 != nil {
		t.Fatalf("Failed first load: %v", err1)
	}

	// Second load (should use cache)
	startSecond := time.Now()
	result2, err2 := g.Load("fmt", "", "")
	secondDuration := time.Since(startSecond)
	if err2 != nil {
		t.Fatalf("Failed second load: %v", err2)
	}

	// Results should be identical
	pkgDoc1, ok1 := result1.(godoc.PackageDoc)
	pkgDoc2, ok2 := result2.(godoc.PackageDoc)
	if !ok1 || !ok2 {
		t.Errorf("Expected PackageDoc for both results")
	}
	if pkgDoc1.ImportPath != pkgDoc2.ImportPath {
		t.Errorf("Cached result differs: %s vs %s", pkgDoc1.ImportPath, pkgDoc2.ImportPath)
	}

	delta := firstDuration - secondDuration
	t.Logf("cache load timings: cold=%s warm=%s delta=%s", firstDuration, secondDuration, delta)
}

func TestCacheWithVersion(t *testing.T) {
	g := newTestGodoc()

	// Load with version
	startFirst := time.Now()
	result1, err1 := g.Load("fmt", "", "latest")
	firstDuration := time.Since(startFirst)
	if err1 != nil {
		t.Fatalf("Failed load with version: %v", err1)
	}

	// Load again with same version (should use cache)
	startSecond := time.Now()
	result2, err2 := g.Load("fmt", "", "latest")
	secondDuration := time.Since(startSecond)
	if err2 != nil {
		t.Fatalf("Failed second load with version: %v", err2)
	}

	// Results should be identical
	pkgDoc1, ok1 := result1.(godoc.PackageDoc)
	pkgDoc2, ok2 := result2.(godoc.PackageDoc)
	if !ok1 || !ok2 {
		t.Errorf("Expected PackageDoc for both results")
	}
	if pkgDoc1.DocText != pkgDoc2.DocText {
		t.Errorf("Cached result with version differs")
	}

	delta := firstDuration - secondDuration
	t.Logf("cache load timings (versioned): cold=%s warm=%s delta=%s", firstDuration, secondDuration, delta)
}

func TestStdLibWithGoVersion(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("os", "", "")
	if err != nil {
		t.Fatalf("Failed to load os: %v", err)
	}
	pkgDoc, ok := result.(godoc.PackageDoc)
	if !ok {
		t.Errorf("Expected PackageDoc, got %T", result)
	}
	if pkgDoc.ImportPath != "os" {
		t.Errorf("Expected ImportPath os, got %s", pkgDoc.ImportPath)
	}
	// Verify it has documentation
	if pkgDoc.DocText == "" {
		t.Errorf("Expected non-empty DocText for std lib")
	}
}

func TestLoadWithGOOSGOARCH(t *testing.T) {
	g := newTestGodoc(godoc.WithGOOS("linux"), godoc.WithGOARCH("amd64"))
	result, err := g.Load("runtime", "", "")
	if err != nil {
		t.Fatalf("Failed to load runtime with GOOS/GOARCH: %v", err)
	}
	pkgDoc, ok := result.(godoc.PackageDoc)
	if !ok {
		t.Errorf("Expected PackageDoc, got %T", result)
	}
	if pkgDoc.Name != "runtime" {
		t.Errorf("Expected Name runtime, got %s", pkgDoc.Name)
	}
}

func TestLoadSymbolVar(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("os", "Args", "")
	if err != nil {
		t.Fatalf("Failed to load os.Args: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	if symDoc.Kind != "var" {
		t.Errorf("Expected Kind var, got %s", symDoc.Kind)
	}
	if symDoc.Name != "Args" {
		t.Errorf("Expected Name Args, got %s", symDoc.Name)
	}
}

func TestLoadComplexSymbol(t *testing.T) {
	g := newTestGodoc()
	// Test loading a method from a complex type
	result, err := g.Load("net/http", "Client.Do", "")
	if err != nil {
		t.Fatalf("Failed to load net/http.Client.Do: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	if symDoc.Kind != "method" {
		t.Errorf("Expected Kind method, got %s", symDoc.Kind)
	}
	if symDoc.Name != "Do" {
		t.Errorf("Expected Name Do, got %s", symDoc.Name)
	}
	if symDoc.Receiver != "Client" {
		t.Errorf("Expected Receiver Client, got %s", symDoc.Receiver)
	}
}

func TestLoadInvalidSymbolFormat(t *testing.T) {
	g := godoc.New()
	_, err := g.Load("fmt", "Printf.", "")
	if err == nil {
		t.Errorf("Expected error for invalid symbol format")
	}
}

func TestLoadEmptySymbol(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "", "")
	if err != nil {
		t.Fatalf("Failed to load fmt with empty symbol: %v", err)
	}
	// Should return PackageDoc when symbol is empty
	_, ok := result.(godoc.PackageDoc)
	if !ok {
		t.Errorf("Expected PackageDoc for empty symbol, got %T", result)
	}
}

func TestCacheDifferentVersions(t *testing.T) {
	g := newTestGodoc()

	// Load with empty version
	result1, err1 := g.Load("fmt", "", "")
	if err1 != nil {
		t.Fatalf("Failed load with empty version: %v", err1)
	}

	// Load with specific version (if fmt supports it)
	result2, err2 := g.Load("fmt", "", "latest")
	if err2 != nil {
		t.Fatalf("Failed load with specific version: %v", err2)
	}

	// Results should be the same for std lib
	pkgDoc1, ok1 := result1.(godoc.PackageDoc)
	pkgDoc2, ok2 := result2.(godoc.PackageDoc)
	if !ok1 || !ok2 {
		t.Errorf("Expected PackageDoc for both results")
	}
	if pkgDoc1.ImportPath != pkgDoc2.ImportPath {
		t.Errorf("Results should be the same for std lib: %s vs %s", pkgDoc1.ImportPath, pkgDoc2.ImportPath)
	}
}

func TestRemotePackageCaching(t *testing.T) {
	g := newTestGodoc()

	// First load of remote package
	result1, err1 := g.Load("github.com/stretchr/testify/assert", "", "")
	if err1 != nil {
		t.Skipf("Skipping remote caching test: %v", err1)
	}

	// Second load (should use cache)
	result2, err2 := g.Load("github.com/stretchr/testify/assert", "", "")
	if err2 != nil {
		t.Fatalf("Failed second remote load: %v", err2)
	}

	// Results should be identical
	pkgDoc1, ok1 := result1.(godoc.PackageDoc)
	pkgDoc2, ok2 := result2.(godoc.PackageDoc)
	if !ok1 || !ok2 {
		t.Errorf("Expected PackageDoc for both results")
	}
	if pkgDoc1.Name != pkgDoc2.Name {
		t.Errorf("Cached remote result differs: %s vs %s", pkgDoc1.Name, pkgDoc2.Name)
	}
}

func TestLoadWithCustomDir(t *testing.T) {
	g := newTestGodoc(godoc.WithWorkdir("."))
	result, err := g.Load("fmt", "", "")
	if err != nil {
		t.Fatalf("Failed to load fmt with custom dir: %v", err)
	}
	pkgDoc, ok := result.(godoc.PackageDoc)
	if !ok {
		t.Errorf("Expected PackageDoc, got %T", result)
	}
	if pkgDoc.ImportPath != "fmt" {
		t.Errorf("Expected ImportPath fmt, got %s", pkgDoc.ImportPath)
	}
}

func TestSymbolDocArgsVariadic(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "Printf", "")
	if err != nil {
		t.Fatalf("Failed to load fmt.Printf: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	// Printf has variadic args
	if len(symDoc.Args) < 2 {
		t.Errorf("Expected at least 2 args for Printf")
	}
	// Check if any arg is variadic (starts with ...)
	hasVariadic := false
	for _, arg := range symDoc.Args {
		if strings.HasPrefix(arg.Type, "...") {
			hasVariadic = true
			break
		}
	}
	if !hasVariadic {
		t.Errorf("Expected variadic args for Printf")
	}
}

func TestSymbolDocGenericArgs(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("slices", "Insert", "")
	if err != nil {
		t.Fatalf("Failed to load slices.Insert: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Fatalf("Expected SymbolDoc, got %T", result)
	}

	if len(symDoc.Args) != 3 {
		t.Fatalf("Expected 3 args for slices.Insert, got %d", len(symDoc.Args))
	}

	if symDoc.Args[0].Type != "S" {
		t.Errorf("expected first argument type S, got %q", symDoc.Args[0].Type)
	}

	if symDoc.Args[1].Type != "int" {
		t.Errorf("expected second argument type int, got %q", symDoc.Args[1].Type)
	}

	if symDoc.Args[2].Type != "...E" {
		t.Errorf("expected third argument type ...E, got %q", symDoc.Args[2].Type)
	}
}

func TestPackageDocSynopsis(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "", "")
	if err != nil {
		t.Fatalf("Failed to load fmt: %v", err)
	}
	pkgDoc, ok := result.(godoc.PackageDoc)
	if !ok {
		t.Errorf("Expected PackageDoc, got %T", result)
	}
	if pkgDoc.Synopsis == "" {
		t.Errorf("Expected non-empty Synopsis")
	}
}

func TestLoadNonExistentRemotePackage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	g := godoc.New(
		godoc.WithContext(ctx),
	)
	_, err := g.Load("github.com/nonexistent/package/that/does/not/exist", "", "")
	if err == nil {
		t.Errorf("Expected error for non-existent remote package")
	}
}

func TestSymbolDocHTML(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "Printf", "")
	if err != nil {
		t.Fatalf("Failed to load fmt.Printf: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}

	// Test HTML generation
	html := symDoc.HTML()
	if html == "" {
		t.Errorf("Expected non-empty HTML")
	}

	// Test that HTML is cached (call again)
	html2 := symDoc.HTML()
	if html != html2 {
		t.Errorf("HTML should be cached and identical")
	}
}

func TestSymbolDocText(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "Printf", "")
	if err != nil {
		t.Fatalf("Failed to load fmt.Printf: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}

	// Test Text method
	text := symDoc.Text()
	if text == "" {
		t.Errorf("Expected non-empty text from SymbolDoc.Text()")
	}

	// Verify it matches the DocText field
	if text != symDoc.DocText {
		t.Errorf("Text() should return DocText field")
	}
}

func TestSymbolDocTextAndHTML(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("fmt", "Stringer", "")
	if err != nil {
		t.Fatalf("Failed to load fmt.Stringer: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}

	// Test both Text and HTML methods
	text := symDoc.Text()
	html := symDoc.HTML()

	if text == "" {
		t.Errorf("Expected non-empty text")
	}
	if html == "" {
		t.Errorf("Expected non-empty HTML")
	}

	// Test that HTML is generated and cached
	html2 := symDoc.HTML()
	if html != html2 {
		t.Errorf("HTML should be cached and identical on second call")
	}
}

func TestLocalModuleCaching(t *testing.T) {
	g := newTestGodoc()

	// Load current package multiple times to test local caching
	result1, err1 := g.Load(".", "", "")
	if err1 != nil {
		t.Fatalf("Failed first load of current package: %v", err1)
	}

	result2, err2 := g.Load(".", "", "")
	if err2 != nil {
		t.Fatalf("Failed second load of current package: %v", err2)
	}

	// Results should be identical
	pkgDoc1, ok1 := result1.(godoc.PackageDoc)
	pkgDoc2, ok2 := result2.(godoc.PackageDoc)
	if !ok1 || !ok2 {
		t.Errorf("Expected PackageDoc for both results")
	}
	if pkgDoc1.Name != pkgDoc2.Name {
		t.Errorf("Cached local result differs: %s vs %s", pkgDoc1.Name, pkgDoc2.Name)
	}
}

func TestLoadWithDifferentGOOSGOARCH(t *testing.T) {
	g := newTestGodoc(godoc.WithGOOS("windows"), godoc.WithGOARCH("amd64"))
	result, err := g.Load("runtime", "", "")
	if err != nil {
		t.Fatalf("Failed to load runtime with different GOOS/GOARCH: %v", err)
	}
	pkgDoc, ok := result.(godoc.PackageDoc)
	if !ok {
		t.Errorf("Expected PackageDoc, got %T", result)
	}
	if pkgDoc.Name != "runtime" {
		t.Errorf("Expected Name runtime, got %s", pkgDoc.Name)
	}
}

func TestCacheConcurrency(t *testing.T) {
	g := newTestGodoc()

	// Test concurrent access to cache
	done := make(chan bool, 10)

	for range 10 {
		go func() {
			result, err := g.Load("fmt", "", "")
			if err != nil {
				t.Errorf("Concurrent load failed: %v", err)
				done <- false
				return
			}
			_ = result
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		if !<-done {
			t.Errorf("One of the concurrent loads failed")
		}
	}
}

func TestLoadSymbolFromComplexPackage(t *testing.T) {
	g := newTestGodoc()
	// Test loading from a package with many symbols
	result, err := g.Load("net/http", "Server", "")
	if err != nil {
		t.Fatalf("Failed to load net/http.Server: %v", err)
	}
	symDoc, ok := result.(godoc.SymbolDoc)
	if !ok {
		t.Errorf("Expected SymbolDoc, got %T", result)
	}
	if symDoc.Kind != "type" {
		t.Errorf("Expected Kind type, got %s", symDoc.Kind)
	}
	if symDoc.Name != "Server" {
		t.Errorf("Expected Name Server, got %s", symDoc.Name)
	}
}

func TestPackageDocTypesMethods(t *testing.T) {
	g := newTestGodoc()
	result, err := g.Load("net/http", "", "")
	if err != nil {
		t.Fatalf("Failed to load net/http: %v", err)
	}
	pkgDoc, ok := result.(godoc.PackageDoc)
	if !ok {
		t.Errorf("Expected PackageDoc, got %T", result)
	}

	// Check that we have types with methods
	hasTypeWithMethods := false
	for _, typ := range pkgDoc.Types {
		if len(typ.Methods) > 0 {
			hasTypeWithMethods = true
			break
		}
	}
	if !hasTypeWithMethods {
		t.Errorf("Expected at least one type with methods")
	}
}

func TestLoadWithWhitespaceVersion(t *testing.T) {
	g := newTestGodoc()
	// Test version with whitespace
	result, err := g.Load("fmt", "", "  v1.21.0  ")
	if err != nil {
		t.Fatalf("Failed to load with whitespace version: %v", err)
	}
	_ = result
}

func TestLoadImportPathParentTraversal(t *testing.T) {
	g := godoc.New()
	_, err := g.Load("../fmt", "", "")
	if err == nil {
		t.Fatalf("expected error for parent traversal import path")
	}
	if !errors.Is(err, godoc.ErrInvalidImportPath) {
		t.Fatalf("expected ErrInvalidImportPath, got %v", err)
	}
}

func TestLoadImportPathAbsolute(t *testing.T) {
	g := godoc.New()
	_, err := g.Load("/fmt", "", "")
	if err == nil {
		t.Fatalf("expected error for absolute import path")
	}
	if !errors.Is(err, godoc.ErrInvalidImportPath) {
		t.Fatalf("expected ErrInvalidImportPath, got %v", err)
	}
}
