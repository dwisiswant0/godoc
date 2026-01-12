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

	"go.dw1.io/fastcache"
	"golang.org/x/tools/go/packages"
)

const (
	cacheMaxEntries = 10_000
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
func getCache() (*fastcache.Cache[string, cacheEntry], error) {
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

		cache, err := loadCacheFromFile(cacheFilePath, cacheMaxEntries)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				cachePersistent = false
				cacheFilePath = ""
			}

			cache = fastcache.New[string, cacheEntry](cacheMaxEntries)
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

func getCacheKey(importPath, version, sel string) string {
	hash := fnv.New64a()
	hash.Write([]byte(importPath))
	hash.Write([]byte{0})
	hash.Write([]byte(version))
	hash.Write([]byte{0})
	hash.Write([]byte(sel))

	return fmt.Sprintf("%x", hash.Sum64())
}

func getValidCacheEntry(cache *fastcache.Cache[string, cacheEntry], key string) (cacheEntry, bool) {
	if cache == nil || key == "" {
		return cacheEntry{}, false
	}

	entry, ok := cache.Get(key)
	return entry, ok
}

func setCacheEntry(cache *fastcache.Cache[string, cacheEntry], entry cacheEntry, keys ...string) error {
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

	if err := cache.SaveToFile(cacheFilePath); err != nil {
		if errors.Is(err, fs.ErrPermission) || os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
			return fmt.Errorf("cache persistence permission error: %w", fs.ErrPermission)
		}
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

func loadCacheFromFile(path string, maxEntries int) (_ *fastcache.Cache[string, cacheEntry], err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("load cache panic: %v", r)
		}
	}()

	cache, err := fastcache.LoadFromFile[string, cacheEntry](path)
	if err != nil {
		return nil, err
	}

	if cache == nil {
		cache = fastcache.New[string, cacheEntry](maxEntries)
	}

	return cache, nil
}
