# ADR 013: File Upload and Tile Generation Migration to Huma

## Status
Proposed

## Context

The plat-geo editor has three legacy HTTP handlers that need migration to Huma:

1. **`handleSourceUpload`** - Multipart file upload for GeoJSON/GeoParquet
2. **`handleSourceDelete`** - Delete source files
3. **`handleTileGenerate`** - Trigger Tippecanoe tile generation with SSE progress

These handlers are more complex than the CRUD endpoints already migrated because they involve:
- Multipart form data parsing
- File system operations
- External command execution (Tippecanoe)
- SSE progress streaming during long operations

## Decision

### 1. Huma Multipart File Upload

Huma supports multipart uploads via `huma.MultipartFormFiles`:

```go
type SourceUploadInput struct {
    File huma.MultipartFile `form:"file" doc:"GeoJSON or GeoParquet file"`
}

func (h *SourceHandler) Upload(ctx context.Context, input *SourceUploadInput) (*huma.StreamResponse, error) {
    // Validate file extension
    ext := strings.ToLower(filepath.Ext(input.File.Filename))
    if !isValidSourceExt(ext) {
        return nil, huma.Error400BadRequest("Invalid file type")
    }

    // Save file
    destPath := filepath.Join(h.sourcesDir, input.File.Filename)
    if err := saveUploadedFile(input.File, destPath); err != nil {
        return nil, huma.Error500InternalServerError("Failed to save file")
    }

    // Return SSE success
    return &huma.StreamResponse{
        Body: func(humaCtx huma.Context) {
            sse := NewSSEContext(humaCtx)
            sse.SendSuccess("File uploaded: " + input.File.Filename)
        },
    }, nil
}
```

### 2. Source Delete with Path Parameter

Use Huma path parameters:

```go
type SourceDeleteInput struct {
    Filename string `path:"filename" doc:"Source filename to delete"`
}

func (h *SourceHandler) Delete(ctx context.Context, input *SourceDeleteInput) (*huma.StreamResponse, error) {
    // Validate filename (no path traversal)
    if strings.ContainsAny(input.Filename, "/\\..") {
        return nil, huma.Error400BadRequest("Invalid filename")
    }

    filePath := filepath.Join(h.sourcesDir, input.Filename)
    if err := os.Remove(filePath); err != nil {
        if os.IsNotExist(err) {
            return nil, huma.Error404NotFound("File not found")
        }
        return nil, huma.Error500InternalServerError("Failed to delete")
    }

    return &huma.StreamResponse{
        Body: func(humaCtx huma.Context) {
            sse := NewSSEContext(humaCtx)
            sse.SendSuccess("Deleted: " + input.Filename)
            // Refresh source list
            html := h.renderSourceList()
            sse.PatchElements(html, "#source-list")
        },
    }, nil
}
```

### 3. Tile Generation with Progress Streaming

The most complex handler - runs Tippecanoe and streams progress:

```go
type TileGenerateInput struct {
    Body struct {
        SourceFile string `json:"sourceFile" required:"true" doc:"Source file name"`
        OutputName string `json:"outputName" required:"true" doc:"Output PMTiles name"`
        LayerName  string `json:"layerName" default:"default" doc:"Layer name in tiles"`
        MinZoom    int    `json:"minZoom" default:"0" minimum:"0" maximum:"22"`
        MaxZoom    int    `json:"maxZoom" default:"14" minimum:"0" maximum:"22"`
    }
}

func (h *TileHandler) Generate(ctx context.Context, input *TileGenerateInput) (*huma.StreamResponse, error) {
    // Validate source exists
    sourcePath := filepath.Join(h.sourcesDir, input.Body.SourceFile)
    if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
        return nil, huma.Error404NotFound("Source file not found")
    }

    return &huma.StreamResponse{
        Body: func(humaCtx huma.Context) {
            sse := NewSSEContext(humaCtx)

            // Progress updates
            sse.SendSignals(map[string]any{
                "tileStatus": "Starting...",
                "tileProgress": 10,
            })

            // Run Tippecanoe
            outputPath := filepath.Join(h.tilesDir, input.Body.OutputName)
            err := h.runTippecanoe(ctx, sourcePath, outputPath, input.Body, func(progress int, status string) {
                sse.SendSignals(map[string]any{
                    "tileStatus": status,
                    "tileProgress": progress,
                })
            })

            if err != nil {
                sse.SendError(err.Error())
                return
            }

            sse.SendSignals(map[string]any{
                "tileStatus": "Complete!",
                "tileProgress": 100,
                "success": "Tiles generated: " + input.Body.OutputName,
            })

            // Refresh tile list
            html := h.renderTileList()
            sse.PatchElements(html, "#tile-list")
        },
    }, nil
}
```

