// Package tiler wraps tippecanoe for tile generation.
package tiler

import (
	"fmt"
	"os/exec"
)

// Tippecanoe implements Tiler using the tippecanoe CLI.
type Tippecanoe struct{}

// NewTippecanoe creates a new Tippecanoe tiler.
func NewTippecanoe() *Tippecanoe {
	return &Tippecanoe{}
}

// Name returns the engine name.
func (t *Tippecanoe) Name() string {
	return "tippecanoe"
}

// Available checks if tippecanoe is installed.
func (t *Tippecanoe) Available() bool {
	_, err := exec.LookPath("tippecanoe")
	return err == nil
}

// Tile converts GeoJSON to PMTiles using tippecanoe.
func (t *Tippecanoe) Tile(inputPath, outputPath string, config TileConfig) error {
	if !t.Available() {
		return fmt.Errorf("tippecanoe not found in PATH")
	}

	args := []string{
		"-o", outputPath,
		"--force",
	}

	// Layer name
	if config.Layer != "" {
		args = append(args, "--layer="+config.Layer)
	}

	// Zoom settings
	if config.MinZoom >= 0 && config.MaxZoom >= 0 {
		args = append(args, fmt.Sprintf("-Z%d", config.MinZoom))
		args = append(args, fmt.Sprintf("-z%d", config.MaxZoom))
	} else {
		args = append(args, "-zg") // Auto-detect zoom
	}

	// Feature handling
	if config.ReduceRate > 0 {
		args = append(args, fmt.Sprintf("-r%d", config.ReduceRate))
	}
	if config.DropDensest {
		args = append(args, "--drop-densest-as-needed")
	}
	if config.NoFeatureLimit {
		args = append(args, "--no-feature-limit")
	}
	if config.NoTileSizeLimit {
		args = append(args, "--no-tile-size-limit")
	}

	args = append(args, inputPath)

	cmd := exec.Command("tippecanoe", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tippecanoe failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// Ensure Tippecanoe implements Tiler.
var _ Tiler = (*Tippecanoe)(nil)
