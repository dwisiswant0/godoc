package godoc

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/doc"
	"go/doc/comment"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

// FuncDoc represents documentation for a function.
type FuncDoc struct {
	Name string    `json:"name" jsonschema:"function name"`
	Args []ArgInfo `json:"args" jsonschema:"function arguments"`
	Doc  string    `json:"doc" jsonschema:"function documentation"`
}

// ValueDoc represents documentation for a constant or variable.
type ValueDoc struct {
	Names []string `json:"names" jsonschema:"value identifiers"`
	Doc   string   `json:"doc" jsonschema:"value documentation"`
}

// ArgInfo represents information about a function or method argument.
type ArgInfo struct {
	Name string `json:"name" jsonschema:"argument name"`
	Type string `json:"type" jsonschema:"argument type"`
}

// MethodDoc represents documentation for a method.
type MethodDoc struct {
	Recv string    `json:"recv" jsonschema:"receiver type name"`
	Name string    `json:"name" jsonschema:"method name"`
	Args []ArgInfo `json:"args" jsonschema:"method arguments"`
	Doc  string    `json:"doc" jsonschema:"method documentation"`
}

// TypeDoc represents documentation for a type, including its methods.
type TypeDoc struct {
	Name    string      `json:"name" jsonschema:"type name"`
	Doc     string      `json:"doc" jsonschema:"type documentation"`
	Methods []MethodDoc `json:"methods" jsonschema:"associated methods"`
}

// PackageDoc represents documentation for a Go package.
type PackageDoc struct {
	ImportPath string     `json:"import_path" jsonschema:"package import path"`
	Name       string     `json:"name" jsonschema:"package name"`
	Synopsis   string     `json:"synopsis" jsonschema:"package synopsis"`
	DocText    string     `json:"doc" jsonschema:"package documentation text"`
	DocHTML    string     `json:"-" jsonschema:"package documentation HTML"`
	Consts     []ValueDoc `json:"consts" jsonschema:"package constants"`
	Vars       []ValueDoc `json:"vars" jsonschema:"package variables"`
	Funcs      []FuncDoc  `json:"funcs" jsonschema:"package functions"`
	Types      []TypeDoc  `json:"types" jsonschema:"package types"`
}

// Text returns the plain text documentation for the package.
func (p PackageDoc) Text() string {
	return p.DocText
}

// HTML returns the HTML documentation for the package.
func (p PackageDoc) HTML() string {
	return p.DocHTML
}

// MarshalJSON implements [json.Marshaler] while omitting internal fields.
func (p PackageDoc) MarshalJSON() ([]byte, error) {
	type alias PackageDoc

	return json.Marshal(alias(p))
}

// SymbolDoc represents documentation for a specific symbol (type, method,
// function, const, or var).
type SymbolDoc struct {
	ImportPath string       `json:"import_path" jsonschema:"package import path"`
	Package    string       `json:"package" jsonschema:"package name"`
	Kind       string       `json:"kind" jsonschema:"symbol kind"`
	Name       string       `json:"name" jsonschema:"symbol name"`
	Receiver   string       `json:"receiver" jsonschema:"receiver type name"`
	Args       []ArgInfo    `json:"args" jsonschema:"symbol arguments"`
	DocText    string       `json:"doc" jsonschema:"symbol documentation text"`
	DocHTML    string       `json:"-" jsonschema:"symbol documentation HTML"`
	docParsed  *comment.Doc // For lazy HTML generation
}

// Text returns the plain text documentation for the symbol.
func (s SymbolDoc) Text() string {
	return s.DocText
}

// HTML returns the HTML documentation for the symbol.
func (s SymbolDoc) HTML() string {
	if s.DocHTML == "" && s.docParsed != nil {
		r := comment.Printer{HeadingLevel: 3}
		s.DocHTML = string(r.HTML(s.docParsed))
	}

	return s.DocHTML
}

// MarshalJSON implements [json.Marshaler] while omitting internal fields.
func (s SymbolDoc) MarshalJSON() ([]byte, error) {
	type alias SymbolDoc

	return json.Marshal(alias(s))
}

// Result is an interface for documentation results, providing access to
// documentation text, HTML, and JSON serialization.
type Result interface {
	Text() string
	HTML() string
	MarshalJSON() ([]byte, error)
}

// Godoc handles the extraction of Go package documentation.
type Godoc struct {
	goos    string
	goarch  string
	workdir string
	ctx     context.Context
}

