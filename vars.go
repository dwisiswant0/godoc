package godoc

import (
	"regexp"
	"sync"

	"github.com/maypok86/otter/v2"
)

var (
	cacheOnce       sync.Once
	globalCache     *otter.Cache[string, cacheEntry]
	cacheFilePath   string
	cachePersistent bool
	cacheMu         sync.Mutex

	symbolRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*$`)
)
