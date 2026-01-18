# ADR-009: Vector Tile Generation with Tippecanoe

## Status

Proposed

ref: https://github.com/felt/tippecanoe

## Context

plat-geo needs to create custom vector tiles from:
- GeoParquet data (places, buildings, custom datasets)
- GeoJSON exports from DuckDB queries
- User-uploaded geographic data

While Protomaps provides ready-made OSM basemaps (ADR-008), we need Tippecanoe to generate overlay tiles from our own data sources.

## What is Tippecanoe?

Tippecanoe is a command-line tool (maintained by Felt) that converts geographic data into vector tilesets. It creates tiles optimized for web mapping that:

- Show detail at all zoom levels
- Preserve data density patterns
- Handle millions of features efficiently
- Output to PMTiles or MBTiles format

## Decision Drivers

- **Custom data visualization**: Overlay our data on basemaps
- **Performance**: Pre-generated tiles are faster than real-time rendering
- **Flexibility**: Control zoom levels, simplification, clustering
- **Integration**: Outputs PMTiles (same format as basemaps)

## Tippecanoe Capabilities

### Input Formats

| Format | Notes |
|--------|-------|
| GeoJSON | Standard, gzipped, or newline-delimited |
| FlatGeobuf | Efficient binary format |
| CSV | With lat/lng columns |
| stdin | Pipe from other tools |

### Output Formats

| Format | Notes |
|--------|-------|
| **PMTiles** | Single file, cloud-friendly (recommended) |
| MBTiles | SQLite-based, traditional |
| Directory | Individual tile files |

### Key Features

| Feature | Description |
|---------|-------------|
| Auto zoom | `-zg` selects optimal zoom levels |
| Clustering | Aggregate dense points with stats |
| Simplification | Reduce geometry complexity at low zooms |
| Attribute filtering | Include/exclude properties |
| Tile size management | Drop/coalesce to fit tile limits |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Data Sources                             │
├─────────────────┬─────────────────┬─────────────────────────┤
│    DuckDB       │   GeoParquet    │      GeoJSON            │
│    Query        │   Files         │      Uploads            │
└────────┬────────┴────────┬────────┴────────┬────────────────┘
         │                 │                 │
         ▼                 ▼                 ▼
┌─────────────────────────────────────────────────────────────┐
│                    GeoJSON Export                            │
│  (DuckDB → ST_AsGeoJSON → newline-delimited GeoJSON)        │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                     Tippecanoe                               │
│  - Feature selection                                        │
│  - Zoom level optimization                                  │
│  - Geometry simplification                                  │
│  - Clustering/aggregation                                   │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  .data/pmtiles/                              │
│  ├── basemap.pmtiles      (Protomaps OSM)                   │
│  ├── places.pmtiles       (Our POI data)                    │
│  ├── buildings.pmtiles    (Custom buildings)                │
│  └── custom.pmtiles       (User uploads)                    │
└─────────────────────────────────────────────────────────────┘
```

## Common Workflows

### 1. Export DuckDB to GeoJSON

```sql
-- Export places as newline-delimited GeoJSON
COPY (
    SELECT json_object(
        'type', 'Feature',
        'geometry', ST_AsGeoJSON(geometry)::JSON,
        'properties', json_object(
            'id', id,
            'name', name,
            'category', category
        )
    ) as feature
    FROM places
    WHERE category IS NOT NULL
) TO '/tmp/places.geojsonl' (FORMAT CSV, HEADER false, QUOTE '');
```

### 2. Generate Tiles with Tippecanoe

```bash
# Basic tile generation
tippecanoe -o places.pmtiles -zg /tmp/places.geojsonl

# With options
tippecanoe \
    -o .data/pmtiles/places.pmtiles \
    -Z 4 -z 14 \                    # Zoom range
    --drop-densest-as-needed \      # Handle dense areas
    --extend-zooms-if-still-dropping \
    --layer=places \                # Layer name
    --name="POI Layer" \
    /tmp/places.geojsonl
```

### 3. Point Clustering

```bash
# Cluster points with count aggregation
tippecanoe \
    -o .data/pmtiles/clustered.pmtiles \
    -zg \
    --cluster-distance=50 \
    --accumulate-attribute=count:sum \
    --cluster-densest-as-needed \
    /tmp/points.geojsonl
```

### 4. Polygon Simplification

```bash
# Buildings with controlled simplification
tippecanoe \
    -o .data/pmtiles/buildings.pmtiles \
    -Z 10 -z 16 \
    --coalesce-densest-as-needed \
    --simplification=10 \
    --detect-shared-borders \
    /tmp/buildings.geojsonl
