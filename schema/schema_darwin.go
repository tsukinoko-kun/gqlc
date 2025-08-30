package schema

import (
	"os"
	"path/filepath"
)

func cacheLocation() string {
	if xdgCacheHome, ok := os.LookupEnv("XDG_CACHE_HOME"); ok {
		return filepath.Join(xdgCacheHome, "gqlc")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Library", "Caches", "gqlc")
	}
	panic("could not determine cache location")
}
