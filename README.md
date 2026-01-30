# plat-geo

Geospatial data platform with PMTiles, GeoParquet, and a fully hypermedia-driven REST API.

## Run

```bash
air
```

```
plat-geo API server starting...
  Server:  http://localhost:8086
  Data:    .data

  Pages:   http://localhost:8086/viewer, http://localhost:8086/editor
  Docs:    http://localhost:8086/docs
  OpenAPI: http://localhost:8086/openapi.json
```

## Pages

### `/editor` — Datastar reactive layer editor (map + sidebar)

![Editor](/.playwright-mcp/editor-real.png)

### `/editor-gen` — Spec-driven editor (form auto-generated from OpenAPI schema)

![Editor (Generated)](/.playwright-mcp/editor-gen.png)

### `/explorer` — HATEOAS hypermedia mesh explorer

![Explorer](/.playwright-mcp/explorer.png)

### `/viewer` — Map viewer

### `/docs` — Interactive API docs (Scalar)

![API Docs](/.playwright-mcp/docs.png)

## API

All endpoints return RFC 8288 Link headers for HATEOAS navigation.
Start at `/health` and follow the links.

### Layers (public API)

| Method | URL | What |
|--------|-----|------|
| `GET` | `/api/v1/layers` | List all layers |
| `POST` | `/api/v1/layers` | Create a layer |
| `GET` | `/api/v1/layers/{id}` | Get a layer |
| `PUT` | `/api/v1/layers/{id}` | Update a layer |
| `PATCH` | `/api/v1/layers/{id}` | Partial update (JSON Merge Patch) |
| `DELETE` | `/api/v1/layers/{id}` | Delete a layer |
| `POST` | `/api/v1/layers/{id}/publish` | Publish (state-dependent action) |
| `POST` | `/api/v1/layers/{id}/unpublish` | Unpublish (state-dependent action) |
| `POST` | `/api/v1/layers/{id}/duplicate` | Duplicate a layer |
| `GET` | `/api/v1/layers/{id}/styles` | List style variants |
| `POST` | `/api/v1/layers/{id}/styles` | Add a style variant |
| `DELETE` | `/api/v1/layers/{id}/styles/{styleId}` | Delete a style variant |

### Editor (Datastar SSE endpoints)

| Method | URL | What |
|--------|-----|------|
| `GET` | `/api/v1/editor/layers` | SSE: render layer list |
| `POST` | `/api/v1/editor/layers` | SSE: create layer from Datastar signals |
| `DELETE` | `/api/v1/editor/layers/{id}` | SSE: delete layer, patch DOM |
| `GET` | `/api/v1/editor/tiles` | SSE: render tile list |
| `GET` | `/api/v1/editor/tiles/select` | SSE: render tile `<select>` options |
| `POST` | `/api/v1/editor/tiles/generate` | SSE: generate PMTiles with progress stream |
| `GET` | `/api/v1/editor/sources` | SSE: render source file list |
| `GET` | `/api/v1/editor/sources/select` | SSE: render source `<select>` options |
| `POST` | `/api/v1/editor/sources/upload` | SSE: upload source file |
| `DELETE` | `/api/v1/editor/sources/{filename}` | SSE: delete source file |
| `GET` | `/api/v1/editor/events` | SSE: resource change event stream |

### Other

| Method | URL | What |
|--------|-----|------|
| `GET` | `/health` | Health check (HATEOAS entry point) |
| `GET` | `/api/v1/info` | Server info |
| `GET` | `/api/v1/sources` | List source files |
| `GET` | `/api/v1/tiles` | List tile files |
| `GET` | `/api/v1/tables` | List database tables |
| `POST` | `/api/v1/query` | Execute SQL query |
| `GET` | `/openapi.json` | OpenAPI 3.1 spec (with x-datastar extensions) |
| `GET` | `/docs` | Interactive API docs (Scalar) |

## Deploy

Live: **https://plat-geo.fly.dev**

```bash
task deploy
```

Requires [Fly.io CLI](https://fly.io/docs/flyctl/install/). First-time setup:

```bash
fly launch          # create app + volume
fly deploy          # build & deploy
```

## Architecture

- **[Architecture Overview](docs/architecture.md)** — Huma + Datastar synergy
- **[Hypermedia & HATEOAS](docs/hypermedia.md)** — RFC 8288 Link headers, state-dependent actions, the explorer mesh
- **[Code Generation](docs/codegen.md)** — Codegen pipeline
- **[Huma Guide](docs/huma.md)** — REST API patterns
- **[Datastar Guide](docs/datastar.md)** — Reactive frontend
