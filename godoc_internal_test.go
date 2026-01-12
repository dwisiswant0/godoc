package godoc

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/doc/comment"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestSymbolDocHTMLLazyGeneration(t *testing.T) {
	parser := new(comment.Parser)
	doc := parser.Parse("Symbol documentation.")

	sym := SymbolDoc{docParsed: doc}
	html := sym.HTML()
	if html == "" {
		t.Fatalf("expected HTML output from parsed doc")
	}
}

func TestGodocContextDefaults(t *testing.T) {
	t.Run("nil option resets to background", func(t *testing.T) {
		g := New(Option(func(g *Godoc) { g.ctx = nil }))
		if g.context() != context.Background() {
			t.Fatalf("expected context.Background for nil option context")
		}
	})

	t.Run("WithContext nil uses background", func(t *testing.T) {
		g := New(WithContext(nil)) //nolint:staticcheck // verify nil context defaults to background
		if g.context() != context.Background() {
			t.Fatalf("expected context.Background for nil WithContext")
		}
	})

	var nilDoc *Godoc
	if nilDoc.context() != context.Background() {
		t.Fatalf("expected context.Background for nil receiver")
	}
}

func TestGodocLoadFillsImportPathFromCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	resetCacheGlobals()
	t.Cleanup(resetCacheGlobals)

	cache, err := getCache()
	if err != nil {
		t.Fatalf("unexpected cache error: %v", err)
	}

	entry := cacheEntry{
		Symbol: &SymbolDoc{
			Package: "fmt",
			Kind:    "func",
			Name:    "Printf",
		},
		cacheMetadata: cacheMetadata{GoVersion: runtime.Version()},
	}

	key := getCacheKey("fmt", getPkgVersion("fmt", ""), "Printf")
	cache.Set(key, entry)

	g := New()
	res, err := g.Load("fmt", "Printf", "")
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	sym, ok := res.(SymbolDoc)
	if !ok {
		t.Fatalf("expected SymbolDoc, got %T", res)
	}

	if sym.Package != "fmt" {
		t.Fatalf("expected package metadata preserved, got %q", sym.Package)
	}
}

func TestPersistentCacheHitSkipsLoad(t *testing.T) {
	t.Helper()

	resetCacheGlobals()
	t.Cleanup(resetCacheGlobals)

	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	pkgKey := getCacheKey("fmt", getPkgVersion("fmt", ""), "")

	g := New()
	if _, err := g.Load("fmt", "", ""); err != nil {
		t.Fatalf("initial load failed: %v", err)
	}

	cache1, err := getCache()
	if err != nil {
		t.Fatalf("failed to access cache after initial load: %v", err)
	}

	entry1, ok := getValidCacheEntry(cache1, pkgKey)
	if !ok || entry1.Package == nil {
		t.Fatalf("expected entry after initial load")
	}

	if entry1.GoVersion == "" {
		t.Fatalf("expected go version recorded after initial load, got empty")
	}

	resetCacheGlobals()

	g2 := New()
	called := false
	cache, err := getCache()
	if err != nil {
		t.Fatalf("failed to reload cache: %v", err)
	}

	entry, ok := getValidCacheEntry(cache, pkgKey)
	if !ok || entry.Package == nil {
		t.Fatalf("expected cache entry for fmt, found: %v", entry.Package)
	}

	g2.loadPkg = func(importPath, dir string, needTypes bool) (*doc.Package, *token.FileSet, *types.Info, *packageAST, string, *packages.Module, string, error) {
		called = true
		return nil, nil, nil, nil, "", nil, "", fmt.Errorf("loadPkg should not be called when cache hit")
	}

	res, err := g2.Load("fmt", "", "")
	if err != nil {
		t.Fatalf("expected cached load, got error: %v", err)
	}

	if called {
		t.Fatalf("expected cache hit to avoid invoking loadPkg")
	}

	pkgDoc, ok := res.(PackageDoc)
	if !ok || strings.TrimSpace(pkgDoc.Name) == "" {
		t.Fatalf("expected valid package doc from cache, got %#v", res)
	}
}

