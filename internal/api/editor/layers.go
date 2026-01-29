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

type LayerHandler struct {
	layerService *service.LayerService
	renderer     *templates.Renderer
}

func NewLayerHandler(layerService *service.LayerService, renderer *templates.Renderer) *LayerHandler {
	return &LayerHandler{layerService: layerService, renderer: renderer}
}

func (h *LayerHandler) RegisterRoutes(api huma.API) {
	huma.Get(api, "/api/v1/editor/layers", h.ListLayers, huma.OperationTags("editor"))
	huma.Post(api, "/api/v1/editor/layers", h.CreateLayer, huma.OperationTags("editor"))
	huma.Delete(api, "/api/v1/editor/layers/{id}", h.DeleteLayer, huma.OperationTags("editor"))
}

func (h *LayerHandler) ListLayers(ctx context.Context, input *EmptyInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSE(humaCtx)
			sse.Patch(h.renderLayerList(h.layerService.List()), "#layer-list")
		},
	}, nil
}

func (h *LayerHandler) CreateLayer(ctx context.Context, input *SignalsInput) (*huma.StreamResponse, error) {
	signals, err := input.MustParse()
	if err != nil {
		return nil, err
	}
	config := ParseLayerConfigSignals(signals)

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
			sse := NewSSE(humaCtx)

			created, err := h.layerService.Create(config)
			if err != nil {
				sse.Error(err.Error())
				return
			}

			resetSignals := ResetLayerConfigSignals()
			resetSignals["success"] = fmt.Sprintf("Layer '%s' created", created.Name)
			resetSignals["_editingLayer"] = false
			sse.Signals(resetSignals)

			sse.Patch(h.renderLayerList(h.layerService.List()), "#layer-list")
			sse.DispatchCustomEvent("layer-changed", map[string]any{
				"action": "created", "id": created.ID, "name": created.Name,
			})
		},
	}, nil
}

type DeleteLayerInput struct {
	ID string `path:"id" doc:"Layer ID to delete"`
}

func (h *LayerHandler) DeleteLayer(ctx context.Context, input *DeleteLayerInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			sse := NewSSE(humaCtx)

			if err := h.layerService.Delete(input.ID); err != nil {
				sse.Error(err.Error())
				return
			}

			sse.RemoveElementByID("layer-" + input.ID)
			sse.Success("Layer deleted")
			sse.DispatchCustomEvent("layer-changed", map[string]any{
				"action": "deleted", "id": input.ID,
			})
		},
	}, nil
}

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
		h.renderer.RenderToBuffer(&buf, "empty-state", map[string]string{
			"Title": "No layers configured", "Message": "Add a layer to get started",
		})
	} else {
		for id, layer := range layers {
			configJSON, _ := json.Marshal(map[string]any{
				"file": layer.File, "pmtilesLayer": layer.PMTilesLayer,
				"geomType": layer.GeomType, "fill": layer.Fill,
				"stroke": layer.Stroke, "opacity": layer.Opacity,
			})
			h.renderer.RenderToBuffer(&buf, "layer-card", LayerCardData{
				ID: id, Name: layer.Name, File: layer.File,
				GeomType: layer.GeomType, ConfigJSON: template.JS(configJSON),
			})
		}
	}
	return buf.String()
}
