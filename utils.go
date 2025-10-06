package godoc

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"
)

// validateInputs checks the import path and selector for validity and security.
func validateInputs(importPath, sel string) error {
	if strings.TrimSpace(importPath) == "" {
		return ErrEmptyImportPath
	}

	if strings.Contains(importPath, "..") {
		return fmt.Errorf("%w: cannot contain '..'", ErrInvalidImportPath)
	}

	if strings.HasPrefix(importPath, "/") {
		return fmt.Errorf("%w: cannot start with '/'", ErrInvalidImportPath)
	}

	if sel != "" && !selectorRegex.MatchString(sel) {
		return fmt.Errorf("%w: %q", ErrInvalidSelector, sel)
	}

	return nil
}

// runGo executes a 'go' command with the given arguments in the specified dir.
func (d *Godoc) runGo(dir string, args ...string) error {
	ctx := context.Background()
	if d != nil {
		ctx = d.context()
	}

	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	// Keep env, but force module mode and ignore any parent go.work.
	cmd.Env = append(os.Environ(), "GO111MODULE=on", "GOWORK=off")
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("go %s: %v", strings.Join(args, " "), ctxErr)
		}

		// TODO(dwisiswant0): Consider including stderr output in the error.

		return fmt.Errorf("%w", err)
	}

	return nil
}

// getVersionFromMod reads the go.mod file in workdir to find the version of
// the specified importPath. If not found or any error occurs, it returns an
// empty string.
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
			return strings.TrimSpace(r.Mod.Version)
		}
	}

	return ""
}

// getPkgVersion gets the appropriate version string for the package based on
// import path and requested version.
func getPkgVersion(importPath, version string) string {
	trimmed := strings.TrimSpace(version)
	if importPath == "." {
		return ""
	}

	if !isRemoteImportPath(importPath) {
		return runtime.Version()
	}

	return trimmed
}

// isRemoteImportPath checks if the import path is a remote path (not std).
func isRemoteImportPath(importPath string) bool {
	if importPath == "." {
		return false
	}

	first := importPath
	if idx := strings.Index(importPath, "/"); idx >= 0 {
		first = importPath[:idx]
	}

	return strings.Contains(first, ".")
}

func resetCacheGlobals() {
	cacheOnce = sync.Once{}
	globalCache = nil
	cacheFilePath = ""
	cachePersistent = false
}