func TestGetOrLoadPkgPropagatesSetCacheError(t *testing.T) {
	root := t.TempDir()
	cacheHome := filepath.Join(root, "cachehome")
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	resetCacheGlobals()
	t.Cleanup(resetCacheGlobals)

	if _, err := getCache(); err != nil {
		t.Fatalf("unexpected cache init error: %v", err)
	}

	roDir := filepath.Join(root, "readonly")
	if err := os.Mkdir(roDir, 0o555); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	cachePersistent = true
	cacheFilePath = filepath.Join(roDir, "cache.gob")

	g := New()
	if _, _, err := g.getOrLoadPkg("fmt", ""); err == nil {
		t.Fatalf("expected persistence error")
	} else if !errors.Is(err, fs.ErrPermission) && !os.IsPermission(err) {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestGetOrLoadSymbolBuildDocError(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	resetCacheGlobals()
	t.Cleanup(resetCacheGlobals)

	g := New()
	if _, _, err := g.getOrLoadSymbol("invalid/package", "Foo", ""); err == nil {
		t.Fatalf("expected error for invalid package")
	}
}

func TestGetOrLoadSymbolSetCacheError(t *testing.T) {
	root := t.TempDir()
	cacheHome := filepath.Join(root, "cachehome")
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	resetCacheGlobals()
	t.Cleanup(resetCacheGlobals)

	if _, err := getCache(); err != nil {
		t.Fatalf("unexpected cache init error: %v", err)
	}

	roDir := filepath.Join(root, "readonly")
	if err := os.Mkdir(roDir, 0o555); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	cachePersistent = true
	cacheFilePath = filepath.Join(roDir, "cache.gob")

	g := New()
	if _, _, err := g.getOrLoadSymbol("fmt", "Println", ""); err == nil {
		t.Fatalf("expected persistence error")
	} else if !errors.Is(err, fs.ErrPermission) && !os.IsPermission(err) {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestGetOrLoadSymbolNotFound(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	resetCacheGlobals()
	t.Cleanup(resetCacheGlobals)

	g := New()
	if _, _, err := g.getOrLoadSymbol("fmt", "NotAThing", ""); err == nil {
		t.Fatalf("expected error for missing symbol")
	}
}

func TestGodocGetOrLoadPkgCacheError(t *testing.T) {
	resetCacheGlobals()
	root := t.TempDir()
	cacheHome := filepath.Join(root, "cachehome")
	if err := os.WriteFile(cacheHome, []byte(""), 0o644); err != nil {
		t.Fatalf("failed to create cache home file: %v", err)
	}
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	t.Cleanup(resetCacheGlobals)

	g := &Godoc{}
	if _, _, err := g.getOrLoadPkg("fmt", ""); err == nil {
		t.Fatalf("expected error when cache initialization fails")
	}
}

func TestLoadDocPkgAdjustsDirForStdlib(t *testing.T) {
	g := &Godoc{}
	dpkg, _, _, _, _, _, dir, err := g.loadDocPkg("fmt", ".", false)
	if err != nil {
		t.Fatalf("unexpected error loading stdlib: %v", err)
	}

	if dpkg == nil {
		t.Fatalf("expected package documentation")
	}

	if dir != "" {
		t.Fatalf("expected empty dir when loading from GOROOT, got %q", dir)
	}
}

func TestLoadDocPkgInvalidImportPath(t *testing.T) {
	g := &Godoc{}
	if _, _, _, _, _, _, _, err := g.loadDocPkg("example.com/this/does/not/exist", "", false); err == nil {
		t.Fatalf("expected error for invalid package load")
	}
}

func TestFieldAndExprHelpers(t *testing.T) {
	fset := token.NewFileSet()
	field := &ast.Field{Type: ast.NewIdent("int")}
	if got := fieldTypeString(nil, nil, nil); got != "" {
		t.Fatalf("expected empty string for nil field, got %q", got)
	}

	if got := fieldTypeString(field, nil, fset); got != "int" {
		t.Fatalf("expected int type, got %q", got)
	}

	if got := embeddedFieldName(field, fset, ""); got != "int" {
		t.Fatalf("expected embedded field name from expr, got %q", got)
	}

	if got := exprString(nil, fset); got != "" {
		t.Fatalf("expected empty string for nil expression")
	}

	if got := exprString(ast.NewIdent("string"), fset); got != "string" {
		t.Fatalf("expected string literal, got %q", got)
	}
}

func TestExtractArgsFallbackPaths(t *testing.T) {
	fset := token.NewFileSet()
	decl := &ast.FuncDecl{
		Type: &ast.FuncType{
			Params: &ast.FieldList{List: []*ast.Field{
				{Type: ast.NewIdent("int")},
				{
					Names: []*ast.Ident{{Name: "name"}},
					Type:  &ast.Ellipsis{Elt: ast.NewIdent("string")},
				},
			}},
		},
	}

	args := extractArgs(decl, fset, nil)
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}

	if args[0].Name != "" || args[0].Type != "int" {
		t.Fatalf("expected first arg to be unnamed int, got %+v", args[0])
	}

	if args[1].Name != "name" || args[1].Type != "...string" {
		t.Fatalf("expected variadic string arg, got %+v", args[1])
	}
}

func TestExtractArgsWithTypesInfo(t *testing.T) {
	fset := token.NewFileSet()
	ell := &ast.Ellipsis{Elt: ast.NewIdent("string")}
	decl := &ast.FuncDecl{
		Type: &ast.FuncType{
			Params: &ast.FieldList{List: []*ast.Field{
				{Type: ast.NewIdent("int")},
				{Names: []*ast.Ident{{Name: "vals"}}, Type: ell},
			}},
		},
	}

	typesInfo := &types.Info{Types: map[ast.Expr]types.TypeAndValue{
		decl.Type.Params.List[0].Type: {Type: types.Typ[types.Int]},
		ell.Elt:                       {Type: types.Typ[types.String]},
	}}

	args := extractArgs(decl, fset, typesInfo)
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}

	if args[0].Name != "" || args[0].Type != "int" {
		t.Fatalf("expected unnamed int arg, got %+v", args[0])
	}

	if args[1].Name != "vals" || args[1].Type != "...string" {
		t.Fatalf("expected variadic string arg, got %+v", args[1])
	}
}

func TestExtractResultsFallbackPaths(t *testing.T) {
	fset := token.NewFileSet()
	decl := &ast.FuncDecl{
		Type: &ast.FuncType{
			Results: &ast.FieldList{List: []*ast.Field{
				{Type: ast.NewIdent("error")},
				{Names: []*ast.Ident{{Name: "count"}}, Type: ast.NewIdent("int")},
			}},
		},
	}

	results := extractResults(decl, fset, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Name != "" || results[0].Type != "error" {
		t.Fatalf("expected first result unnamed error, got %+v", results[0])
	}

	if results[1].Name != "count" || results[1].Type != "int" {
		t.Fatalf("expected second result named int, got %+v", results[1])
	}
}

func TestExtractResultsWithTypesInfo(t *testing.T) {
	fset := token.NewFileSet()
	decl := &ast.FuncDecl{
		Type: &ast.FuncType{
			Results: &ast.FieldList{List: []*ast.Field{
				{Type: ast.NewIdent("string")},
				{Names: []*ast.Ident{{Name: "err"}}, Type: ast.NewIdent("error")},
			}},
		},
	}

	typesInfo := &types.Info{Types: map[ast.Expr]types.TypeAndValue{
		decl.Type.Results.List[0].Type: {Type: types.Typ[types.String]},
		decl.Type.Results.List[1].Type: {Type: types.Universe.Lookup("error").Type()},
	}}

	results := extractResults(decl, fset, typesInfo)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Type != "string" {
		t.Fatalf("expected first result string, got %+v", results[0])
	}

	if results[1].Name != "err" || results[1].Type != "error" {
		t.Fatalf("expected named error result, got %+v", results[1])
	}
}

func TestArgsFromSignature(t *testing.T) {
	params := types.NewTuple(
		types.NewVar(token.NoPos, nil, "", types.Typ[types.Int]),
		types.NewVar(token.NoPos, nil, "", types.NewSlice(types.Typ[types.String])),
	)

	sig := types.NewSignatureType(nil, nil, nil, params, nil, true)
	args := argsFromSignature(sig, []string{"hint", ""})

	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}

	if args[0].Name != "hint" || args[0].Type != "int" {
		t.Fatalf("expected hinted name and int type, got %+v", args[0])
	}

	if args[1].Name != "" || args[1].Type != "...string" {
		t.Fatalf("expected variadic string with no name, got %+v", args[1])
	}
}

func TestArgsFromSignatureNonSliceVariadic(t *testing.T) {
	params := types.NewTuple(types.NewVar(token.NoPos, nil, "", types.Typ[types.String]))
	sig := types.NewSignatureType(nil, nil, nil, params, nil, true)

	args := argsFromSignature(sig, nil)
	if len(args) != 1 {
		t.Fatalf("expected single arg, got %d", len(args))
	}

	if args[0].Type != "...string" {
		t.Fatalf("expected variadic string fallback, got %q", args[0].Type)
	}
}

func TestResultsFromSignature(t *testing.T) {
	results := types.NewTuple(
		types.NewVar(token.NoPos, nil, "", types.Typ[types.Int]),
		types.NewVar(token.NoPos, nil, "err", types.Universe.Lookup("error").Type()),
	)

	sig := types.NewSignatureType(nil, nil, nil, nil, results, false)
	out := resultsFromSignature(sig, []string{"value", ""})

	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}

	if out[0].Name != "value" || out[0].Type != "int" {
		t.Fatalf("expected hinted int result, got %+v", out[0])
	}

	if out[1].Name != "err" || out[1].Type != "error" {
		t.Fatalf("expected named error result, got %+v", out[1])
	}
}

func TestGetVersionFromMod(t *testing.T) {
	tempDir := t.TempDir()
	if version := getVersionFromMod(tempDir, "example.com/foo"); version != "" {
		t.Fatalf("expected empty version when go.mod missing, got %q", version)
	}

	goMod := "module example.com/test\n\nrequire example.com/foo v1.2.3\n"
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("failed writing go.mod: %v", err)
	}

	if version := getVersionFromMod(tempDir, "example.com/foo"); version != "v1.2.3" {
		t.Fatalf("expected v1.2.3, got %q", version)
	}

	if version := getVersionFromMod(tempDir, "example.com/bar"); version != "" {
		t.Fatalf("expected empty version for missing dependency, got %q", version)
	}
}