### 4. Move to TilerService

Extract Tippecanoe execution to a dedicated service:

```go
// internal/service/tiler.go
type TilerService struct {
    sourcesDir string
    tilesDir   string
}

type TileGenerateOptions struct {
    SourceFile string
    OutputName string
    LayerName  string
    MinZoom    int
    MaxZoom    int
}

type ProgressFunc func(progress int, status string)

func (s *TilerService) Generate(ctx context.Context, opts TileGenerateOptions, onProgress ProgressFunc) error {
    args := []string{
        "-o", filepath.Join(s.tilesDir, opts.OutputName),
        "-l", opts.LayerName,
        "-Z", strconv.Itoa(opts.MinZoom),
        "-z", strconv.Itoa(opts.MaxZoom),
        "--force",
        "--drop-densest-as-needed",
        filepath.Join(s.sourcesDir, opts.SourceFile),
    }

    cmd := exec.CommandContext(ctx, "tippecanoe", args...)
    // ... execute with progress parsing
}
```

## Implementation Steps

### Phase 1: Service Layer
- [ ] Create `TilerService` with Tippecanoe execution
- [ ] Add progress callback support
- [ ] Add validation methods

### Phase 2: Migrate Upload
- [ ] Add `Upload` method to `SourceHandler` with Huma multipart
- [ ] Register route: `POST /api/v1/editor/sources/upload`
- [ ] Update editor.html to use new endpoint
- [ ] Remove legacy `handleSourceUpload`

### Phase 3: Migrate Delete
- [ ] Add `Delete` method to `SourceHandler` with path parameter
- [ ] Register route: `DELETE /api/v1/editor/sources/{filename}`
- [ ] Update editor.html to use new endpoint
- [ ] Remove legacy `handleSourceDelete`

### Phase 4: Migrate Tile Generation
- [ ] Add `Generate` method to `TileHandler`
- [ ] Register route: `POST /api/v1/editor/tiles/generate`
- [ ] Update editor.html to use new endpoint
- [ ] Remove legacy `handleTileGenerate`

### Phase 5: Cleanup
- [ ] Remove legacy handlers from server.go
- [ ] Update OpenAPI spec generator
- [ ] Regenerate TypeScript types
- [ ] Update tests

## API Changes

### New Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/editor/sources/upload` | Upload source file |
| `DELETE` | `/api/v1/editor/sources/{filename}` | Delete source file |
| `POST` | `/api/v1/editor/tiles/generate` | Generate PMTiles |

### Request/Response Types

```typescript
// Upload - multipart/form-data
interface SourceUploadInput {
    file: File;
}

// Generate
interface TileGenerateInput {
    sourceFile: string;
    outputName: string;
    layerName?: string;  // default: "default"
    minZoom?: number;    // default: 0
    maxZoom?: number;    // default: 14
}

// SSE Progress Events
interface TileProgress {
    tileStatus: string;
    tileProgress: number;
    success?: string;
    error?: string;
}
```

## Consequences

### Positive
- All endpoints documented in OpenAPI spec
- TypeScript types for upload/generate inputs
- Consistent error handling via Huma
- Input validation via Huma tags
- Cleaner server.go (remove ~200 lines)

### Negative
- Huma multipart handling may differ from stdlib
- Need to handle Tippecanoe stderr for progress
- Context cancellation needs careful handling

### Risks
- File upload size limits need configuration
- Tippecanoe path may vary across systems
- Long-running tile generation needs timeout handling

## References

- [Huma Multipart Forms](https://huma.rocks/features/request-inputs/#multipart-forms)
- [Tippecanoe Options](https://github.com/felt/tippecanoe#options)
- [ADR-012: Huma REST API Framework](012-huma-rest-api-framework.md)
