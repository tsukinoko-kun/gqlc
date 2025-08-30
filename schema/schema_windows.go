package schema

import (
	"os"
	"path/filepath"
)

func cacheLocation() string {
	if localAppData, ok := os.LookupEnv("LOCALAPPDATA"); ok {
		return filepath.Join(localAppData, "gqlc", "cache")
	}
	if appData, ok := os.LookupEnv("APPDATA"); ok {
		return filepath.Join(appData, "gqlc", "cache")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "AppData", "Local", "gqlc", "cache")
	}
	panic("could not determine cache location")
}
