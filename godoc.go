package godoc

import (
	"context"
	"fmt"
	"go/ast"
	"go/doc"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

// Godoc handles the extraction of Go package documentation.
type Godoc struct {
	goos     string
	goarch   string
	workdir  string
	ctx      context.Context
	loadPkg  func(string, string, bool) (*doc.Package, *token.FileSet, *types.Info, *packageAST, string, *packages.Module, string, error)
	checkDep func(string, string) (string, func(), error)
	depCache *sync.Map
}

// New creates a new [Godoc] with the specified configuration.
func New(opts ...Option) Godoc {
	g := Godoc{
		workdir:  ".", // Default
		ctx:      context.Background(),
		depCache: &sync.Map{},
	}

	g.SetOptions(opts...)

	if g.ctx == nil {
		g.ctx = context.Background()
	}

	g.loadPkg = g.loadDocPkg
	g.checkDep = g.checkModuleDep

	return g
}

// context returns the effective context for operations.
func (d *Godoc) context() context.Context {
	if d == nil || d.ctx == nil {
		return context.Background()
	}

	return d.ctx
}

// Load loads documentation for a Go package or a specific selector within it.
//
// If sel is empty, it loads the entire package documentation.
// Otherwise, it loads documentation for the specified selector (type, method,
// function, const, or var).
//
// For remote packages, it may add them to the current module to fetch the
// documentation.
//
// Version specifies the module version to use; if empty, uses the latest.
func (d *Godoc) Load(importPath, sel, version string) (Result, error) {
	if err := validateInputs(importPath, sel); err != nil {
		return nil, err
	}

	if sel == "" {
		pkgDoc, _, err := d.getOrLoadPkg(importPath, version)
		if err != nil {
			return nil, err
		}

		return pkgDoc, nil
	}

	symDoc, pkgPath, err := d.getOrLoadSymbol(importPath, sel, version)
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

	if entry, ok := getValidCacheEntry(cache, key); ok {
		if entry.Package != nil {
			if isRemoteImportPath(importPath) {
				return *entry.Package, entry.Package.ImportPath, nil
			}

			if entry.GoVersion == runtime.Version() || (entry.GoVersion == "" && expected == runtime.Version()) {
				if entry.GoVersion == "" {
					entry.GoVersion = runtime.Version()
					cache.Set(key, entry)
				}
				return *entry.Package, entry.Package.ImportPath, nil
			}
		}

		// stale entry or missing package payload; fall through to rebuild
	}

	pkgDoc, _, pkgPath, actualVersion, meta, err := d.buildDoc(importPath, version, false)
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
func (d *Godoc) getOrLoadSymbol(importPath, sel, version string) (SymbolDoc, string, error) {
	cache, err := getCache()
	if err != nil {
		return SymbolDoc{}, "", err
	}

	expected := getPkgVersion(importPath, version)
	key := getCacheKey(importPath, expected, sel)

	if entry, ok := getValidCacheEntry(cache, key); ok {
		if entry.Symbol != nil {
			if isRemoteImportPath(importPath) {
				return *entry.Symbol, entry.Symbol.ImportPath, nil
			}

			if entry.GoVersion == runtime.Version() || (entry.GoVersion == "" && expected == runtime.Version()) {
				if entry.GoVersion == "" {
					entry.GoVersion = runtime.Version()
					cache.Set(key, entry)
				}
				return *entry.Symbol, entry.Symbol.ImportPath, nil
			}
		}

		// stale entry or missing symbol payload; fall through to rebuild
	}

	_, symbols, pkgPath, actualVersion, meta, err := d.buildDoc(importPath, version, true)
	if err != nil {
		return SymbolDoc{}, "", err
	}

	symDoc, ok := symbols[sel]
	if !ok {
		return SymbolDoc{}, pkgPath, fmt.Errorf("selector %q not found in %q", sel, pkgPath)
	}

	entry := cacheEntry{
		Symbol:        &symDoc,
		cacheMetadata: meta,
	}

	keys := uniqKeys(key, getCacheKey(importPath, "", sel))
	if actualVersion != "" {
		keys = append(keys, getCacheKey(importPath, actualVersion, sel))
	}

	if err := setCacheEntry(cache, entry, keys...); err != nil {
		return SymbolDoc{}, "", err
	}

	return symDoc, pkgPath, nil
}

// buildDoc loads and builds documentation for the specified import path and
// version.
func (d *Godoc) buildDoc(importPath, version string, needSymbols bool) (PackageDoc, map[string]SymbolDoc, string, string, cacheMetadata, error) {
	version = strings.TrimSpace(version)
	if d.loadPkg == nil {
		d.loadPkg = d.loadDocPkg
	}

	if d.checkDep == nil {
		d.checkDep = d.checkModuleDep
	}

	var symbols, symbols2 map[string]SymbolDoc

	needTypes := needSymbols
	dpkg, fset, typesInfo, astInfo, pkgPath, module, _, err := d.loadPkg(importPath, "", needTypes)
	if err == nil {
		if !needTypes && pkgRequiresTypesInfo(dpkg) {
			dpkgTyped, fsetTyped, typesInfoTyped, astInfoTyped, pkgPathTyped, moduleTyped, _, typedErr := d.loadPkg(importPath, "", true)
			if typedErr == nil {
				dpkg = dpkgTyped
				fset = fsetTyped
				typesInfo = typesInfoTyped
				pkgPath = pkgPathTyped
				module = moduleTyped
				astInfo = astInfoTyped
			} else {
				// Preserve original package data if type-enriched load fails.
				_ = typedErr
			}
		}

		pkgDoc := toPkgDoc(dpkg, fset, typesInfo, astInfo, pkgPath)
		if needSymbols {
			symbols = buildSymbolIndex(dpkg, fset, typesInfo, astInfo, pkgPath)
		}
		meta := deriveCacheMetadata(module, version)

		if isRemoteImportPath(importPath) {
			if meta.ModuleVersion == "" && version != "" {
				meta.ModuleVersion = version
			}

			if meta.ModuleVersion != "" {
				meta.GoVersion = ""
			}
		}

		return pkgDoc, symbols, pkgPath, version, meta, nil
	}

	modDir, cleanup, err2 := d.checkDep(importPath, version)
	if err2 != nil {
		return PackageDoc{}, nil, "", "", cacheMetadata{}, fmt.Errorf("local load failed (%w) and module dependency setup failed (%w)", err, err2)
	}

	if cleanup != nil && modDir != d.workdir {
		defer cleanup()
	}

	dpkg2, fset2, typesInfo2, astInfo2, pkgPath2, module2, _, err3 := d.loadPkg(importPath, modDir, true)
	if err3 != nil {
		return PackageDoc{}, nil, "", "", cacheMetadata{}, fmt.Errorf("load with module dependency failed: %w", err3)
	}

	pkgDoc := toPkgDoc(dpkg2, fset2, typesInfo2, astInfo2, pkgPath2)
	if needSymbols {
		symbols2 = buildSymbolIndex(dpkg2, fset2, typesInfo2, astInfo2, pkgPath2)
	}

	actualVersion := getVersionFromMod(modDir, importPath)
	if actualVersion == "" {
		actualVersion = version
	}

	meta := deriveCacheMetadata(module2, actualVersion)
	if isRemoteImportPath(importPath) {
		if meta.ModuleVersion == "" && actualVersion != "" {
			meta.ModuleVersion = actualVersion
		}

		if meta.ModuleVersion != "" {
			meta.GoVersion = ""
		}
	}

	return pkgDoc, symbols2, pkgPath2, actualVersion, meta, nil
}

// loadDocPkg loads documentation for a Go package.
func (d *Godoc) loadDocPkg(importPath, dir string, needTypes bool) (*doc.Package, *token.FileSet, *types.Info, *packageAST, string, *packages.Module, string, error) {
	ctx := d.context()
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedCompiledGoFiles |
			packages.NeedModule,
		Env:     append(os.Environ(), "GOWORK=off"),
		Dir:     dir, // empty = current working directory/module
		Context: ctx,
	}

	if needTypes {
		cfg.Mode |= packages.NeedTypes | packages.NeedTypesInfo
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
		return nil, nil, nil, nil, "", nil, "", err
	}

	var hasErrors bool
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		return nil, nil, nil, nil, "", nil, "", fmt.Errorf("build/load errors for %q", importPath)
	}

	var p *packages.Package
	for _, cand := range pkgs {
		if len(cand.Syntax) > 0 {
			p = cand
			break
		}
	}

	if p == nil {
		return nil, nil, nil, nil, "", nil, "", fmt.Errorf("no syntax found for %q", importPath)
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
		return nil, nil, nil, nil, "", nil, "", err
	}

	astInfo := buildPkgAST(p, files)

	return dpkg, p.Fset, p.TypesInfo, astInfo, p.PkgPath, p.Module, cfg.Dir, nil
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

	targetKey := importPath
	if version != "" {
		targetKey = importPath + "@" + version
	}

	if cache := d.depCache; cache != nil {
		if cached, ok := cache.Load(targetKey); ok {
			if useWorkdir, _ := cached.(bool); useWorkdir {
				return d.workdir, nil, nil
			}
		} else {
			target := importPath
			if version != "" {
				target = targetKey
			}

			if err := d.runGo(d.workdir, "list", target); err == nil {
				cache.Store(targetKey, true)

				return d.workdir, nil, nil
			}

			cache.Store(targetKey, false)
		}
	} else {
		target := importPath
		if version != "" {
			target = targetKey
		}

		if err := d.runGo(d.workdir, "list", target); err == nil {
			return d.workdir, nil, nil
		}
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
	if version != "" {
		target = importPath + "@" + version
	}

	if err := d.runGo(tempDir, "get", target); err != nil {
		cleanup()

		return "", nil, fmt.Errorf("go get %q failed: %w", target, err)
	}

	return tempDir, cleanup, nil
}
