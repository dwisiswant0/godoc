package godoc

import (
	"regexp"
	"sync"

	"go.dw1.io/fastcache"
)

var (
	cacheOnce       sync.Once
	globalCache     *fastcache.Cache[string, cacheEntry]
	cacheFilePath   string
	cachePersistent bool
	cacheMu         sync.Mutex

	selectorRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*$`)
)
