package godoc

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/maypok86/otter/v2"
	"github.com/maypok86/otter/v2/stats"
	"golang.org/x/tools/go/packages"
)

// cacheMetadata holds metadata about the cached entry.
type cacheMetadata struct {
	GoVersion     string
	ModuleVersion string
}

// cacheEntry represents a cached documentation entry.
type cacheEntry struct {
	Package *PackageDoc
	Symbol  *SymbolDoc
	cacheMetadata
}

// getCache initializes and returns the global cache instance.
func getCache() (*otter.Cache[string, cacheEntry], error) {
	var cacheInitErr error

	cacheOnce.Do(func() {
		dir, err := getCacheDir()
		if err != nil {
			cacheInitErr = err

			return
		}

		if err := os.MkdirAll(dir, 0o755); err != nil {
			cacheInitErr = fmt.Errorf("could not create cache directory: %w", err)

			return
		}

		cacheFilePath = filepath.Join(dir, "cache.gob")
		cachePersistent = true

		cache := otter.Must(&otter.Options[string, cacheEntry]{
			MaximumSize:      10_000,
			ExpiryCalculator: otter.ExpiryWriting[string, cacheEntry](24 * time.Hour),
			StatsRecorder:    stats.NewCounter(),
		})

		if err := loadCacheFromFile(cache, cacheFilePath); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				cachePersistent = false
				cacheFilePath = ""
			}
		}

		globalCache = cache
	})

	if cacheInitErr != nil {
		return nil, cacheInitErr
	}

	return globalCache, nil
}

func getCacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("could not get user cache directory: %w", err)
	}

	return filepath.Join(dir, "godoc"), nil
}

func getCacheKey(importPath, version, symbol string) string {
	hash := fnv.New64a()
	hash.Write([]byte(importPath))
	hash.Write([]byte{0})
	hash.Write([]byte(version))
	hash.Write([]byte{0})
	hash.Write([]byte(symbol))

	return fmt.Sprintf("%x", hash.Sum64())
}

func setCacheEntry(cache *otter.Cache[string, cacheEntry], entry cacheEntry, keys ...string) error {
	for _, key := range keys {
		if key == "" {
			continue
		}

		cache.Set(key, entry)
	}

	if !cachePersistent || cacheFilePath == "" {
		return nil
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	if err := otter.SaveCacheToFile(cache, cacheFilePath); err != nil {
		return err
	}

	return nil
}

func uniqKeys(keys ...string) []string {
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))

	for _, key := range keys {
		if key == "" {
			continue
		}

		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		out = append(out, key)
	}

	return out
}

func deriveCacheMetadata(module *packages.Module, resolvedVersion string) cacheMetadata {
	meta := cacheMetadata{}
	resolved := strings.TrimSpace(resolvedVersion)

	if module == nil || module.Path == "" {
		meta.GoVersion = runtime.Version()

		return meta
	}

	if !module.Main {
		version := strings.TrimSpace(module.Version)
		if version == "" {
			version = resolved
		}

		meta.ModuleVersion = version

		return meta
	}

	if resolved != "" {
		meta.ModuleVersion = resolved
	}

	return meta
}

func loadCacheFromFile(cache *otter.Cache[string, cacheEntry], path string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("load cache panic: %v", r)
		}
	}()

	return otter.LoadCacheFromFile(cache, path)
}
