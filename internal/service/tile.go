package service

import (
	"fmt"
	"os"
	"path/filepath"
)

// TileService manages PMTiles files.
type TileService struct {
	tilesDir string
}

// NewTileService creates a new tile service.
func NewTileService(dataDir string) *TileService {
	return &TileService{
		tilesDir: filepath.Join(dataDir, "tiles"),
	}
}

// List returns all available PMTiles files.
func (s *TileService) List() ([]TileFile, error) {
	entries, err := os.ReadDir(s.tilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []TileFile{}, nil
		}
		return nil, err
	}

	var files []TileFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".pmtiles" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, TileFile{
			Name: entry.Name(),
			Size: formatSize(info.Size()),
		})
	}

	return files, nil
}

// TilesDir returns the path to the tiles directory.
func (s *TileService) TilesDir() string {
	return s.tilesDir
}

// formatSize returns a human-readable file size.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
