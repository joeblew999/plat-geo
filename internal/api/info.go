package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
)

// InfoHandler handles server info endpoints.
type InfoHandler struct {
	dataDir string
	dbOK    bool
}

// NewInfoHandler creates a new info handler.
func NewInfoHandler(dataDir string, dbOK bool) *InfoHandler {
	return &InfoHandler{
		dataDir: dataDir,
		dbOK:    dbOK,
	}
}

// RegisterRoutes registers info routes with Huma.
// Note: Root "/" endpoint is handled by server.go to avoid ServeMux conflicts
func (h *InfoHandler) RegisterRoutes(api huma.API) {
	huma.Get(api, "/api/v1/info", h.GetInfo)
}

// InfoOutput is the response for server info.
type InfoOutput struct {
	Body struct {
		Name     string   `json:"name" doc:"Service name"`
		Version  string   `json:"version" doc:"Service version"`
		DataDir  string   `json:"data_dir" doc:"Data directory path"`
		DB       bool     `json:"db" doc:"Whether database is available"`
		Features []string `json:"features" doc:"Available features"`
	}
}

// GetInfo returns server information.
func (h *InfoHandler) GetInfo(ctx context.Context, input *struct{}) (*InfoOutput, error) {
	return &InfoOutput{
		Body: struct {
			Name     string   `json:"name" doc:"Service name"`
			Version  string   `json:"version" doc:"Service version"`
			DataDir  string   `json:"data_dir" doc:"Data directory path"`
			DB       bool     `json:"db" doc:"Whether database is available"`
			Features []string `json:"features" doc:"Available features"`
		}{
			Name:    "plat-geo",
			Version: "0.1.0",
			DataDir: h.dataDir,
			DB:      h.dbOK,
			Features: []string{
				"geoparquet",
				"pmtiles",
				"routing",
				"duckdb",
			},
		},
	}, nil
}