func TestGetVersionFromModParseError(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module"), 0o644); err != nil {
		t.Fatalf("failed writing invalid go.mod: %v", err)
	}

	if version := getVersionFromMod(tempDir, "example.com/foo"); version != "" {
		t.Fatalf("expected empty version on parse error, got %q", version)
	}
}

func TestGetOrLoadPkgAddsActualVersionKey(t *testing.T) {
	cacheHome := filepath.Join(t.TempDir(), "cachehome")
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	resetCacheGlobals()
	t.Cleanup(resetCacheGlobals)

	g := New()
	cache, err := getCache()
	if err != nil {
		t.Fatalf("unexpected cache init error: %v", err)
	}

	versionKey := getCacheKey("fmt", "latest", "")
	if _, ok := getValidCacheEntry(cache, versionKey); ok {
		t.Fatalf("expected version-specific key to be absent before load")
	}

	if _, _, err := (&g).getOrLoadPkg("fmt", " latest "); err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	if _, ok := getValidCacheEntry(cache, versionKey); !ok {
		t.Fatalf("expected cache entry for actual version key")
	}
}

func TestGetOrLoadSymbolAddsActualVersionKey(t *testing.T) {
	cacheHome := filepath.Join(t.TempDir(), "cachehome")
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	resetCacheGlobals()
	t.Cleanup(resetCacheGlobals)

	g := New()
	cache, err := getCache()
	if err != nil {
		t.Fatalf("unexpected cache init error: %v", err)
	}

	symbolKey := getCacheKey("fmt", "release", "Printf")
	if _, ok := getValidCacheEntry(cache, symbolKey); ok {
		t.Fatalf("expected versioned symbol key absent before load")
	}

	if _, _, err := (&g).getOrLoadSymbol("fmt", "Printf", " release "); err != nil {
		t.Fatalf("unexpected symbol load error: %v", err)
	}

	if _, ok := getValidCacheEntry(cache, symbolKey); !ok {
		t.Fatalf("expected cache entry for versioned symbol key")
	}
}

