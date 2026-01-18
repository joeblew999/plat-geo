# plat-geo

Geospatial data platform for maps and routing with GeoParquet and PMTiles support.

![Map Viewer](docs/screenshots/viewer.png)

## Features

- **GeoParquet** - Efficient columnar storage for geospatial data
- **PMTiles** - Single-file tile archives for vector and raster maps
- **DuckDB** - Analytical queries with spatial extensions
- **DuckLake** - Data lakehouse for versioned geospatial tables
- **NornicDB** - Graph database for spatial relationships and AI
- **Huma REST API** - OpenAPI 3.1 with auto-generated docs and clients
- **Datastar SSE** - Real-time UI updates with server-sent events

## Quick Start

```bash
# Quickstart: download sample data and setup
task quickstart

# Start the server
task dev
```

Then open:
- **Viewer**: http://localhost:8086/viewer - Map viewer with layer toggles
- **Editor**: http://localhost:8086/editor - Layer configuration & tile generation
- **API Docs**: http://localhost:8086/docs - Interactive OpenAPI documentation

### Manual Setup

```bash
# Install dependencies
task deps

# Build
task build

# Initialize data directory
task data:init

# Run the server
task dev

# Run all services with process-compose
process-compose up
```

## Screenshots

### Map Viewer (`/viewer`)

Full-screen map viewer with layer toggles and PMTiles rendering:

![Map Viewer](docs/screenshots/viewer.png)

### Layer Editor (`/editor`)

Layer configuration interface with live preview:

![Layer Editor](docs/screenshots/editor.png)

### API Documentation (`/docs`)

Interactive OpenAPI documentation with all endpoints:

![API Docs](docs/screenshots/api-docs.png)

## Web UI

### Viewer (`/viewer`)

Read-only map viewer with layer toggles:

- Displays all configured layers from PMTiles
- Checkbox controls for layer visibility
- Full-screen map with zoom controls
- Protomaps basemap integration

### Editor (`/editor`)

Layer configuration interface for managing PMTiles layers:

- **Layers Tab**: Add, edit, preview, and delete map layers
- **Tiles Tab**: View PMTiles files, generate tiles from GeoJSON
- **Sources Tab**: Upload and manage GeoJSON/GeoParquet sources

Each layer configuration supports:
- Layer name and PMTiles file selection
- Layer type within PMTiles (e.g., "buildings", "roads")
- Geometry type (point, line, polygon)
- Styling: fill color, stroke color, opacity
- Default visibility toggle

## REST API

Interactive documentation available at http://localhost:8086/docs

### Layer Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/layers` | List all layers |
| `POST` | `/api/v1/layers` | Create a new layer |
| `GET` | `/api/v1/layers/{id}` | Get layer by ID |
| `DELETE` | `/api/v1/layers/{id}` | Delete a layer |

### Source & Tile Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/sources` | List source files |
| `GET` | `/api/v1/tiles` | List PMTiles files |

### System Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/api/v1/info` | Service information |
| `GET` | `/api/v1/tables` | List DuckDB tables |
| `POST` | `/api/v1/query` | Execute SQL query |
| `GET` | `/openapi.json` | OpenAPI 3.1 specification |

### Editor SSE Endpoints

Real-time updates for the editor UI using Datastar:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/editor/layers` | Stream layer list |
| `POST` | `/api/v1/editor/layers` | Create layer (SSE) |
| `DELETE` | `/api/v1/editor/layers/{id}` | Delete layer (SSE) |
| `GET` | `/api/v1/editor/tiles` | Stream tile list |
| `GET` | `/api/v1/editor/sources` | Stream source list |

## Code Generation

Generate TypeScript types from the OpenAPI spec:

```bash
# Generate all (OpenAPI spec + TypeScript types)
task huma:all

# Generate just TypeScript types
task huma:ts

# Export OpenAPI spec
task huma:spec
```

Generated files:
- `openapi.json` - OpenAPI 3.1 specification
- `web/src/generated/api.ts` - TypeScript types

## Configuration

Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
```

| Variable | Default | Description |
|----------|---------|-------------|
| `GEO_PORT` | 8086 | Server port |
| `GEO_HOST` | 0.0.0.0 | Host to bind |
| `DATA_DIR` | .data | Data directory |
| `BIN_DIR` | .bin | Binary directory |
| `DUCKDB_UI_PORT` | 4213 | DuckDB UI port |
| `NORNICDB_HTTP_PORT` | 7474 | NornicDB HTTP port |
| `NORNICDB_BOLT_PORT` | 7687 | NornicDB Bolt port |

### Layer Configuration

Layers are stored in `.data/layers.json`:

