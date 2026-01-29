package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
)

type InfoHandler struct {
	dataDir string
	dbOK    bool
}

func NewInfoHandler(dataDir string, dbOK bool) *InfoHandler {
	return &InfoHandler{dataDir: dataDir, dbOK: dbOK}
}

func (h *InfoHandler) RegisterRoutes(api huma.API) {
	huma.Get(api, "/api/v1/info", h.GetInfo, huma.OperationTags("health"))
}

type InfoBody struct {
	Name     string   `json:"name" doc:"Service name"`
	Version  string   `json:"version" doc:"Service version"`
	DataDir  string   `json:"data_dir" doc:"Data directory path"`
	DB       bool     `json:"db" doc:"Whether database is available"`
	Features []string `json:"features" doc:"Available features"`
}

func (h *InfoHandler) GetInfo(ctx context.Context, input *struct{}) (*struct{ Body InfoBody }, error) {
	return &struct{ Body InfoBody }{Body: InfoBody{
		Name:     "plat-geo",
		Version:  "0.1.0",
		DataDir:  h.dataDir,
		DB:       h.dbOK,
		Features: []string{"geoparquet", "pmtiles", "routing", "duckdb"},
	}}, nil
}