func TestBuildDocRemoteMetaAdjustsVersion(t *testing.T) {
	g := New()
	d := &g

	d.loadPkg = func(importPath, dir string, needTypes bool) (*doc.Package, *token.FileSet, *types.Info, *packageAST, string, *packages.Module, string, error) {
		if dir != "" {
			t.Fatalf("expected first load to use empty dir, got %q", dir)
		}

		if !needTypes {
			t.Fatalf("expected needTypes true when symbols required")
		}

		return &doc.Package{Name: "foo"}, token.NewFileSet(), &types.Info{}, nil, "example.com/foo", nil, "", nil
	}

	pkgDoc, symbols, pkgPath, actualVersion, meta, err := d.buildDoc("example.com/foo", "v1.2.3", true)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	if pkgDoc.ImportPath != "example.com/foo" {
		t.Fatalf("expected import path preserved, got %q", pkgDoc.ImportPath)
	}

	if pkgPath != "example.com/foo" {
		t.Fatalf("expected pkg path example.com/foo, got %q", pkgPath)
	}

	if actualVersion != "v1.2.3" {
		t.Fatalf("expected trimmed actual version, got %q", actualVersion)
	}

	if symbols == nil {
		t.Fatalf("expected symbol index map")
	}

	if meta.ModuleVersion != "v1.2.3" {
		t.Fatalf("expected module version fallback, got %q", meta.ModuleVersion)
	}

	if meta.GoVersion != "" {
		t.Fatalf("expected go version cleared for remote module, got %q", meta.GoVersion)
	}
}

