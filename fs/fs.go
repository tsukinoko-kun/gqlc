package fs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CollectGraphQLFiles collects all GraphQL files from the given path.
// If the path is a file, it returns that file.
// If the path is a directory, it walks the directory and collects all .graphql and .gql files.
func CollectGraphQLFiles(path string) ([]*os.File, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path %s: %w", path, err)
	}

	var files []*os.File

	if stat.IsDir() {
		// Walk directory and collect GraphQL files
		if err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !isGraphQLFile(p) {
				return nil
			}
			f, err := os.Open(p)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", p, err)
			}
			files = append(files, f)
			return nil
		}); err != nil {
			// Close any opened files on error
			closeFiles(files)
			return nil, fmt.Errorf("failed to walk directory %s: %w", path, err)
		}
	} else {
		// Single file
		if !isGraphQLFile(path) {
			return nil, fmt.Errorf("file %s is not a GraphQL file (must have .graphql or .gql extension)", path)
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", path, err)
		}
		files = append(files, f)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no GraphQL files found in %s", path)
	}

	return files, nil
}

// isGraphQLFile checks if a file has a GraphQL extension
func isGraphQLFile(path string) bool {
	return strings.HasSuffix(path, ".graphql") || strings.HasSuffix(path, ".gql")
}

// closeFiles closes all files in the slice
func closeFiles(files []*os.File) {
	for _, f := range files {
		if f != nil {
			_ = f.Close()
		}
	}
}
