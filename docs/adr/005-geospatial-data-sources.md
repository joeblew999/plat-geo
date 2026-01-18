# ADR-005: Geospatial Data Sources

## Status

Proposed

## Context

plat-geo needs geospatial data for:
- Base maps (roads, water, land use)
- Points of interest (places, businesses)
- Administrative boundaries (countries, states, cities)
- Transportation networks (for routing)
- Building footprints

We need to evaluate available open data sources and determine which to use for different purposes.

## Decision Drivers

- **License**: Must allow commercial use
- **Quality**: Accurate, validated data
- **Coverage**: Global or regional availability
- **Format**: Compatible with our stack (GeoParquet, PMTiles, DuckDB)
- **Freshness**: Update frequency
- **Size**: Manageable download and storage

## Data Sources Evaluated

### 1. Overture Maps Foundation (Recommended Primary)

| Aspect | Details |
|--------|---------|
| Provider | Linux Foundation (Amazon, Meta, Microsoft, TomTom) |
| License | CDLA-V2 (permissive) + ODbL (for OSM-derived) |
| Format | GeoParquet |
| Size | Hundreds of GB (global) |
| Updates | Regular releases |

**Data Themes:**
- **Places**: 59M+ POIs with categories
- **Buildings**: 2.3B+ footprints (ML-derived)
- **Transportation**: Road networks, segments
- **Addresses**: Global address points
- **Base**: Land use, water, infrastructure
- **Divisions**: Admin boundaries

**Access:**
```bash
# AWS S3
aws s3 ls s3://overturemaps-us-west-2/release/

# Azure Blob
https://overturemapswestus2.blob.core.windows.net/release/

# DuckDB direct query
SELECT * FROM read_parquet('s3://overturemaps-us-west-2/release/2024-01-17-alpha.0/theme=places/type=place/*');
```

**Pros:**
- Machine-readable, consistent schema
- Pre-processed from 200+ sources
- GeoParquet native (works great with DuckDB)
- Active development

**Cons:**
- Large dataset size
- Some themes still maturing
- Mixed licensing per theme

### 2. OpenStreetMap (OSM)

| Aspect | Details |
|--------|---------|
| Provider | OSM Community |
| License | ODbL (share-alike) |
| Format | PBF, XML (convert to GeoParquet) |
| Size | ~70GB (planet PBF) |
| Updates | Minutely/daily diffs |

**Tools:**
- **QuackOSM**: DuckDB-based OSM to GeoParquet converter
- **osm2pgsql**: PostgreSQL importer
- **osmium**: Processing tool

**Pros:**
- Most detailed map data
- Frequent updates
- Strong community validation
- Rich tagging system

**Cons:**
- Requires processing/conversion
- ODbL share-alike requirements
- Inconsistent tagging globally

### 3. Protomaps Basemaps (Recommended for Tiles)

| Aspect | Details |
|--------|---------|
| Provider | Protomaps |
| License | ODbL (OSM-derived) |
| Format | PMTiles |
| Size | ~100GB (planet, all zooms) |
| Updates | Daily builds |

**Access:**
```bash
# Download planet
curl -O https://build.protomaps.com/20250115.pmtiles

# Extract region
pmtiles extract https://build.protomaps.com/20250115.pmtiles \
  region.pmtiles --bbox=-122.5,37.5,-122.0,38.0
```

**Pros:**
- Ready-to-use vector tiles
- Daily OSM updates
- Extract any region
- Single-file deployment

**Cons:**
- Requires attribution
- Large full-planet file
- Pre-styled (less flexible)

### 4. Natural Earth

| Aspect | Details |
|--------|---------|
| Provider | Community/Cartographers |
| License | Public Domain |
| Format | Shapefile, GeoJSON |
| Size | Small (MB range) |
| Updates | Periodic |

**Scales:**
- 1:10m - High detail
- 1:50m - Medium detail
- 1:110m - Low detail (overview)

**Data Available:**
- Country boundaries (Admin 0)
- State/Province boundaries (Admin 1)
- Populated places
- Physical features (rivers, lakes)
- Coastlines

**Pros:**
- Public domain (no attribution required)
- Clean, cartographer-curated
- Small file sizes
- Multiple scales

**Cons:**
- Less detail than OSM
- Infrequent updates
- No POIs

### 5. OpenAddresses

| Aspect | Details |
|--------|---------|
| Provider | OpenAddresses.io |
| License | Varies by source |
| Format | CSV, GeoJSON |
| Coverage | 1B+ addresses |
| Updates | Continuous |

**Pros:**
- Authoritative address data
- Global coverage
- Machine-readable

**Cons:**
- Mixed licensing
- Variable quality by region

## Comparison Matrix

| Source | POIs | Roads | Buildings | Boundaries | Tiles | License |
|--------|------|-------|-----------|------------|-------|---------|
| Overture Maps | ✅ | ✅ | ✅ | ✅ | ❌ | CDLA/ODbL |
| OpenStreetMap | ✅ | ✅ | ✅ | ✅ | ❌ | ODbL |
| Protomaps | ❌ | ✅ | ✅ | ✅ | ✅ | ODbL |
| Natural Earth | ❌ | ❌ | ❌ | ✅ | ❌ | Public Domain |
| OpenAddresses | ❌ | ❌ | ❌ | ❌ | ❌ | Mixed |

