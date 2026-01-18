package service

import (
	"fmt"
	"io"
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

// ValidExtensions returns the valid source file extensions.
var ValidExtensions = map[string]bool{
	".geojson":    true,
	".json":       true,
	".parquet":    true,
	".geoparquet": true,
}

// ValidateFilename checks if a filename is valid for upload.
func (s *SourceService) ValidateFilename(filename string) error {
	// Check for path traversal
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		return fmt.Errorf("invalid filename")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if !ValidExtensions[ext] {
		return fmt.Errorf("only .geojson, .json, .parquet, or .geoparquet files are allowed")
	}

	return nil
}

// Save saves content to a file in the sources directory.
func (s *SourceService) Save(filename string, content io.Reader) error {
	if err := s.ValidateFilename(filename); err != nil {
		return err
	}

	// Ensure sources directory exists
	if err := os.MkdirAll(s.sourcesDir, 0755); err != nil {
		return fmt.Errorf("failed to create sources directory: %w", err)
	}

	destPath := filepath.Join(s.sourcesDir, filename)
	dest, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, content); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Delete removes a source file.
func (s *SourceService) Delete(filename string) error {
	// Check for path traversal
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		return fmt.Errorf("invalid filename")
	}

	filePath := filepath.Join(s.sourcesDir, filename)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", filename)
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
