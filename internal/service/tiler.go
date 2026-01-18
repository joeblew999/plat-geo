package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// TilerService handles tile generation using Tippecanoe.
type TilerService struct {
	sourcesDir string
	tilesDir   string
}

// NewTilerService creates a new tiler service.
func NewTilerService(dataDir string) *TilerService {
	return &TilerService{
		sourcesDir: filepath.Join(dataDir, "sources"),
		tilesDir:   filepath.Join(dataDir, "tiles"),
	}
}

// TileGenerateOptions contains options for tile generation.
type TileGenerateOptions struct {
	SourceFile string `json:"sourceFile" required:"true" doc:"Source file name"`
	OutputName string `json:"outputName" required:"true" doc:"Output PMTiles name"`
	LayerName  string `json:"layerName" doc:"Layer name in tiles"`
	MinZoom    int    `json:"minZoom" minimum:"0" maximum:"22" doc:"Minimum zoom level"`
	MaxZoom    int    `json:"maxZoom" minimum:"0" maximum:"22" doc:"Maximum zoom level"`
}

// ProgressFunc is called with progress updates during tile generation.
type ProgressFunc func(progress int, status string)

// Generate creates PMTiles from a source file using Tippecanoe.
func (s *TilerService) Generate(ctx context.Context, opts TileGenerateOptions, onProgress ProgressFunc) error {
	// Apply defaults
	if opts.LayerName == "" {
		opts.LayerName = "default"
	}
	if opts.MinZoom == 0 && opts.MaxZoom == 0 {
		opts.MaxZoom = 14
	}

	// Ensure output has .pmtiles extension
	if !strings.HasSuffix(opts.OutputName, ".pmtiles") {
		opts.OutputName = opts.OutputName + ".pmtiles"
	}

	sourcePath := filepath.Join(s.sourcesDir, opts.SourceFile)
	outputPath := filepath.Join(s.tilesDir, opts.OutputName)

	// Validate source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source file not found: %s", opts.SourceFile)
	}

	// Ensure tiles directory exists
	if err := os.MkdirAll(s.tilesDir, 0755); err != nil {
		return fmt.Errorf("failed to create tiles directory: %w", err)
	}

	if onProgress != nil {
		onProgress(10, "Starting tile generation...")
	}

	args := []string{
		"-o", outputPath,
		"-l", opts.LayerName,
		"-Z", strconv.Itoa(opts.MinZoom),
		"-z", strconv.Itoa(opts.MaxZoom),
		"--force",
		"--drop-densest-as-needed",
		sourcePath,
	}

	if onProgress != nil {
		onProgress(30, "Running Tippecanoe...")
	}

	cmd := exec.CommandContext(ctx, "tippecanoe", args...)

	// Capture stderr for progress/errors
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return fmt.Errorf("tippecanoe is not installed. Run 'task tippecanoe:install' to install it")
		}
		return fmt.Errorf("failed to start tippecanoe: %w", err)
	}

	// Read stderr for progress updates
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		// Tippecanoe outputs progress like "99.9%  11/14"
		if strings.Contains(line, "%") && onProgress != nil {
			// Extract percentage if possible
			parts := strings.Fields(line)
			if len(parts) > 0 {
				pctStr := strings.TrimSuffix(parts[0], "%")
				if pct, err := strconv.ParseFloat(pctStr, 64); err == nil {
					// Scale 0-100% to 30-90% of our progress
					progress := 30 + int(pct*0.6)
					onProgress(progress, fmt.Sprintf("Processing: %s%%", pctStr))
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("tile generation failed: %w", err)
	}

	if onProgress != nil {
		onProgress(100, "Tiles generated successfully!")
	}

	return nil
}

// SourcesDir returns the sources directory path.
func (s *TilerService) SourcesDir() string {
	return s.sourcesDir
}

// TilesDir returns the tiles directory path.
func (s *TilerService) TilesDir() string {
	return s.tilesDir
}

// ValidateSourceFile checks if a source file exists and has a valid extension.
func (s *TilerService) ValidateSourceFile(filename string) error {
	// Check for path traversal
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		return fmt.Errorf("invalid filename")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	validExts := map[string]bool{
		".geojson":    true,
		".json":       true,
		".parquet":    true,
		".geoparquet": true,
	}
	if !validExts[ext] {
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	sourcePath := filepath.Join(s.sourcesDir, filename)
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filename)
	}

	return nil
}
