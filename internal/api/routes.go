// Package api defines the Huma API routes and handlers.
package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/joeblew999/plat-geo/internal/service"
)

// Services holds the service dependencies for API handlers.
type Services struct {
	Layer  *service.LayerService
	Tile   *service.TileService
	Source *service.SourceService
}

// Types

type IDInput struct {
	ID string `path:"id" doc:"Layer ID" example:"buildings"`
}

type LayerOutput struct {
	Body service.LayerConfig
}

type LayersOutput struct {
	Body map[string]service.LayerConfig
}

type MessageBody struct {
	Message string `json:"message" doc:"Result message"`
}

type CreatedLayerBody struct {
	ID      string              `json:"id" doc:"Generated layer ID"`
	Layer   service.LayerConfig `json:"layer" doc:"Created layer configuration"`
	Message string              `json:"message" doc:"Result message"`
}

type HealthBody struct {
	Status  string `json:"status" doc:"Health status" example:"ok"`
	Version string `json:"version" doc:"API version" example:"1.0.0"`
}

// APIHandler holds all REST API handlers. Methods named Register* are
// auto-discovered by huma.AutoRegister.
type APIHandler struct {
	svc *Services
}

func NewAPIHandler(svc *Services) *APIHandler {
	return &APIHandler{svc: svc}
}

// RegisterHealth registers health check routes.
func (h *APIHandler) RegisterHealth(api huma.API) {
	huma.Get(api, "/health", h.GetHealth, huma.OperationTags("health"))
}

// RegisterLayers registers layer CRUD routes.
func (h *APIHandler) RegisterLayers(api huma.API) {
	huma.Get(api, "/api/v1/layers", h.GetLayers, huma.OperationTags("layers"))
	huma.Post(api, "/api/v1/layers", h.CreateLayer, huma.OperationTags("layers"))
	huma.Get(api, "/api/v1/layers/{id}", h.GetLayer, huma.OperationTags("layers"))
	huma.Put(api, "/api/v1/layers/{id}", h.PutLayer, huma.OperationTags("layers"))
	huma.Delete(api, "/api/v1/layers/{id}", h.DeleteLayer, huma.OperationTags("layers"))
}

// RegisterSources registers source listing routes.
func (h *APIHandler) RegisterSources(api huma.API) {
	huma.Get(api, "/api/v1/sources", h.GetSources, huma.OperationTags("sources"))
}

// RegisterTiles registers tile listing routes.
func (h *APIHandler) RegisterTiles(api huma.API) {
	huma.Get(api, "/api/v1/tiles", h.GetTiles, huma.OperationTags("tiles"))
}

// Handlers

func (h *APIHandler) GetHealth(ctx context.Context, input *struct{}) (*struct{ Body HealthBody }, error) {
	return &struct{ Body HealthBody }{Body: HealthBody{Status: "ok", Version: "1.0.0"}}, nil
}

func (h *APIHandler) GetLayers(ctx context.Context, input *struct{}) (*LayersOutput, error) {
	if h.svc == nil || h.svc.Layer == nil {
		return &LayersOutput{Body: map[string]service.LayerConfig{}}, nil
	}
	return &LayersOutput{Body: h.svc.Layer.List()}, nil
}

func (h *APIHandler) CreateLayer(ctx context.Context, input *struct{ Body service.LayerConfig }) (*struct{ Body CreatedLayerBody }, error) {
	if h.svc == nil || h.svc.Layer == nil {
		return nil, huma.Error400BadRequest("service not available")
	}
	created, err := h.svc.Layer.Create(input.Body)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	return &struct{ Body CreatedLayerBody }{Body: CreatedLayerBody{
		ID: created.ID, Layer: created, Message: "Layer created",
	}}, nil
}

func (h *APIHandler) GetLayer(ctx context.Context, input *IDInput) (*LayerOutput, error) {
	if h.svc == nil || h.svc.Layer == nil {
		return nil, huma.Error404NotFound("service not available")
	}
	layer, ok := h.svc.Layer.Get(input.ID)
	if !ok {
		return nil, huma.Error404NotFound("layer not found")
	}
	return &LayerOutput{Body: layer}, nil
}

func (h *APIHandler) PutLayer(ctx context.Context, input *struct {
	IDInput
	Body service.LayerConfig
}) (*LayerOutput, error) {
	if h.svc == nil || h.svc.Layer == nil {
		return nil, huma.Error400BadRequest("service not available")
	}
	updated, err := h.svc.Layer.Update(input.ID, input.Body)
	if err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}
	return &LayerOutput{Body: updated}, nil
}

func (h *APIHandler) DeleteLayer(ctx context.Context, input *IDInput) (*struct{ Body MessageBody }, error) {
	if h.svc == nil || h.svc.Layer == nil {
		return nil, huma.Error400BadRequest("service not available")
	}
	if err := h.svc.Layer.Delete(input.ID); err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}
	return &struct{ Body MessageBody }{Body: MessageBody{Message: "Layer deleted"}}, nil
}

func (h *APIHandler) GetSources(ctx context.Context, input *struct{}) (*struct{ Body []service.SourceFile }, error) {
	if h.svc == nil || h.svc.Source == nil {
		return &struct{ Body []service.SourceFile }{Body: []service.SourceFile{}}, nil
	}
	sources, err := h.svc.Source.List()
	if err != nil {
		return &struct{ Body []service.SourceFile }{Body: []service.SourceFile{}}, nil
	}
	return &struct{ Body []service.SourceFile }{Body: sources}, nil
}

func (h *APIHandler) GetTiles(ctx context.Context, input *struct{}) (*struct{ Body []service.TileFile }, error) {
	if h.svc == nil || h.svc.Tile == nil {
		return &struct{ Body []service.TileFile }{Body: []service.TileFile{}}, nil
	}
	tiles, err := h.svc.Tile.List()
	if err != nil {
		return &struct{ Body []service.TileFile }{Body: []service.TileFile{}}, nil
	}
	return &struct{ Body []service.TileFile }{Body: tiles}, nil
}
