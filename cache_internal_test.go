package godoc

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"go.dw1.io/fastcache"
	"golang.org/x/tools/go/packages"
)

func TestGetCacheCreatesDirectory(t *testing.T) {
	t.Helper()
	resetCacheGlobals()

	root := t.TempDir()
	cacheHome := filepath.Join(root, "cachehome")
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	cache, err := getCache()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cache == nil {
		t.Fatalf("expected cache instance")
	}

	if cacheFilePath == "" {
		t.Fatalf("expected cache file path to be set")
	}

	if _, err := os.Stat(filepath.Dir(cacheFilePath)); err != nil {
		t.Fatalf("expected cache directory to exist: %v", err)
	}
}

func TestGetCacheFailsWhenCacheDirIsFile(t *testing.T) {
	t.Helper()
	resetCacheGlobals()

	root := t.TempDir()
	cacheHome := filepath.Join(root, "cachehome")
	if err := os.WriteFile(cacheHome, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	cache, err := getCache()
	if err == nil {
		t.Fatalf("expected error, got cache %v", cache)
	}

	if cachePersistent {
		t.Fatalf("expected persistence to remain disabled on failure")
	}
}

func TestGetCacheClearsPersistenceWhenLoadFails(t *testing.T) {
	t.Helper()
	resetCacheGlobals()

	root := t.TempDir()
	cacheHome := filepath.Join(root, "cachehome")
	cacheDir := filepath.Join(cacheHome, "godoc")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	badCache := filepath.Join(cacheDir, "cache.gob")
	if err := os.WriteFile(badCache, []byte("corrupted"), 0o644); err != nil {
		t.Fatalf("failed to write corrupted cache: %v", err)
	}

	t.Setenv("XDG_CACHE_HOME", cacheHome)

	cache, err := getCache()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cache == nil {
		t.Fatalf("expected cache instance")
	}

	if cachePersistent {
		t.Fatalf("expected persistence disabled when load fails")
	}

	if cacheFilePath != "" {
		t.Fatalf("expected cache file path cleared, got %q", cacheFilePath)
	}
}

func TestSetCacheEntryHandlesSaveError(t *testing.T) {
	t.Helper()
	resetCacheGlobals()

	cachePersistent = true
	root := t.TempDir()
	roDir := filepath.Join(root, "readonly")
	if err := os.Mkdir(roDir, 0o555); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	cacheFilePath = filepath.Join(roDir, "cache.gob")

	cache := fastcache.New[string, cacheEntry](10)

	entry := cacheEntry{cacheMetadata: cacheMetadata{GoVersion: runtime.Version()}}
	err := setCacheEntry(cache, entry, "alpha")
	if err == nil {
		t.Fatalf("expected error when saving to readonly directory")
	}

	if !errors.Is(err, fs.ErrPermission) && !os.IsPermission(err) {
		t.Fatalf("expected permission error, got %v", err)
	}

	if _, ok := cache.Get("alpha"); !ok {
		t.Fatalf("expected cache entry to be set despite save error")
	}
}

func TestSetCacheEntrySuccess(t *testing.T) {
	t.Helper()
	resetCacheGlobals()

	cachePersistent = true
	cacheFilePath = filepath.Join(t.TempDir(), "cache.gob")

	cache := fastcache.New[string, cacheEntry](5)

	entry := cacheEntry{cacheMetadata: cacheMetadata{GoVersion: runtime.Version()}}
	if err := setCacheEntry(cache, entry, "alpha", "beta"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if _, err := os.Stat(cacheFilePath); err != nil {
		t.Fatalf("expected cache file to be written: %v", err)
	}
}

func TestSetCacheEntrySkipsPersistenceWhenDisabled(t *testing.T) {
	t.Helper()
	resetCacheGlobals()

	cachePersistent = false
	cacheFilePath = filepath.Join(t.TempDir(), "cache.gob")

	cache := fastcache.New[string, cacheEntry](5)

	entry := cacheEntry{cacheMetadata: cacheMetadata{GoVersion: runtime.Version()}}
	if err := setCacheEntry(cache, entry, "alpha"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if _, err := os.Stat(cacheFilePath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected cache file not to exist, got %v", err)
	}

}

func TestUniqKeys(t *testing.T) {
	t.Helper()

	keys := uniqKeys("", "alpha", "beta", "alpha", "", "gamma")
	expected := []string{"alpha", "beta", "gamma"}

	if !reflect.DeepEqual(keys, expected) {
		t.Fatalf("expected %v, got %v", expected, keys)
	}
}

func TestDeriveCacheMetadata(t *testing.T) {
	t.Helper()

	meta := deriveCacheMetadata(nil, "")
	if meta.GoVersion != runtime.Version() {
		t.Fatalf("expected go version from runtime, got %q", meta.GoVersion)
	}

	module := &packages.Module{Path: "example.com/mod", Version: "  v1.2.3  ", Main: false}
	meta = deriveCacheMetadata(module, "")
	if meta.ModuleVersion != "v1.2.3" {
		t.Fatalf("expected module version trimmed, got %q", meta.ModuleVersion)
	}

	module.Version = ""
	meta = deriveCacheMetadata(module, " v0.9.0 ")
	if meta.ModuleVersion != "v0.9.0" {
		t.Fatalf("expected resolved version applied, got %q", meta.ModuleVersion)
	}

	module.Main = true
	meta = deriveCacheMetadata(module, " v2.0.0 ")
	if meta.ModuleVersion != "v2.0.0" {
		t.Fatalf("expected resolved version override, got %q", meta.ModuleVersion)
	}
}
