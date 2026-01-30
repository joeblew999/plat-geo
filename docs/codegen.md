# Code Generation

All generated code in this project comes from one source: **Go struct tags in `internal/service/types.go`**.

Change a struct → `air` rebuilds → everything regenerates automatically.

## Pipeline

```
internal/service/types.go
  │
  ├─ humastargen     → internal/api/editor/signals_gen.go     Datastar signal parsers
  │                   → web/templates/generated/layer-form-gen.html  Datastar form fragments
  │
  ├─ huma (runtime)  → openapi.json                           OpenAPI 3.1 spec
  │
  ├─ openapi-typescript → web/src/generated/api.ts            TypeScript types (frontend)
  │
  └─ humaclient      → pkg/geoclient/client.go                Go SDK (external consumers)
                        typed methods + Follow() for HATEOAS link traversal
```

## How it runs

```
air → task build → task huma:gen → { gen:datastar, spec:json, ts, gen:client }
```

Taskfile `sources`/`generates` fingerprinting means unchanged files are skipped. See `taskfiles/huma.yml` for the full pipeline definition.

## What each step produces

### 1. Datastar signals + forms (`humastargen`)

**Tool**: `cmd/humastargen/main.go` — reads struct tags via `reflect`

**Output**:
- `internal/api/editor/signals_gen.go` — `ParseLayerConfigSignals()`, `ResetLayerConfigSignals()`, signal name constants
- `web/templates/generated/layer-form-gen.html` — HTML form with `data-bind` attributes matching signals

**Tags used**: `signal`, `input`, `sse`, `default`, `enum`, `required`, `minimum`, `maximum`, `doc`

```go
// From types.go:
Name string `json:"name" required:"true" minLength:"1" doc:"Display name"`

// Generates in signals_gen.go:
Name: s.String("newlayername"),

// Generates in layer-form-gen.html:
<label>Display name</label>
<input type="text" data-bind:newlayername required>
```

### 2. OpenAPI spec (`huma`)

**Tool**: Huma framework — reads struct tags at server startup

**Output**: `openapi.json`

**Tags used**: `json`, `required`, `minLength`, `maxLength`, `minimum`, `maximum`, `enum`, `default`, `doc`, `example`

The spec is extracted by running the server binary: `go run ./cmd/geo spec > openapi.json`

### 3. TypeScript types (`openapi-typescript`)

**Tool**: `bun x openapi-typescript` — reads `openapi.json`

**Output**:
- `web/src/generated/api.ts` — TypeScript interfaces for all endpoints, request/response schemas
- `web/src/generated/client.ts` — type-safe fetch wrapper (hand-written, imports from `api.ts`)

```ts
import { api } from './generated/client';

const { data } = await api.GET('/api/v1/layers');
// data is fully typed — TypeScript knows the shape
```

### 4. Go client SDK (`humaclient`)

**Tool**: `humaclient` library — reads OpenAPI spec from Huma API at codegen time

**Output**: `pkg/geoclient/client.go` — 1900+ lines:
- `PlatGeoAPIClient` interface with 29 typed methods
- `Follow()` method for HATEOAS link traversal using RFC 8288 Link headers
- `parseLinkHeader()` for extracting rel URLs from responses
- Functional options: `WithHeader()`, `WithQuery()`, `WithBody()`

```go
import "github.com/joeblew999/plat-geo/pkg/geoclient"

c := geoclient.New("http://localhost:8086")
_, layer, _ := c.GetLayer(ctx, "buildings")
// layer is fully typed — Go knows the shape
```

The SDK is committed to the repo so external Go services can `go get` it without running the server.

## Commands

| Command | What |
|---------|------|
| `task gen` | Run full pipeline (all 4 steps) |
| `task huma:gen:datastar` | Signals + HTML forms only |
| `task huma:spec:json` | OpenAPI spec only |
| `task huma:ts` | TypeScript types only |
| `task huma:gen:client` | Go SDK only |
| `task dev` | Air hot reload (runs gen on every rebuild) |

## Generated files

All generated files have a `DO NOT EDIT` header. Grep for them:

```bash
grep -r "DO NOT EDIT" --include="*.go" --include="*.ts" --include="*.html"
```

| File | Generator |
|------|-----------|
| `internal/api/editor/signals_gen.go` | `cmd/humastargen` |
| `web/templates/generated/layer-form-gen.html` | `cmd/humastargen` |
| `openapi.json` | `cmd/geo spec` |
| `web/src/generated/api.ts` | `openapi-typescript` |
| `pkg/geoclient/client.go` | `humaclient` |
