package service

import (
	"os"
	"path/filepath"
	"strings"
)

// SourceService manages source data files.
type SourceService struct {
	sourcesDir string
}

// NewSourceService creates a new source service.
func NewSourceService(dataDir string) *SourceService {
	return &SourceService{
		sourcesDir: filepath.Join(dataDir, "sources"),
	}
}

// List returns all available source files.
func (s *SourceService) List() ([]SourceFile, error) {
	entries, err := os.ReadDir(s.sourcesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SourceFile{}, nil
		}
		return nil, err
	}

	// Supported source file extensions and their types
	extToType := map[string]string{
		".geojson":    "GeoJSON",
		".json":       "GeoJSON",
		".csv":        "CSV",
		".gpkg":       "GeoPackage",
		".shp":        "Shapefile",
		".parquet":    "GeoParquet",
		".geoparquet": "GeoParquet",
	}

	var files []SourceFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		fileType, ok := extToType[ext]
		if !ok {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, SourceFile{
			Name:     entry.Name(),
			Size:     formatSize(info.Size()),
			FileType: fileType,
		})
	}

	return files, nil
}

// SourcesDir returns the path to the sources directory.
func (s *SourceService) SourcesDir() string {
	return s.sourcesDir
}