// New creates a new [Godoc] with the specified configuration.
func New(opts ...Option) Godoc {
	g := Godoc{
		workdir: ".", // Default
		ctx:     context.Background(),
	}

	for _, opt := range opts {
		opt(&g)
	}

	if g.ctx == nil {
		g.ctx = context.Background()
	}

	return g
}

func (d *Godoc) context() context.Context {
	if d == nil || d.ctx == nil {
		return context.Background()
	}

	return d.ctx
}

// Load loads documentation for a Go package or a specific symbol within it.
//
// If symbol is empty, it loads the entire package documentation.
// Otherwise, it loads documentation for the specified symbol (type, method,
// function, const, or var).
//
// For remote packages, it may add them to the current module to fetch the
// documentation.
//
// Version specifies the module version to use; if empty, uses the latest.
func (d *Godoc) Load(importPath, symbol, version string) (Result, error) {
	if err := validateInputs(importPath, symbol); err != nil {
		return nil, err
	}

	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		pkgDoc, _, err := d.getOrLoadPkg(importPath, version)
		if err != nil {
			return nil, err
		}

		return pkgDoc, nil
	}

	symDoc, pkgPath, err := d.getOrLoadSymbol(importPath, symbol, version)
	if err != nil {
		return nil, err
	}

	if symDoc.ImportPath == "" {
		// Defensive: ensure import path metadata is populated for results
		// originating from cache.
		symDoc.ImportPath = pkgPath
	}

	return symDoc, nil
}

// getOrLoadPkg gets package doc from cache (or loads it if not cached).
func (d *Godoc) getOrLoadPkg(importPath, version string) (PackageDoc, string, error) {
	cache, err := getCache()
	if err != nil {
		return PackageDoc{}, "", err
	}

	expected := getPkgVersion(importPath, version)
	key := getCacheKey(importPath, expected, "")

	if entry, ok := cache.GetIfPresent(key); ok {
		if entry.Package != nil {
			if isRemoteImportPath(importPath) {
				return *entry.Package, entry.Package.ImportPath, nil
			}

			if entry.GoVersion == runtime.Version() {
				return *entry.Package, entry.Package.ImportPath, nil
			}
		}

		// stale entry or missing package payload; fall through to rebuild
	}

	pkgDoc, _, pkgPath, actualVersion, meta, err := d.buildDoc(importPath, version)
	if err != nil {
		return PackageDoc{}, "", err
	}

	entry := cacheEntry{
		Package:       &pkgDoc,
		cacheMetadata: meta,
	}

	keys := uniqKeys(key, getCacheKey(importPath, "", ""))
	if actualVersion != "" {
		keys = append(keys, getCacheKey(importPath, actualVersion, ""))
	}

	if err := setCacheEntry(cache, entry, keys...); err != nil {
		return PackageDoc{}, "", err
	}

	return pkgDoc, pkgPath, nil
}

// getOrLoadSymbol gets symbol doc from cache (or loads it if not cached).
func (d *Godoc) getOrLoadSymbol(importPath, symbol, version string) (SymbolDoc, string, error) {
	cache, err := getCache()
	if err != nil {
		return SymbolDoc{}, "", err
	}

	trimmedSymbol := strings.TrimSpace(symbol)
	expected := getPkgVersion(importPath, version)
	key := getCacheKey(importPath, expected, trimmedSymbol)

	if entry, ok := cache.GetIfPresent(key); ok {
		if entry.Symbol != nil {
			if isRemoteImportPath(importPath) {
				return *entry.Symbol, entry.Symbol.ImportPath, nil
			}

			if entry.GoVersion == runtime.Version() {
				return *entry.Symbol, entry.Symbol.ImportPath, nil
			}
		}

		// stale entry or missing symbol payload; fall through to rebuild
	}

	_, symbols, pkgPath, actualVersion, meta, err := d.buildDoc(importPath, version)
	if err != nil {
		return SymbolDoc{}, "", err
	}

	symDoc, ok := symbols[trimmedSymbol]
	if !ok {
		return SymbolDoc{}, pkgPath, fmt.Errorf("symbol %q not found in %q", symbol, pkgPath)
	}

	entry := cacheEntry{
		Symbol:        &symDoc,
		cacheMetadata: meta,
	}

	keys := uniqKeys(key, getCacheKey(importPath, "", trimmedSymbol))
	if actualVersion != "" {
		keys = append(keys, getCacheKey(importPath, actualVersion, trimmedSymbol))
	}

	if err := setCacheEntry(cache, entry, keys...); err != nil {
		return SymbolDoc{}, "", err
	}

	return symDoc, pkgPath, nil
}