## Decision

### Recommended Data Stack

| Purpose | Source | Format |
|---------|--------|--------|
| **Base tiles** | Protomaps | PMTiles |
| **POIs/Places** | Overture Maps | GeoParquet |
| **Buildings** | Overture Maps | GeoParquet |
| **Roads/Routing** | Overture Maps | GeoParquet |
| **Boundaries** | Natural Earth | GeoParquet |
| **Detailed edits** | OpenStreetMap | GeoParquet via QuackOSM |

### Data Pipeline

```
┌─────────────────────────────────────────────────────────────┐
│                     Data Sources                             │
├─────────────┬─────────────┬─────────────┬──────────────────┤
│  Protomaps  │  Overture   │   Natural   │       OSM        │
│   (tiles)   │   (POIs)    │   Earth     │   (detailed)     │
└──────┬──────┴──────┬──────┴──────┬──────┴────────┬─────────┘
       │             │             │               │
       ▼             ▼             ▼               ▼
┌─────────────────────────────────────────────────────────────┐
│                    .data/ directory                          │
├─────────────┬─────────────┬─────────────┬──────────────────┤
│  pmtiles/   │  parquet/   │  parquet/   │    parquet/      │
│  basemap    │  places     │  boundaries │    osm_*         │
└──────┬──────┴──────┬──────┴──────┬──────┴────────┬─────────┘
       │             │             │               │
       ▼             ▼             ▼               ▼
┌─────────────────────────────────────────────────────────────┐
│                      DuckDB                                  │
│  - Load GeoParquet files                                    │
│  - Spatial queries                                          │
│  - Join across sources                                      │
└─────────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────┐
│                     NornicDB                                 │
│  - Graph relationships                                      │
│  - Vector search                                            │
│  - Routing (from road network)                              │
└─────────────────────────────────────────────────────────────┘
```

### Initial Dataset Tasks

```yaml
# Taskfile additions for data acquisition
tasks:
  data:download:basemap:
    desc: Download Protomaps basemap for region
    cmds:
      - pmtiles extract https://build.protomaps.com/$(date +%Y%m%d).pmtiles \
          {{.DATA_DIR}}/pmtiles/basemap.pmtiles --bbox={{.BBOX}}

  data:download:overture:places:
    desc: Download Overture places for region
    cmds:
      - |
        {{.DUCKDB}} -c "
          COPY (
            SELECT * FROM read_parquet('s3://overturemaps-us-west-2/release/*/theme=places/type=place/*')
            WHERE bbox.xmin > {{.MIN_LNG}} AND bbox.xmax < {{.MAX_LNG}}
              AND bbox.ymin > {{.MIN_LAT}} AND bbox.ymax < {{.MAX_LAT}}
          ) TO '{{.DATA_DIR}}/parquet/places.parquet' (FORMAT PARQUET);
        "

  data:download:natural-earth:
    desc: Download Natural Earth boundaries
    cmds:
      - curl -o /tmp/ne.zip https://naciscdn.org/naturalearth/10m/cultural/ne_10m_admin_0_countries.zip
      - unzip -o /tmp/ne.zip -d /tmp/ne
      - ogr2ogr -f Parquet {{.DATA_DIR}}/parquet/countries.parquet /tmp/ne/*.shp
```

## Licensing Summary

| Source | License | Attribution Required | Share-Alike |
|--------|---------|---------------------|-------------|
| Overture (non-OSM) | CDLA-V2 | No | No |
| Overture (OSM-derived) | ODbL | Yes | Yes |
| OpenStreetMap | ODbL | Yes | Yes |
| Protomaps | ODbL | Yes | Yes |
| Natural Earth | Public Domain | No | No |

**Attribution text for ODbL:**
```
© OpenStreetMap contributors
```

## Storage Estimates

| Dataset | Region | Approx Size |
|---------|--------|-------------|
| Protomaps basemap | Planet | ~100GB |
| Protomaps basemap | US | ~15GB |
| Protomaps basemap | City | ~100MB |
| Overture places | Planet | ~50GB |
| Overture places | Country | ~1-5GB |
| Natural Earth | Planet | ~500MB |
| OSM (raw PBF) | Planet | ~70GB |

## Consequences

### Positive
- Multiple complementary data sources
- GeoParquet enables efficient DuckDB queries
- PMTiles simplifies tile serving
- Clear licensing for commercial use

### Negative
- Multiple sources require synchronization
- Large storage for full planet data
- Some data overlap between sources
- Attribution requirements for ODbL

### Neutral
- Can start with regional extracts
- Pipeline can grow incrementally
- Format conversions are well-supported

## References

- [Overture Maps Documentation](https://docs.overturemaps.org/getting-data/)
- [Overture Maps on AWS](https://registry.opendata.aws/overture/)
- [Protomaps Basemap Downloads](https://docs.protomaps.com/basemaps/downloads)
- [GeoParquet Specification](https://geoparquet.org/)
- [Natural Earth Data](https://www.naturalearthdata.com/)
- [OpenStreetMap](https://www.openstreetmap.org/)
- [QuackOSM](https://github.com/kraina-ai/quackosm)
- [Why Overture Chose GeoParquet](https://overturemaps.org/blog/2025/why-we-chose-geoparquet-breaking-down-data-silos-at-overture-maps/)