```

### 5. Merge Multiple Sources

```bash
# Combine multiple layers into one tileset
tippecanoe \
    -o .data/pmtiles/combined.pmtiles \
    -zg \
    --named-layer=places:/tmp/places.geojsonl \
    --named-layer=roads:/tmp/roads.geojsonl \
    --named-layer=buildings:/tmp/buildings.geojsonl
```

## Installation Options

| Method | Platform | Notes |
|--------|----------|-------|
| Homebrew | macOS | `brew install tippecanoe` |
| apt | Ubuntu | `apt install tippecanoe` |
| Docker | Any | Community images available |
| Source | Any | Requires C++11 compiler |

### Docker Images

No official pre-built binaries, but Docker images are available:

| Image | Notes |
|-------|-------|
| `jskeates/tippecanoe` | Alpine-based, lightweight |
| `ingmapping/tippecanoe` | Ubuntu-based |
| `metacollin/tippecanoe` | v1.36.0 |

```bash
# Run via Docker
docker run --rm -v $(pwd):/data jskeates/tippecanoe \
  tippecanoe -o /data/output.pmtiles /data/input.geojson
```

## Taskfile Integration

```yaml
# taskfiles/Taskfile-tippecanoe.yml
version: "3"

vars:
  BIN_DIR: '{{.BIN_DIR | default ".bin"}}'
  DATA_DIR: '{{.DATA_DIR | default ".data"}}'
  PMTILES_DIR: '{{.DATA_DIR}}/pmtiles'
  TIPPECANOE_IMAGE: 'jskeates/tippecanoe'

tasks:
  install:
    desc: Install Tippecanoe
    cmds:
      - |
        if command -v brew &> /dev/null; then
          brew install tippecanoe
        elif command -v apt &> /dev/null; then
          sudo apt install tippecanoe
        else
          echo "Using Docker image instead: {{.TIPPECANOE_IMAGE}}"
          docker pull {{.TIPPECANOE_IMAGE}}
        fi

  install:docker:
    desc: Pull Tippecanoe Docker image
    cmds:
      - docker pull {{.TIPPECANOE_IMAGE}}

  version:
    desc: Show Tippecanoe version
    cmds:
      - tippecanoe --version

  generate:
    desc: Generate tiles from GeoJSON (FILE=input.geojson NAME=output)
    vars:
      FILE: '{{.FILE}}'
      NAME: '{{.NAME | default "tiles"}}'
      MIN_ZOOM: '{{.MIN_ZOOM | default "0"}}'
      MAX_ZOOM: '{{.MAX_ZOOM | default "14"}}'
    cmds:
      - mkdir -p {{.PMTILES_DIR}}
      - |
        tippecanoe \
          -o {{.PMTILES_DIR}}/{{.NAME}}.pmtiles \
          -Z {{.MIN_ZOOM}} -z {{.MAX_ZOOM}} \
          --drop-densest-as-needed \
          --extend-zooms-if-still-dropping \
          --force \
          {{.FILE}}

  generate:auto:
    desc: Generate tiles with automatic zoom detection
    vars:
      FILE: '{{.FILE}}'
      NAME: '{{.NAME | default "tiles"}}'
    cmds:
      - mkdir -p {{.PMTILES_DIR}}
      - tippecanoe -o {{.PMTILES_DIR}}/{{.NAME}}.pmtiles -zg --force {{.FILE}}

  cluster:
    desc: Generate clustered point tiles
    vars:
      FILE: '{{.FILE}}'
      NAME: '{{.NAME | default "clustered"}}'
      DISTANCE: '{{.DISTANCE | default "50"}}'
    cmds:
      - mkdir -p {{.PMTILES_DIR}}
      - |
        tippecanoe \
          -o {{.PMTILES_DIR}}/{{.NAME}}.pmtiles \
          -zg \
          --cluster-distance={{.DISTANCE}} \
          --cluster-densest-as-needed \
          --force \
          {{.FILE}}

  join:
    desc: Join CSV attributes to existing tileset
    vars:
      TILES: '{{.TILES}}'
      CSV: '{{.CSV}}'
      OUTPUT: '{{.OUTPUT}}'
    cmds:
      - tile-join -o {{.OUTPUT}} -c {{.CSV}} {{.TILES}}

  decode:
    desc: Decode tiles back to GeoJSON
    vars:
      TILES: '{{.TILES}}'
      Z: '{{.Z | default "10"}}'
      X: '{{.X}}'
      Y: '{{.Y}}'
    cmds:
      - tippecanoe-decode {{.TILES}} {{.Z}} {{.X}} {{.Y}}

  from-duckdb:
    desc: Export DuckDB table to tiles (TABLE=places)
    vars:
      TABLE: '{{.TABLE}}'
      NAME: '{{.NAME | default .TABLE}}'
    cmds:
      - mkdir -p {{.PMTILES_DIR}}
      - |
        {{.DUCKDB}} {{.DATA_DIR}}/duckdb/geo.duckdb -c "
          COPY (
            SELECT json_object(
              'type', 'Feature',
              'geometry', ST_AsGeoJSON(geometry)::JSON,
              'properties', to_json(columns(* EXCLUDE geometry))
            )
            FROM {{.TABLE}}
          ) TO '/tmp/{{.TABLE}}.geojsonl' (FORMAT CSV, HEADER false, QUOTE '');
        "
      - tippecanoe -o {{.PMTILES_DIR}}/{{.NAME}}.pmtiles -zg --force /tmp/{{.TABLE}}.geojsonl
      - rm /tmp/{{.TABLE}}.geojsonl