```json
{
  "firenze": {
    "name": "Firenze",
    "file": "firenze.pmtiles",
    "pmtilesLayer": "buildings",
    "geomType": "polygon",
    "defaultVisible": true,
    "fill": "#3388ff",
    "stroke": "#2266cc",
    "opacity": 0.7
  }
}
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| geo | 8086 | Main API server |
| duckdb-ui | 4213 | DuckDB web interface |
| nornicdb | 7474/7687 | Graph database |

## Tasks

```bash
# DuckDB
task duckdb:install       # Install DuckDB CLI
task duckdb:shell         # Open DuckDB shell
task duckdb:ui:run        # Start DuckDB UI

# DuckLake
task ducklake:install     # Install DuckLake extension
task ducklake:attach      # Attach DuckLake database

# NornicDB
task nornicdb:start       # Start NornicDB container
task nornicdb:stats       # Show node/relationship counts
task nornicdb:cypher      # Run Cypher query

# PMTiles
task pmtiles:install      # Install pmtiles CLI
task pmtiles:extract      # Extract region from Protomaps
task pmtiles:serve        # Serve PMTiles locally

# Tippecanoe
task tippecanoe:install   # Install Tippecanoe
task tippecanoe:generate  # Generate tiles from GeoJSON
task tippecanoe:cluster   # Generate clustered point tiles

# Huma API
task huma:all             # Generate TypeScript types
task huma:spec            # Export OpenAPI spec
task huma:ts              # Generate TypeScript from spec
task huma:docs            # Show API docs URL
```

## Architecture Decisions

See [docs/adr/](docs/adr/) for architecture decision records:

- [ADR-001: PMTiles Cloud Storage](docs/adr/001-pmtiles-cloud-storage.md)
- [ADR-002: DuckDB Storage and Caching](docs/adr/002-duckdb-storage-caching.md)
- [ADR-003: DuckLake Catalog and Storage](docs/adr/003-ducklake-catalog-storage.md)
- [ADR-004: NornicDB Graph Integration](docs/adr/004-nornicdb-graph-integration.md)
- [ADR-005: Geospatial Data Sources](docs/adr/005-geospatial-data-sources.md)
- [ADR-006: DuckDB Extensions](docs/adr/006-duckdb-extensions.md)
- [ADR-007: HUGR GraphQL Layer](docs/adr/007-hugr-graphql-layer.md)
- [ADR-008: Basemap Strategy](docs/adr/008-basemap-strategy.md)
- [ADR-009: Tile Generation with Tippecanoe](docs/adr/009-tile-generation-tippecanoe.md)
- [ADR-010: Geospatial Tiling System](docs/adr/010-tiling-system.md)
- [ADR-011: Web UI Viewer and Editor](docs/adr/011-web-ui-viewer-editor.md)
- [ADR-012: Huma REST API Framework](docs/adr/012-huma-rest-api-framework.md)

## xplat Integration

This project uses [xplat](https://github.com/joeblew999/xplat) for tooling.

```bash
# Install as xplat package
xplat pkg install plat-geo

# Run with process-compose
xplat process up
```

## References

### PMTiles
- [Protomaps Documentation](https://docs.protomaps.com/)
- [PMTiles Cloud Storage](https://docs.protomaps.com/pmtiles/cloud-storage)
- [go-pmtiles](https://github.com/protomaps/go-pmtiles)

### DuckDB
- [DuckDB Documentation](https://duckdb.org/docs/)
- [DuckDB S3 API](https://duckdb.org/docs/stable/core_extensions/httpfs/s3api.html)
- [DuckDB Geography Extension](https://duckdb.org/community_extensions/extensions/geography)

### DuckLake
- [DuckLake Documentation](https://ducklake.select/docs/stable/duckdb/introduction)
- [Choosing a Catalog Database](https://ducklake.select/docs/stable/duckdb/usage/choosing_a_catalog_database)

### NornicDB
- [NornicDB Releases](https://github.com/orneryd/NornicDB/releases)

### GeoParquet
- [GeoParquet Specification](https://github.com/opengeospatial/geoparquet)
- [duckdb-geography](https://github.com/paleolimbot/duckdb-geography)

### Tippecanoe
- [Tippecanoe GitHub](https://github.com/felt/tippecanoe)
- [Felt Blog: Tippecanoe](https://felt.com/blog/tippecanoe)

### Huma
- [Huma Documentation](https://huma.rocks/)
- [Huma GitHub](https://github.com/danielgtaylor/huma)

### Datastar
- [Datastar Documentation](https://data-star.dev/)
- [Datastar Go SDK](https://github.com/starfederation/datastar-go)
