// Package api defines the Huma API routes and handlers.
package api

import (
	"context"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/joeblew999/plat-geo/internal/humastar"
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

type ListInput struct {
	Limit  int `query:"limit" default:"20" minimum:"1" maximum:"100" doc:"Items per page"`
	Offset int `query:"offset" default:"0" minimum:"0" doc:"Items to skip"`
}

// LayerBody wraps LayerConfig with state-dependent hypermedia actions.
type LayerBody struct {
	service.LayerConfig
}

// layerActions defines the action templates for layer resources.
var layerActions = []humastar.ActionDef{
	{Rel: "duplicate", Pattern: "/api/v1/layers/%s/duplicate", Method: "POST", Title: "Duplicate", Schema: "/schemas/DuplicateInput.json"},
	{Rel: "delete", Pattern: "/api/v1/layers/%s", Method: "DELETE", Title: "Delete"},
}

// Actions implements humastar.Actor — emits state-dependent hypermedia actions.
func (b LayerBody) Actions() []humastar.Action {
	actions := humastar.ActionsFor(b.ID, layerActions)
	// State-dependent: publish or unpublish
	if b.Published {
		actions = append(actions, humastar.Action{
			Rel: "unpublish", Href: fmt.Sprintf("/api/v1/layers/%s/unpublish", b.ID),
			Method: "POST", Title: "Unpublish",
		})
	} else {
		actions = append(actions, humastar.Action{
			Rel: "publish", Href: fmt.Sprintf("/api/v1/layers/%s/publish", b.ID),
			Method: "POST", Title: "Publish",
		})
	}
	// Cross-resource link to tiles
	actions = append(actions, humastar.Action{
		Rel: "related", Href: "/api/v1/tiles", Title: "Tile Files",
	})
	return actions
}

type LayerOutput struct {
	Body LayerBody
}

type DuplicateInput struct {
	Name string `json:"name" required:"true" minLength:"1" maxLength:"100" doc:"Name for the duplicate layer"`
}

type StyleIDInput struct {
	ID      string `path:"id" doc:"Layer ID"`
	StyleID string `path:"styleId" doc:"Style name"`
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
	huma.Post(api, "/api/v1/layers/{id}/duplicate", h.DuplicateLayer, huma.OperationTags("layers"))
	huma.Post(api, "/api/v1/layers/{id}/publish", h.PublishLayer, huma.OperationTags("layers"))
	huma.Post(api, "/api/v1/layers/{id}/unpublish", h.UnpublishLayer, huma.OperationTags("layers"))
	huma.Get(api, "/api/v1/layers/{id}/styles", h.GetStyles, huma.OperationTags("layers"))
	huma.Post(api, "/api/v1/layers/{id}/styles", h.AddStyle, huma.OperationTags("layers"))
	huma.Delete(api, "/api/v1/layers/{id}/styles/{styleId}", h.DeleteStyle, huma.OperationTags("layers"))
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
	return &LayerOutput{Body: LayerBody{layer}}, nil
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
	return &LayerOutput{Body: LayerBody{updated}}, nil
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

func (h *APIHandler) DuplicateLayer(ctx context.Context, input *struct {
	IDInput
	Body DuplicateInput
}) (*struct{ Body CreatedLayerBody }, error) {
	if h.svc == nil || h.svc.Layer == nil {
		return nil, huma.Error400BadRequest("service not available")
	}
	dup, err := h.svc.Layer.Duplicate(input.ID, input.Body.Name)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	return &struct{ Body CreatedLayerBody }{Body: CreatedLayerBody{
		ID: dup.ID, Layer: dup, Message: "Layer duplicated",
	}}, nil
}

func (h *APIHandler) GetSources(ctx context.Context, input *ListInput) (*struct {
	Body humastar.PageBody[service.SourceFile]
}, error) {
	if h.svc == nil || h.svc.Source == nil {
		return &struct{ Body humastar.PageBody[service.SourceFile] }{}, nil
	}
	items, total, err := h.svc.Source.ListPaged(input.Offset, input.Limit)
	if err != nil {
		return &struct{ Body humastar.PageBody[service.SourceFile] }{}, nil
	}
	return &struct{ Body humastar.PageBody[service.SourceFile] }{Body: humastar.PageBody[service.SourceFile]{
		Total: total, Offset: input.Offset, Limit: input.Limit,
		Data: items,
	}}, nil
}

func (h *APIHandler) GetTiles(ctx context.Context, input *ListInput) (*struct {
	Body humastar.PageBody[service.TileFile]
}, error) {
	if h.svc == nil || h.svc.Tile == nil {
		return &struct{ Body humastar.PageBody[service.TileFile] }{}, nil
	}
	items, total, err := h.svc.Tile.ListPaged(input.Offset, input.Limit)
	if err != nil {
		return &struct{ Body humastar.PageBody[service.TileFile] }{}, nil
	}
	return &struct{ Body humastar.PageBody[service.TileFile] }{Body: humastar.PageBody[service.TileFile]{
		Total: total, Offset: input.Offset, Limit: input.Limit,
		Data: items,
	}}, nil
}

func (h *APIHandler) PublishLayer(ctx context.Context, input *IDInput) (*LayerOutput, error) {
	if h.svc == nil || h.svc.Layer == nil {
		return nil, huma.Error400BadRequest("service not available")
	}
	layer, err := h.svc.Layer.Publish(input.ID)
	if err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}
	return &LayerOutput{Body: LayerBody{layer}}, nil
}

func (h *APIHandler) UnpublishLayer(ctx context.Context, input *IDInput) (*LayerOutput, error) {
	if h.svc == nil || h.svc.Layer == nil {
		return nil, huma.Error400BadRequest("service not available")
	}
	layer, err := h.svc.Layer.Unpublish(input.ID)
	if err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}
	return &LayerOutput{Body: LayerBody{layer}}, nil
}

func (h *APIHandler) GetStyles(ctx context.Context, input *IDInput) (*struct {
	Body []service.Style
}, error) {
	if h.svc == nil || h.svc.Layer == nil {
		return nil, huma.Error400BadRequest("service not available")
	}
	styles, err := h.svc.Layer.ListStyles(input.ID)
	if err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}
	return &struct{ Body []service.Style }{Body: styles}, nil
}

func (h *APIHandler) AddStyle(ctx context.Context, input *struct {
	IDInput
	Body service.Style
}) (*struct{ Body service.Style }, error) {
	if h.svc == nil || h.svc.Layer == nil {
		return nil, huma.Error400BadRequest("service not available")
	}
	style, err := h.svc.Layer.AddStyle(input.ID, input.Body)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	return &struct{ Body service.Style }{Body: style}, nil
}

func (h *APIHandler) DeleteStyle(ctx context.Context, input *StyleIDInput) (*struct{ Body MessageBody }, error) {
	if h.svc == nil || h.svc.Layer == nil {
		return nil, huma.Error400BadRequest("service not available")
	}
	if err := h.svc.Layer.DeleteStyle(input.ID, input.StyleID); err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}
	return &struct{ Body MessageBody }{Body: MessageBody{Message: "Style deleted"}}, nil
}