// buildDoc loads and builds documentation for the specified import path and
// version.
func (d *Godoc) buildDoc(importPath, version string) (PackageDoc, map[string]SymbolDoc, string, string, cacheMetadata, error) {
	trimmedVersion := strings.TrimSpace(version)

	dpkg, fset, typesInfo, pkgPath, module, _, err := d.loadDocPkg(importPath, "")
	if err == nil {
		pkgDoc := toPkgDoc(dpkg, fset, typesInfo, pkgPath)
		symbols := buildSymbolIndex(dpkg, fset, typesInfo, pkgPath)
		meta := deriveCacheMetadata(module, trimmedVersion)

		if isRemoteImportPath(importPath) {
			if meta.ModuleVersion == "" && strings.TrimSpace(trimmedVersion) != "" {
				meta.ModuleVersion = strings.TrimSpace(trimmedVersion)
			}

			if strings.TrimSpace(meta.ModuleVersion) != "" {
				meta.GoVersion = ""
			}
		}

		return pkgDoc, symbols, pkgPath, trimmedVersion, meta, nil
	}

	modDir, cleanup, err2 := d.checkModuleDep(importPath, trimmedVersion)
	if err2 != nil {
		return PackageDoc{}, nil, "", "", cacheMetadata{}, fmt.Errorf("local load failed (%v) and module dependency setup failed (%v)", err, err2)
	}

	if cleanup != nil && modDir != d.workdir {
		defer cleanup()
	}

	dpkg2, fset2, typesInfo2, pkgPath2, module2, _, err3 := d.loadDocPkg(importPath, modDir)
	if err3 != nil {
		return PackageDoc{}, nil, "", "", cacheMetadata{}, fmt.Errorf("load with module dependency failed: %w", err3)
	}

	pkgDoc := toPkgDoc(dpkg2, fset2, typesInfo2, pkgPath2)
	symbols := buildSymbolIndex(dpkg2, fset2, typesInfo2, pkgPath2)

	actualVersion := strings.TrimSpace(getVersionFromMod(modDir, importPath))
	if actualVersion == "" {
		actualVersion = trimmedVersion
	}

	meta := deriveCacheMetadata(module2, actualVersion)
	if isRemoteImportPath(importPath) {
		if meta.ModuleVersion == "" && strings.TrimSpace(actualVersion) != "" {
			meta.ModuleVersion = strings.TrimSpace(actualVersion)
		}

		if strings.TrimSpace(meta.ModuleVersion) != "" {
			meta.GoVersion = ""
		}
	}

	return pkgDoc, symbols, pkgPath2, actualVersion, meta, nil
}

// loadDocPkg loads documentation for a Go package.
func (d *Godoc) loadDocPkg(importPath, dir string) (*doc.Package, *token.FileSet, *types.Info, string, *packages.Module, string, error) {
	ctx := d.context()
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo,
		Env:     append(os.Environ(), "GOWORK=off"),
		Dir:     dir, // empty = current working directory/module
		Context: ctx,
	}

	// load from GOROOT
	if dir == "." && importPath != "." && !strings.Contains(importPath, "/") {
		cfg.Dir = ""
	}

	if d.goos != "" {
		cfg.Env = append(cfg.Env, "GOOS="+d.goos)
	}

	if d.goarch != "" {
		cfg.Env = append(cfg.Env, "GOARCH="+d.goarch)
	}

	pkgs, err := packages.Load(cfg, importPath)
	if err != nil {
		return nil, nil, nil, "", nil, "", err
	}

	var hasErrors bool
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		return nil, nil, nil, "", nil, "", fmt.Errorf("build/load errors for %q", importPath)
	}

	var p *packages.Package
	for _, cand := range pkgs {
		if len(cand.Syntax) > 0 {
			p = cand
			break
		}
	}

	if p == nil {
		return nil, nil, nil, "", nil, "", fmt.Errorf("no syntax found for %q", importPath)
	}

	var files []*ast.File
	for i, f := range p.Syntax {
		fn := p.GoFiles[i]
		if strings.HasSuffix(fn, "_test.go") {
			continue
		}

		files = append(files, f)
	}

	dpkg, err := doc.NewFromFiles(p.Fset, files, p.PkgPath)
	if err != nil {
		return nil, nil, nil, "", nil, "", err
	}

	return dpkg, p.Fset, p.TypesInfo, p.PkgPath, p.Module, cfg.Dir, nil
}

