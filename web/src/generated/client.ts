/**
 * Type-safe API client generated from OpenAPI spec.
 *
 * Usage:
 *   import { api, LayerConfig } from './generated/client';
 *
 *   // GET /api/v1/layers - fully typed response
 *   const { data, error } = await api.GET('/api/v1/layers');
 *   if (data) {
 *     Object.entries(data).forEach(([id, layer]) => {
 *       console.log(layer.name, layer.geomType);  // TypeScript knows the shape!
 *     });
 *   }
 *
 *   // POST /api/v1/layers - request body is type-checked
 *   const { data: created } = await api.POST('/api/v1/layers', {
 *     body: {
 *       name: 'Buildings',
 *       file: 'buildings.pmtiles',
 *       geomType: 'polygon',  // TypeScript enforces: 'polygon' | 'line' | 'point'
 *       defaultVisible: true,
 *       opacity: 0.7,
 *     }
 *   });
 *
 *   // DELETE /api/v1/layers/{id} - path params are typed
 *   await api.DELETE('/api/v1/layers/{id}', {
 *     params: { path: { id: 'buildings' } }
 *   });
 */

import createClient from 'openapi-fetch';
import type { paths, components } from './api';

// Create the type-safe client
export const api = createClient<paths>({
  baseUrl: typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8086',
});

// Export types for convenience
export type { paths, components } from './api';

// Helper type aliases - use LayerConfig for both input and output (DRY!)
export type LayerConfig = components['schemas']['LayerConfig'];
export type SourceFile = components['schemas']['SourceFile'];
export type TileFile = components['schemas']['TileFile'];
export type RenderRule = components['schemas']['RenderRule'];
export type LegendItem = components['schemas']['LegendItem'];

// For creating layers, use the same LayerConfig type
export type CreateLayerInput = components['schemas']['LayerConfig'];
