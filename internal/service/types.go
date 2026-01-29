// Package service contains business logic for the plat-geo platform.
package service

// LayerConfig represents a map layer configuration.
// Single source of truth: Huma reads tags for OpenAPI + validation,
// cmd/gen reads tags for Datastar signal helpers + HTML forms.
//
// Custom tags for codegen:
//
//	signal:"name"              — override Datastar signal suffix (default: lowercase field name)
//	input:"color|sse"          — force input type (color picker, SSE-loaded select)
//	sse:"/url,element-id"      — SSE endpoint + target element ID for dynamic selects
type LayerConfig struct {
	ID             string       `json:"id,omitempty" doc:"Unique layer identifier" example:"buildings"`
	Name           string       `json:"name" required:"true" minLength:"1" maxLength:"100" doc:"Display name" example:"Buildings"`
	File           string       `json:"file" required:"true" doc:"Source file name" example:"buildings.pmtiles" input:"sse" sse:"/api/v1/editor/tiles/select,pmtiles-select"`
	PMTilesLayer   string       `json:"pmtilesLayer,omitempty" doc:"Layer name within PMTiles" example:"buildings" default:"default"`
	GeomType       string       `json:"geomType" required:"true" enum:"polygon,line,point" doc:"Geometry type" example:"polygon" default:"polygon"`
	DefaultVisible bool         `json:"defaultVisible" default:"true" doc:"Whether layer is visible by default" example:"true" signal:"visible"`
	Fill           string       `json:"fill,omitempty" doc:"Fill color (CSS)" example:"#3388ff" default:"#3388ff" input:"color"`
	Stroke         string       `json:"stroke,omitempty" doc:"Stroke color (CSS)" example:"#2266cc" default:"#2266cc" input:"color"`
	Opacity        float64      `json:"opacity,omitempty" minimum:"0" maximum:"1" default:"0.7" doc:"Layer opacity (0-1)" example:"0.7"`
	RenderRules    []RenderRule `json:"renderRules,omitempty" doc:"Conditional styling rules"`
	Legend         []LegendItem `json:"legend,omitempty" doc:"Legend entries for this layer"`
}

// RenderRule defines conditional styling rules for a layer.
type RenderRule struct {
	FilterProp  string  `json:"filterProp,omitempty" doc:"Property name to filter on"`
	FilterValue string  `json:"filterValue,omitempty" doc:"Value to match"`
	Fill        string  `json:"fill" doc:"Fill color (CSS)"`
	Stroke      string  `json:"stroke,omitempty" doc:"Stroke color (CSS)"`
	Opacity     float64 `json:"opacity,omitempty" doc:"Opacity (0-1)"`
	Width       float64 `json:"width,omitempty" doc:"Line width"`
	Radius      float64 `json:"radius,omitempty" doc:"Point radius"`
}

// LegendItem defines a legend entry.
type LegendItem struct {
	Label string `json:"label" doc:"Legend label"`
	Color string `json:"color" doc:"Legend color (CSS)"`
}

// SourceFile represents a source data file (GeoJSON, etc.).
type SourceFile struct {
	Name     string `json:"name" doc:"File name" example:"buildings.geojson"`
	Size     string `json:"size" doc:"Human-readable file size" example:"1.2 MB"`
	FileType string `json:"fileType" doc:"File type: GeoJSON or GeoParquet" example:"GeoJSON"`
}

// TileFile represents a PMTiles file.
type TileFile struct {
	Name string `json:"name" doc:"PMTiles file name" example:"buildings.pmtiles"`
	Size string `json:"size" doc:"Human-readable file size" example:"5.4 MB"`
}