// checkModuleDep ensures the target import is available for loading.
//
// If the importPath is already in the go.mod of the specified dir, uses that
// dir. Otherwise, creates a temp module and adds the import there.
func (d *Godoc) checkModuleDep(importPath, version string) (string, func(), error) {
	modFilePath := filepath.Join(d.workdir, "go.mod")
	data, err := os.ReadFile(modFilePath)
	if err == nil {
		f, err := modfile.Parse(modFilePath, data, nil)
		if err == nil {
			for _, r := range f.Require {
				if r.Mod.Path == importPath {
					if version == "" || r.Mod.Version == version {
						return d.workdir, nil, nil
					}
					// Version mismatch, fallback to temp
				}
			}
		}
		// go.mod exists but importPath not present/parse error, fallback to temp.
	}

	tempDir, err := os.MkdirTemp("", "godoc-*")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() { _ = os.RemoveAll(tempDir) }

	if err := d.runGo(tempDir, "mod", "init", filepath.Base(tempDir)); err != nil {
		cleanup()

		return "", nil, fmt.Errorf("go mod init failed: %w", err)
	}

	target := importPath
	if strings.TrimSpace(version) != "" {
		target = importPath + "@" + strings.TrimSpace(version)
	}

	if err := d.runGo(tempDir, "get", target); err != nil {
		cleanup()

		return "", nil, fmt.Errorf("go get %q failed: %w", target, err)
	}

	_ = d.runGo(tempDir, "mod", "download", "all")
	// not fatal; some modules may not need download if go get already did it.

	return tempDir, cleanup, nil
}

// buildSymbolIndex builds an index of symbols in the package for quick lookup.
func buildSymbolIndex(p *doc.Package, fset *token.FileSet, typesInfo *types.Info, importPath string) map[string]SymbolDoc {
	if p == nil {
		return nil
	}

	result := make(map[string]SymbolDoc)

	add := func(key string, doc SymbolDoc) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}

		result[key] = doc
	}

	for _, t := range p.Types {
		add(t.Name, makeSymbolDoc(importPath, p, "type", t.Name, "", t.Doc, nil))

		for _, m := range t.Methods {
			add(t.Name+"."+m.Name, makeSymbolDoc(importPath, p, "method", m.Name, t.Name, m.Doc, extractArgs(m.Decl, fset, typesInfo)))
		}

		for _, f := range t.Funcs {
			add(t.Name+"."+f.Name, makeSymbolDoc(importPath, p, "func", f.Name, "", f.Doc, extractArgs(f.Decl, fset, typesInfo)))
		}
	}

	for _, f := range p.Funcs {
		add(f.Name, makeSymbolDoc(importPath, p, "func", f.Name, "", f.Doc, extractArgs(f.Decl, fset, typesInfo)))
	}

	for _, c := range p.Consts {
		for _, name := range c.Names {
			add(name, makeSymbolDoc(importPath, p, "const", name, "", c.Doc, nil))
		}
	}

	for _, v := range p.Vars {
		for _, name := range v.Names {
			add(name, makeSymbolDoc(importPath, p, "var", name, "", v.Doc, nil))
		}
	}

	return result
}

