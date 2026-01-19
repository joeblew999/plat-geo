// Package editor contains Datastar SSE handlers for the editor UI.
package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/plat-geo/internal/service"
	"github.com/joeblew999/plat-geo/internal/templates"
)

// LayerHandler handles layer-related SSE endpoints.
type LayerHandler struct {
	layerService *service.LayerService
	renderer     *templates.Renderer
}

// NewLayerHandler creates a new layer handler.
func NewLayerHandler(layerService *service.LayerService, renderer *templates.Renderer) *LayerHandler {
	return &LayerHandler{
		layerService: layerService,
		renderer:     renderer,
	}
}

// RegisterRoutes registers layer editor routes with Huma.
func (h *LayerHandler) RegisterRoutes(api huma.API) {
	huma.Get(api, "/api/v1/editor/layers", h.ListLayers)
	huma.Post(api, "/api/v1/editor/layers", h.CreateLayer)
	huma.Delete(api, "/api/v1/editor/layers/{id}", h.DeleteLayer)
}

// ListLayers streams the layer list as SSE HTML fragments.
func (h *LayerHandler) ListLayers(ctx context.Context, input *EmptyInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSEContext(humaCtx)

			layers := h.layerService.List()
			html := h.renderLayerList(layers)

			sse.PatchElements(html, "#layer-list")
		},
	}, nil
}

// CreateLayer creates a new layer from Datastar signals and streams updated list.
// Signal names are generated - see signals_gen.go for LayerConfigSignalNames.
func (h *LayerHandler) CreateLayer(ctx context.Context, input *SignalsInput) (*huma.StreamResponse, error) {
	// Parse Datastar signals from request body
	signals, err := input.MustParse()
	if err != nil {
		return nil, err
	}

	// Use generated parser - type-safe signal → struct mapping
	config := ParseLayerConfigSignals(signals)

	// Validate required fields
	if config.Name == "" {
		return nil, huma.Error400BadRequest("Layer name is required")
	}
	if config.File == "" {
		return nil, huma.Error400BadRequest("PMTiles file is required")
	}
	if config.GeomType == "" {
		return nil, huma.Error400BadRequest("Geometry type is required")
	}

	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSEContext(humaCtx)

			created, err := h.layerService.Create(config)
			if err != nil {
				sse.SendError(err.Error())
				return
			}

			// Reset form signals and show success
			// Use generated reset - ensures signal names match parser
			resetSignals := ResetLayerConfigSignals()
			resetSignals["success"] = fmt.Sprintf("Layer '%s' created", created.Name)
			resetSignals["_editingLayer"] = false
			sse.SendSignals(resetSignals)

			layers := h.layerService.List()
			html := h.renderLayerList(layers)
			sse.PatchElements(html, "#layer-list")
		},
	}, nil
}

// DeleteLayerInput is the input for deleting a layer.
type DeleteLayerInput struct {
	ID string `path:"id" doc:"Layer ID to delete"`
}

// DeleteLayer deletes a layer and streams updated list.
func (h *LayerHandler) DeleteLayer(ctx context.Context, input *DeleteLayerInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSEContext(humaCtx)

			if err := h.layerService.Delete(input.ID); err != nil {
				sse.SendError(err.Error())
				return
			}

			sse.SendSuccess("Layer deleted")

			layers := h.layerService.List()
			html := h.renderLayerList(layers)
			sse.PatchElements(html, "#layer-list")
		},
	}, nil
}

// LayerCardData holds data for rendering a layer card template.
type LayerCardData struct {
	ID         string
	Name       string
	File       string
	GeomType   string
	ConfigJSON template.JS
}

func (h *LayerHandler) renderLayerList(layers map[string]service.LayerConfig) string {
	var buf bytes.Buffer

	if len(layers) == 0 {
		if err := h.renderer.RenderToBuffer(&buf, "empty-state", map[string]string{
			"Title":   "No layers configured",
			"Message": "Add a layer to get started",
		}); err != nil {
			return "<!-- template error: " + err.Error() + " -->"
		}
	} else {
		for id, layer := range layers {
			configJSON, _ := json.Marshal(map[string]any{
				"file":         layer.File,
				"pmtilesLayer": layer.PMTilesLayer,
				"geomType":     layer.GeomType,
				"fill":         layer.Fill,
				"stroke":       layer.Stroke,
				"opacity":      layer.Opacity,
			})

			if err := h.renderer.RenderToBuffer(&buf, "layer-card", LayerCardData{
				ID:         id,
				Name:       layer.Name,
				File:       layer.File,
				GeomType:   layer.GeomType,
				ConfigJSON: template.JS(configJSON),
			}); err != nil {
				buf.WriteString("<!-- template error: " + err.Error() + " -->")
			}
		}
	}

	return buf.String()
}