func TestBuildDocFallbackModule(t *testing.T) {
	g := New()
	d := &g
	d.workdir = t.TempDir()

	modDir := t.TempDir()
	cleanupCalled := false
	loadCalls := 0

	d.loadPkg = func(importPath, dir string, needTypes bool) (*doc.Package, *token.FileSet, *types.Info, *packageAST, string, *packages.Module, string, error) {
		if loadCalls == 0 {
			loadCalls++
			if !needTypes {
				t.Fatalf("expected initial load to request type info when symbols needed")
			}
			return nil, nil, nil, nil, "", nil, "", errors.New("first load failed")
		}

		if dir != modDir {
			t.Fatalf("expected fallback load to use mod dir %q, got %q", modDir, dir)
		}

		if !needTypes {
			t.Fatalf("expected fallback load to preserve type requirement")
		}

		return &doc.Package{Name: "foo"}, token.NewFileSet(), &types.Info{}, nil, importPath, nil, "", nil
	}

	d.checkDep = func(importPath, version string) (string, func(), error) {
		if importPath != "example.com/foo" {
			t.Fatalf("unexpected import path %q", importPath)
		}

		return modDir, func() { cleanupCalled = true }, nil
	}

	pkgDoc, symbols, pkgPath, actualVersion, meta, err := d.buildDoc("example.com/foo", "latest", true)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	if pkgDoc.ImportPath != "example.com/foo" {
		t.Fatalf("expected import path preserved, got %q", pkgDoc.ImportPath)
	}

	if pkgPath != "example.com/foo" {
		t.Fatalf("expected pkg path example.com/foo, got %q", pkgPath)
	}

	if symbols == nil {
		t.Fatalf("expected symbol index map")
	}

	if actualVersion != "latest" {
		t.Fatalf("expected actual version fallback to trimmed version, got %q", actualVersion)
	}

	if meta.ModuleVersion != "latest" {
		t.Fatalf("expected module version from trimmed version, got %q", meta.ModuleVersion)
	}

	if meta.GoVersion != "" {
		t.Fatalf("expected go version cleared when module version present, got %q", meta.GoVersion)
	}

	if !cleanupCalled {
		t.Fatalf("expected cleanup to be invoked")
	}
}

func TestBuildDocFallbackLoadError(t *testing.T) {
	g := New()
	d := &g
	d.workdir = t.TempDir()

	modDir := t.TempDir()
	loadCalls := 0

	d.loadPkg = func(importPath, dir string, needTypes bool) (*doc.Package, *token.FileSet, *types.Info, *packageAST, string, *packages.Module, string, error) {
		if loadCalls == 0 {
			loadCalls++
			if needTypes {
				t.Fatalf("expected type info to be skipped when symbol index not required")
			}
			return nil, nil, nil, nil, "", nil, "", errors.New("first load failed")
		}

		return nil, nil, nil, nil, "", nil, "", errors.New("second load failed")
	}

	d.checkDep = func(string, string) (string, func(), error) {
		return modDir, func() {}, nil
	}

	_, _, _, _, _, err := d.buildDoc("example.com/foo", " ", false)
	if err == nil || !strings.Contains(err.Error(), "load with module dependency failed") {
		t.Fatalf("expected wrapped error from fallback load, got %v", err)
	}
}

func TestSignatureForDeclUsesTypesInfo(t *testing.T) {
	decl := &ast.FuncDecl{Type: &ast.FuncType{}}
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(), nil, false)
	typesInfo := &types.Info{Types: map[ast.Expr]types.TypeAndValue{
		decl.Type: {Type: sig},
	}}

	if got := signatureForDecl(decl, typesInfo); got != sig {
		t.Fatalf("expected signature from types info")
	}
}

func TestExprStringFallback(t *testing.T) {
	got := exprString(&ast.BadExpr{From: token.Pos(10), To: token.Pos(0)}, token.NewFileSet())
	if got != "BadExpr" {
		t.Fatalf("expected fallback string \"BadExpr\", got %q", got)
	}
}
