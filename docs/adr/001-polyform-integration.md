# ADR-001: Integration of EliCDavis/polyform for 3D Geometry Processing

## Status

**Proposed** - Pending deep-dive analysis

## Context

The plat-geo system handles geographical data including GeoParquet and PMTiles formats. There's potential need for advanced 3D geometry processing capabilities beyond what standard geo libraries provide.

[polyform](https://github.com/EliCDavis/polyform) is a Go library that provides:
- Comprehensive 3D geometry primitives and operations
- Mesh processing (PLY, OBJ, GLTF support)
- Mathematical foundations for spatial operations
- Potential use in terrain modeling, 3D visualization, and geometric transformations

## Decision

**To Be Determined** after deep-dive analysis.

## Analysis Required

### 1. Library Capabilities
- [ ] Review core geometry types and operations
- [ ] Evaluate mesh processing capabilities
- [ ] Check GLTF export support quality
- [ ] Assess performance characteristics
- [ ] Review API design and Go idioms

### 2. Use Cases for plat-geo
- [ ] Terrain elevation processing from GeoParquet
- [ ] 3D tile generation (3D Tiles format?)
- [ ] Building/structure extrusion from 2D footprints
- [ ] Contour/isoline generation
- [ ] Point cloud processing

### 3. Integration Points
- [ ] How would polyform integrate with existing geo libraries?
- [ ] Coordinate system transformations (WGS84 <-> local 3D)
- [ ] Data pipeline: GeoParquet -> polyform -> PMTiles/3D Tiles

### 4. Alternatives to Consider
- [ ] go3d - simpler 3D math library
- [ ] Custom implementation for specific use cases
- [ ] External tools (PDAL, CGAL bindings)

## Consequences

**Positive** (if adopted):
- Rich 3D geometry capabilities
- Pure Go implementation (easy deployment)
- Active maintenance

**Negative/Risks**:
- Additional dependency
- Learning curve
- Potential scope creep into 3D features

## References

- Repository: https://github.com/EliCDavis/polyform
- Go package: `github.com/EliCDavis/polyform`

## Notes

*This ADR is a placeholder for future deep-dive analysis. No implementation decisions have been made yet.*
