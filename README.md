# plat-geo

Geospatial data platform with PMTiles and GeoParquet.

## Architecture

This project uses **Huma** (Go REST framework) + **Datastar** (reactive frontend) with code generation for type safety across the stack.

**[Read the Architecture Docs →](docs/README.md)**

- [Why Huma + Datastar?](docs/architecture.md) - How they work together
- [Code Generation Phases](docs/codegen.md) - The 3-phase gen pipeline

![Editor](docs/screenshots/editor.png)

## Install

```bash
xplat pkg install plat-geo
```

## Run

```bash
geo serve
```

Open http://localhost:8086/editor

## Development

```bash
xplat task dev
```

## Screenshots

![Editor](docs/screenshots/editor.png)
![Viewer](docs/screenshots/viewer.png)
![API Docs](docs/screenshots/api-docs.png)
