# plat-geo

Geospatial data platform with PMTiles, GeoParquet, and a fully hypermedia-driven REST API.

## Run

```bash
air
```

Open http://localhost:8086

| URL | What |
|-----|------|
| `/editor` | Datastar reactive layer editor |
| `/explorer` | HATEOAS hypermedia mesh explorer |
| `/docs` | OpenAPI interactive docs |
| `/health` | Entry point (follow the Link headers) |

## Architecture

- **[Architecture Overview](docs/architecture.md)** — Huma + Datastar synergy
- **[Hypermedia & HATEOAS](docs/hypermedia.md)** — RFC 8288 Link headers, state-dependent actions, the explorer mesh
- **[Code Generation](docs/codegen.md)** — 3-phase gen pipeline
- **[Huma Guide](docs/huma.md)** — REST API patterns
- **[Datastar Guide](docs/datastar.md)** — Reactive frontend