```

## Pipeline Examples

### Full Pipeline: Overture Places → Tiles

```bash
# 1. Download Overture places for region
task duckdb:query -- -c "
  COPY (
    SELECT * FROM read_parquet('s3://overturemaps-us-west-2/release/*/theme=places/*')
    WHERE bbox.xmin > -122.6 AND bbox.xmax < -121.7
      AND bbox.ymin > 37.2 AND bbox.ymax < 38.0
  ) TO '.data/parquet/bayarea_places.parquet';
"

# 2. Export to GeoJSON
task duckdb:query -- -c "
  COPY (
    SELECT json_object(
      'type', 'Feature',
      'geometry', ST_AsGeoJSON(geometry)::JSON,
      'properties', json_object('id', id, 'name', names.primary, 'category', categories.primary)
    )
    FROM '.data/parquet/bayarea_places.parquet'
  ) TO '/tmp/places.geojsonl' (FORMAT CSV, HEADER false, QUOTE '');
"

# 3. Generate tiles
tippecanoe \
  -o .data/pmtiles/places.pmtiles \
  -zg \
  --cluster-distance=50 \
  --cluster-densest-as-needed \
  --layer=places \
  /tmp/places.geojsonl
```

### Incremental Updates

```bash
# Update specific tiles without regenerating all
tile-join \
  -o .data/pmtiles/updated.pmtiles \
  .data/pmtiles/existing.pmtiles \
  .data/pmtiles/new_data.pmtiles \
  --force
```

## Comparison with Alternatives

| Tool | Pros | Cons |
|------|------|------|
| **Tippecanoe** | Fast, flexible, PMTiles output | CLI only |
| Martin | Real-time from PostGIS | Requires PostGIS |
| pg_tileserv | PostGIS native | No pre-generation |
| t-rex | Rust, configurable | More complex setup |

## Performance Tips

1. **Use newline-delimited GeoJSON** (`.geojsonl`) for large datasets
2. **Pre-filter in DuckDB** before export to reduce data volume
3. **Use `-zg`** for automatic zoom optimization
4. **Enable parallel reading** with `-P` for faster processing
5. **Use `--drop-densest-as-needed`** to handle dense urban areas

## Integration with Existing Stack

```
┌─────────────────────────────────────────────────────────────┐
│                      plat-geo                                │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Data Layer:                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   DuckDB    │  │  NornicDB   │  │  Overture   │         │
│  │  GeoParquet │  │   Graph     │  │   Places    │         │
│  └──────┬──────┘  └─────────────┘  └──────┬──────┘         │
│         │                                  │                 │
│         └──────────────┬──────────────────┘                 │
│                        ▼                                     │
│  Tile Generation:     Tippecanoe                            │
│                        │                                     │
│                        ▼                                     │
│  Tile Serving:   .data/pmtiles/                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  basemap    │  │   places    │  │  buildings  │         │
│  │ (Protomaps) │  │(Tippecanoe) │  │(Tippecanoe) │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Consequences

### Positive
- Generate custom tiles from any GeoJSON/GeoParquet
- PMTiles output matches our basemap format
- Handles millions of features efficiently
- Fine-grained control over zoom, simplification, clustering

### Negative
- Additional build step in data pipeline
- CLI tool requires installation
- Large datasets need significant processing time

### Neutral
- Can regenerate tiles as data updates
- Same tile format for basemaps and overlays
- Integrates with existing DuckDB workflow

## References

- [Tippecanoe GitHub](https://github.com/felt/tippecanoe)
- [Felt Blog: Tippecanoe](https://felt.com/blog/tippecanoe)
- [PMTiles Specification](https://github.com/protomaps/PMTiles)
- [Vector Tile Specification](https://github.com/mapbox/vector-tile-spec)
