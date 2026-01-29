package editor

import (
	"bytes"
	"context"
	"fmt"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/plat-geo/internal/service"
	"github.com/joeblew999/plat-geo/internal/templates"
)

type TileHandler struct {
	tileService  *service.TileService
	tilerService *service.TilerService
	renderer     *templates.Renderer
}

func NewTileHandler(tileService *service.TileService, renderer *templates.Renderer) *TileHandler {
	return &TileHandler{tileService: tileService, renderer: renderer}
}

func (h *TileHandler) SetTilerService(tilerService *service.TilerService) {
	h.tilerService = tilerService
}

func (h *TileHandler) RegisterRoutes(api huma.API) {
	huma.Get(api, "/api/v1/editor/tiles", h.ListTiles, huma.OperationTags("editor"))
	huma.Get(api, "/api/v1/editor/tiles/select", h.ListTilesSelect, huma.OperationTags("editor"))
	huma.Post(api, "/api/v1/editor/tiles/generate", h.Generate, huma.OperationTags("editor"))
}

func (h *TileHandler) Generate(ctx context.Context, input *SignalsInput) (*huma.StreamResponse, error) {
	signals, err := input.MustParse()
	if err != nil {
		return nil, err
	}

	opts := service.TileGenerateOptions{
		SourceFile: signals.String("sourcefile"),
		OutputName: signals.String("outputname"),
		LayerName:  signals.String("layername"),
		MinZoom:    signals.Int("minzoom"),
		MaxZoom:    signals.Int("maxzoom"),
	}
	if opts.SourceFile == "" {
		return nil, huma.Error400BadRequest("Source file is required")
	}
	if opts.OutputName == "" {
		return nil, huma.Error400BadRequest("Output name is required")
	}

	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSE(humaCtx)

			if h.tilerService == nil {
				sse.Error("Tiler service not configured")
				return
			}

			sse.ConsoleLogf("Starting tile generation: %s → %s", opts.SourceFile, opts.OutputName)

			err := h.tilerService.Generate(ctx, opts, func(progress int, status string) {
				sse.Signals(map[string]any{
					"tileStatus":   status,
					"tileProgress": progress,
				})
				if sse.IsClosed() {
					return
				}
			})

			if err != nil {
				sse.ConsoleError(err)
				sse.Error(err.Error())
				return
			}

			sse.Signals(map[string]any{
				"tileStatus":   "Complete!",
				"tileProgress": 100,
				"success":      fmt.Sprintf("Tiles generated: %s", opts.OutputName),
			})

			if tiles, err := h.tileService.List(); err == nil {
				sse.Patch(h.renderTileList(tiles), "#tile-list")
				sse.Patch(h.renderTileSelect(tiles), "#pmtiles-select")
			}

			sse.DispatchCustomEvent("tiles-generated", map[string]any{
				"output": opts.OutputName, "source": opts.SourceFile,
			})
			sse.ConsoleLogf("Tile generation complete: %s", opts.OutputName)
		},
	}, nil
}

func (h *TileHandler) ListTiles(ctx context.Context, input *EmptyInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSE(humaCtx)
			tiles, err := h.tileService.List()
			if err != nil {
				sse.Error("Failed to list tiles: " + err.Error())
				return
			}
			sse.Patch(h.renderTileList(tiles), "#tile-list")
		},
	}, nil
}

func (h *TileHandler) ListTilesSelect(ctx context.Context, input *EmptyInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSE(humaCtx)
			tiles, err := h.tileService.List()
			if err != nil {
				sse.Error("Failed to list tiles: " + err.Error())
				return
			}
			sse.Patch(h.renderTileSelect(tiles), "#pmtiles-select")
		},
	}, nil
}

type TileCardData struct {
	Name string
	Size string
}

func (h *TileHandler) renderTileList(tiles []service.TileFile) string {
	var buf bytes.Buffer
	if len(tiles) == 0 {
		h.renderer.RenderToBuffer(&buf, "empty-state", map[string]string{
			"Title": "No PMTiles Found", "Message": "Upload GeoJSON files and generate tiles, or add .pmtiles files to .data/tiles/",
		})
	} else {
		for _, t := range tiles {
			h.renderer.RenderToBuffer(&buf, "tile-card", TileCardData{Name: t.Name, Size: t.Size})
		}
	}
	return buf.String()
}

func (h *TileHandler) renderTileSelect(tiles []service.TileFile) string {
	var buf bytes.Buffer
	h.renderer.RenderToBuffer(&buf, "select-option", SelectOptionData{Label: "-- Select a PMTiles file --"})
	for _, t := range tiles {
		h.renderer.RenderToBuffer(&buf, "select-option", SelectOptionData{
			Value: t.Name, Label: t.Name + " (" + t.Size + ")",
		})
	}
	return buf.String()
}
