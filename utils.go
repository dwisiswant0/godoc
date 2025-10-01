package godoc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// validateInputs checks the import path and symbol for validity and security.
func validateInputs(importPath, symbol string) error {
	if strings.TrimSpace(importPath) == "" {
		return ErrEmptyImportPath
	}

	if strings.Contains(importPath, "..") {
		return fmt.Errorf("%w: cannot contain '..'", ErrInvalidImportPath)
	}

	if strings.HasPrefix(importPath, "/") {
		return fmt.Errorf("%w: cannot start with '/'", ErrInvalidImportPath)
	}

	if symbol != "" && !symbolRegex.MatchString(symbol) {
		return fmt.Errorf("%w: %q", ErrInvalidSymbol, symbol)
	}

	return nil
}

// runGo executes a 'go' command with the given arguments in the specified dir.
func (d *Godoc) runGo(dir string, args ...string) error {
	ctx := context.Background()
	if d != nil {
		ctx = d.context()
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	// Keep env, but force module mode and ignore any parent go.work.
	cmd.Env = append(os.Environ(),
		"GO111MODULE=on",
		"GOWORK=off",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("go %s: %v", strings.Join(args, " "), ctxErr)
		}

		return fmt.Errorf("go %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}

	return nil
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
