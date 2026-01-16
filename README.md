# plat-geo

Geographical system for maps and routing with GeoParquet and PMTiles support.

## Features

- **GeoParquet** - Efficient columnar storage for geospatial data
- **PMTiles** - Single-file tile archives for vector and raster maps
- **DuckDB** - Analytical queries with spatial extensions
- **DuckLake** - Data lakehouse for versioned geospatial tables
- **NornicDB** - Graph database for spatial relationships and AI
- **Routing** - Path finding and route optimization

## Quick Start

```bash
# Install dependencies
task deps

# Build
task build

# Run the server
task dev

# Run all services with process-compose
process-compose up
```

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

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |
| `GET /api/v1/info` | Service information |
| `GET /api/v1/tables` | List DuckDB tables |
| `POST /api/v1/query` | Execute SQL query |

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
```

## Architecture Decisions

See [docs/adr/](docs/adr/) for architecture decision records:

- [ADR-001: PMTiles Cloud Storage](docs/adr/001-pmtiles-cloud-storage.md)
- [ADR-002: DuckDB Storage and Caching](docs/adr/002-duckdb-storage-caching.md)
- [ADR-003: DuckLake Catalog and Storage](docs/adr/003-ducklake-catalog-storage.md)

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