// toPkgDoc converts a doc.Package to a PackageDoc struct, extracting constants,
// variables, functions, and types.
func toPkgDoc(p *doc.Package, fset *token.FileSet, typesInfo *types.Info, importPath string) PackageDoc {
	syn := p.Synopsis(p.Doc)

	var (
		consts = make([]ValueDoc, 0, len(p.Consts))
		vars   = make([]ValueDoc, 0, len(p.Vars))
		funcs  = make([]FuncDoc, 0, len(p.Funcs)+len(p.Types))
		types  = make([]TypeDoc, 0, len(p.Types))
	)

	for _, c := range p.Consts {
		consts = append(consts, ValueDoc{
			Names: c.Names,
			Doc:   strings.TrimSpace(c.Doc),
		})
	}

	for _, v := range p.Vars {
		vars = append(vars, ValueDoc{
			Names: v.Names,
			Doc:   strings.TrimSpace(v.Doc),
		})
	}

	for _, f := range p.Funcs {
		funcs = append(funcs, FuncDoc{
			Name: f.Name,
			Args: extractArgs(f.Decl, fset, typesInfo),
			Doc:  strings.TrimSpace(f.Doc),
		})
	}

	for _, t := range p.Types {
		for _, f := range t.Funcs {
			funcs = append(funcs, FuncDoc{
				Name: f.Name,
				Args: extractArgs(f.Decl, fset, typesInfo),
				Doc:  strings.TrimSpace(f.Doc),
			})
		}
	}

	for _, t := range p.Types {
		td := TypeDoc{
			Name:    t.Name,
			Doc:     strings.TrimSpace(t.Doc),
			Methods: make([]MethodDoc, 0, len(t.Methods)),
		}

		for _, m := range t.Methods {
			td.Methods = append(td.Methods, MethodDoc{
				Recv: t.Name,
				Name: m.Name,
				Args: extractArgs(m.Decl, fset, typesInfo),
				Doc:  strings.TrimSpace(m.Doc),
			})
		}

		types = append(types, td)
	}

	var html string
	if strings.TrimSpace(p.Doc) != "" {
		d := new(comment.Parser).Parse(p.Doc)
		r := comment.Printer{HeadingLevel: 2}
		html = string(r.HTML(d))
	}

	sort.Slice(consts, func(i, j int) bool { return strings.Join(consts[i].Names, ",") < strings.Join(consts[j].Names, ",") })
	sort.Slice(vars, func(i, j int) bool { return strings.Join(vars[i].Names, ",") < strings.Join(vars[j].Names, ",") })
	sort.Slice(funcs, func(i, j int) bool { return funcs[i].Name < funcs[j].Name })
	sort.Slice(types, func(i, j int) bool { return types[i].Name < types[j].Name })

	return PackageDoc{
		ImportPath: importPath,
		Name:       p.Name,
		Synopsis:   syn,
		DocText:    strings.TrimSpace(p.Doc),
		DocHTML:    html,
		Consts:     consts,
		Vars:       vars,
		Funcs:      funcs,
		Types:      types,
	}
}

// makeSymbolDoc creates a SymbolDoc struct from the provided parameters,
// including lazy HTML rendering of documentation.
func makeSymbolDoc(importPath string, p *doc.Package, kind, name, recv, text string, args []ArgInfo) SymbolDoc {
	var html string
	if strings.TrimSpace(text) != "" {
		doc := new(comment.Parser).Parse(text)
		r := comment.Printer{HeadingLevel: 3}
		html = string(r.HTML(doc))
	}

	return SymbolDoc{
		ImportPath: importPath,
		Package:    p.Name,
		Kind:       kind,
		Name:       name,
		Receiver:   recv,
		Args:       args,
		DocText:    strings.TrimSpace(text),
		DocHTML:    html,
	}
}

// extractArgs extracts argument information from a function declaration.
func extractArgs(decl *ast.FuncDecl, _ *token.FileSet, typesInfo *types.Info) []ArgInfo {
	if decl == nil || decl.Type == nil || decl.Type.Params == nil {
		return nil
	}

	var args []ArgInfo

	for _, field := range decl.Type.Params.List {
		typ := ""
		if field.Type != nil && typesInfo != nil {
			if ell, ok := field.Type.(*ast.Ellipsis); ok {
				if ell.Elt != nil {
					if tv, ok := typesInfo.Types[ell.Elt]; ok {
						typ = "..." + tv.Type.String()
					}
				}
			} else {
				if tv, ok := typesInfo.Types[field.Type]; ok {
					typ = tv.Type.String()
				}
			}
		}

		if len(field.Names) == 0 {
			// Unnamed parameter
			args = append(args, ArgInfo{Name: "", Type: typ})
		} else {
			for _, name := range field.Names {
				args = append(args, ArgInfo{Name: name.Name, Type: typ})
			}
		}
	}

	return args
}

// getVersionFromMod parses the go.mod file in workdir and returns the version
// for the given importPath. If not found or error, returns empty string.
func getVersionFromMod(workdir, importPath string) string {
	modFilePath := filepath.Join(workdir, "go.mod")

	data, err := os.ReadFile(modFilePath)
	if err != nil {
		return ""
	}

	f, err := modfile.Parse(modFilePath, data, nil)
	if err != nil {
		return ""
	}

	for _, r := range f.Require {
		if r.Mod.Path == importPath {
			return r.Mod.Version
		}
	}

	return ""
}
