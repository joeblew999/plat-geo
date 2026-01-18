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

var svc *Services

// RegisterRoutes registers all API routes with the Huma API.
func RegisterRoutes(api huma.API, services *Services) {
	svc = services

	api.OpenAPI().Tags = append(api.OpenAPI().Tags,
		&huma.Tag{Name: "layers", Description: "Layer management operations"},
		&huma.Tag{Name: "tiles", Description: "Tile serving and management"},
		&huma.Tag{Name: "sources", Description: "Source file management"},
		&huma.Tag{Name: "health", Description: "Health check endpoints"},
	)

	huma.Get(api, "/health", GetHealth)
	huma.Get(api, "/api/v1/layers", GetLayers)
	huma.Post(api, "/api/v1/layers", CreateLayer)
	huma.Get(api, "/api/v1/layers/{id}", GetLayer)
	huma.Delete(api, "/api/v1/layers/{id}", DeleteLayer)
	huma.Get(api, "/api/v1/sources", GetSources)
	huma.Get(api, "/api/v1/tiles", GetTiles)
}

// ==================== Health ====================

type HealthBody struct {
	Status  string `json:"status" doc:"Health status" example:"ok"`
	Version string `json:"version" doc:"API version" example:"1.0.0"`
}

type HealthOutput struct {
	Body HealthBody
}

func GetHealth(ctx context.Context, input *struct{}) (*HealthOutput, error) {
	return &HealthOutput{
		Body: HealthBody{Status: "ok", Version: "1.0.0"},
	}, nil
}

// ==================== Layers ====================
// Uses service.LayerConfig directly - DRY!

type GetLayersOutput struct {
	Body map[string]service.LayerConfig
}

func GetLayers(ctx context.Context, input *struct{}) (*GetLayersOutput, error) {
	if svc == nil || svc.Layer == nil {
		return &GetLayersOutput{Body: map[string]service.LayerConfig{}}, nil
	}
	return &GetLayersOutput{Body: svc.Layer.List()}, nil
}

// CreateLayerInput uses service.LayerConfig directly for the body.
// Huma reads validation tags (required, minLength, etc.) from service.LayerConfig.
type CreateLayerInput struct {
	Body service.LayerConfig
}

type CreateLayerOutput struct {
	Body struct {
		ID      string              `json:"id" doc:"Generated layer ID"`
		Layer   service.LayerConfig `json:"layer" doc:"Created layer configuration"`
		Message string              `json:"message" doc:"Success message" example:"Layer created successfully"`
	}
}

func CreateLayer(ctx context.Context, input *CreateLayerInput) (*CreateLayerOutput, error) {
	if svc == nil || svc.Layer == nil {
		return nil, huma.Error400BadRequest("service not available")
	}

	created, err := svc.Layer.Create(input.Body)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	return &CreateLayerOutput{
		Body: struct {
			ID      string              `json:"id" doc:"Generated layer ID"`
			Layer   service.LayerConfig `json:"layer" doc:"Created layer configuration"`
			Message string              `json:"message" doc:"Success message" example:"Layer created successfully"`
		}{
			ID:      created.ID,
			Layer:   created,
			Message: "Layer created successfully",
		},
	}, nil
}

type GetLayerInput struct {
	ID string `path:"id" doc:"Layer ID" example:"buildings"`
}

type GetLayerOutput struct {
	Body service.LayerConfig
}

func GetLayer(ctx context.Context, input *GetLayerInput) (*GetLayerOutput, error) {
	if svc == nil || svc.Layer == nil {
		return nil, huma.Error404NotFound("service not available")
	}

	layer, ok := svc.Layer.Get(input.ID)
	if !ok {
		return nil, huma.Error404NotFound("layer not found")
	}

	return &GetLayerOutput{Body: layer}, nil
}

type DeleteLayerInput struct {
	ID string `path:"id" doc:"Layer ID to delete" example:"buildings"`
}

type DeleteLayerOutput struct {
	Body struct {
		Message string `json:"message" doc:"Success message" example:"Layer deleted successfully"`
	}
}

func DeleteLayer(ctx context.Context, input *DeleteLayerInput) (*DeleteLayerOutput, error) {
	if svc == nil || svc.Layer == nil {
		return nil, huma.Error400BadRequest("service not available")
	}

	if err := svc.Layer.Delete(input.ID); err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}

	return &DeleteLayerOutput{
		Body: struct {
			Message string `json:"message" doc:"Success message" example:"Layer deleted successfully"`
		}{Message: "Layer deleted successfully"},
	}, nil
}

// ==================== Sources ====================

type GetSourcesOutput struct {
	Body []service.SourceFile
}

func GetSources(ctx context.Context, input *struct{}) (*GetSourcesOutput, error) {
	if svc == nil || svc.Source == nil {
		return &GetSourcesOutput{Body: []service.SourceFile{}}, nil
	}

	sources, err := svc.Source.List()
	if err != nil {
		return &GetSourcesOutput{Body: []service.SourceFile{}}, nil
	}
	return &GetSourcesOutput{Body: sources}, nil
}

// ==================== Tiles ====================

type GetTilesOutput struct {
	Body []service.TileFile
}

func GetTiles(ctx context.Context, input *struct{}) (*GetTilesOutput, error) {
	if svc == nil || svc.Tile == nil {
		return &GetTilesOutput{Body: []service.TileFile{}}, nil
	}

	tiles, err := svc.Tile.List()
	if err != nil {
		return &GetTilesOutput{Body: []service.TileFile{}}, nil
	}
	return &GetTilesOutput{Body: tiles}, nil
}
