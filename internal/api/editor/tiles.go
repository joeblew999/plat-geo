package editor

import (
	"bytes"
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/plat-geo/internal/service"
	"github.com/joeblew999/plat-geo/internal/templates"
)

// TileHandler handles tile-related SSE endpoints.
type TileHandler struct {
	tileService *service.TileService
	renderer    *templates.Renderer
}

// NewTileHandler creates a new tile handler.
func NewTileHandler(tileService *service.TileService, renderer *templates.Renderer) *TileHandler {
	return &TileHandler{
		tileService: tileService,
		renderer:    renderer,
	}
}

// RegisterRoutes registers tile editor routes with Huma.
func (h *TileHandler) RegisterRoutes(api huma.API) {
	huma.Get(api, "/api/v1/editor/tiles", h.ListTiles)
	huma.Get(api, "/api/v1/editor/tiles/select", h.ListTilesSelect)
}

// ListTiles streams the tile list as SSE HTML fragments.
func (h *TileHandler) ListTiles(ctx context.Context, input *EmptyInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSEContext(humaCtx)

			tiles, err := h.tileService.List()
			if err != nil {
				sse.SendError("Failed to list tiles: " + err.Error())
				return
			}

			html := h.renderTileList(tiles)
			sse.PatchElements(html, "#tile-list")
		},
	}, nil
}

// ListTilesSelect streams tiles as select options.
func (h *TileHandler) ListTilesSelect(ctx context.Context, input *EmptyInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSEContext(humaCtx)

			tiles, err := h.tileService.List()
			if err != nil {
				sse.SendError("Failed to list tiles: " + err.Error())
				return
			}

			html := h.renderTileSelect(tiles)
			sse.PatchElements(html, "#pmtiles-select")
		},
	}, nil
}

// TileCardData holds data for rendering a tile card template.
type TileCardData struct {
	Name string
	Size string
}

func (h *TileHandler) renderTileList(tiles []service.TileFile) string {
	var buf bytes.Buffer

	if len(tiles) == 0 {
		if err := h.renderer.RenderToBuffer(&buf, "empty-state", map[string]string{
			"Title":   "No PMTiles Found",
			"Message": "Upload GeoJSON files and generate tiles, or add .pmtiles files to .data/tiles/",
		}); err != nil {
			return "<!-- template error: " + err.Error() + " -->"
		}
	} else {
		for _, tile := range tiles {
			if err := h.renderer.RenderToBuffer(&buf, "tile-card", TileCardData{
				Name: tile.Name,
				Size: tile.Size,
			}); err != nil {
				buf.WriteString("<!-- template error: " + err.Error() + " -->")
			}
		}
	}

	return buf.String()
}

func (h *TileHandler) renderTileSelect(tiles []service.TileFile) string {
	var buf bytes.Buffer

	if err := h.renderer.RenderToBuffer(&buf, "select-option", SelectOptionData{
		Value: "",
		Label: "-- Select a PMTiles file --",
	}); err != nil {
		return "<!-- template error: " + err.Error() + " -->"
	}

	for _, tile := range tiles {
		if err := h.renderer.RenderToBuffer(&buf, "select-option", SelectOptionData{
			Value: tile.Name,
			Label: tile.Name + " (" + tile.Size + ")",
		}); err != nil {
			buf.WriteString("<!-- template error: " + err.Error() + " -->")
		}
	}

	return buf.String()
}
