package index

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SupportedExtensions lists file extensions that can be indexed.
var SupportedExtensions = map[string]bool{
	".txt":  true,
	".md":   true,
	".log":  true,
	".csv":  true,
	".json": true,
	".xml":  true,
	".html": true,
	".htm":  true,
	".yaml": true,
	".yml":  true,
	".toml": true,
	".cfg":  true,
	".conf": true,
	".ini":  true,
}

// IndexDirectory walks a directory and indexes all supported text files.
// Returns the number of documents indexed.
func IndexDirectory(idx *InvertedIndex, dirPath string) (int, error) {
	count := 0
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !SupportedExtensions[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		// Skip files larger than 10MB.
		if info.Size() > 10*1024*1024 {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		idx.AddDocument(path, content)
		count++
		return nil
	})
	if err != nil {
		return count, fmt.Errorf("walk %s: %w", dirPath, err)
	}
	return count, nil
}
