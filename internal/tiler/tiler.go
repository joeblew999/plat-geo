// Package tiler provides tile generation for geospatial data.
package tiler

// TileConfig holds settings for tile generation.
type TileConfig struct {
	MinZoom         int
	MaxZoom         int
	Layer           string // Layer name in the tiles
	DropDensest     bool   // Drop features in dense tiles
	NoFeatureLimit  bool   // Don't limit features per tile
	NoTileSizeLimit bool   // Don't limit tile size
	ReduceRate      int    // Feature reduction rate (tippecanoe -r flag, 0 = default)
}

// Tiler generates PMTiles from GeoJSON.
type Tiler interface {
	// Tile converts a GeoJSON file to PMTiles.
	// Returns the output path and any error.
	Tile(inputPath, outputPath string, config TileConfig) error

	// Name returns the engine name (e.g., "tippecanoe", "go").
	Name() string

	// Available returns true if this tiler can be used.
	Available() bool
}
