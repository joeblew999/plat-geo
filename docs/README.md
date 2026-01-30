# plat-geo Architecture Documentation

This documentation explains the synergistic architecture combining **Huma** (Go HTTP framework) and **Datastar** (reactive frontend library) with code generation.

## Quick Links

| Document | Description |
|----------|-------------|
| [Architecture Overview](./architecture.md) | How Huma + Datastar work together |
| [Hypermedia & HATEOAS](./hypermedia.md) | RFC 8288 Link headers, state-dependent actions, explorer mesh |
| [Huma Guide](./huma.md) | REST API patterns and SSE streaming |
| [Datastar Guide](./datastar.md) | Frontend signals and DOM updates |
| [Code Generation](./codegen.md) | The three phases of code generation |

## Why This Architecture?

**The Problem**: Traditional SPAs require maintaining separate frontend/backend codebases with manual synchronization of types, API contracts, and state management.

**Our Solution**: A single source of truth in Go structs that generates:
1. OpenAPI specs for REST clients
2. TypeScript types for type safety
3. Signal parsers for Datastar form binding

## The Synergy

```
┌─────────────────┐     SSE (HTML + Signals)     ┌─────────────────┐
│    Datastar     │ ◄──────────────────────────► │      Huma       │
│   (Frontend)    │                              │    (Backend)    │
└────────┬────────┘                              └────────┬────────┘
         │                                                │
         │  data-signals                                  │  service.LayerConfig
         │  data-bind                                     │  (single source of truth)
         │  @post()/@get()                                │
         │                                                │
         └────────────────────┬───────────────────────────┘
                              │
                      Code Generation
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
         signals_gen.go   openapi.json   api.ts
```

## Key Benefits

1. **Type Safety Across Stack** - Go structs → generated parsers → no runtime type errors
2. **No Manual Sync** - Change a struct field, regenerate, done
3. **Server-Rendered UI** - HTML fragments over SSE, no client-side routing
4. **Full REST API** - Standard JSON endpoints for external clients
5. **Progressive Enhancement** - Works without JS, enhanced with Datastar

## Getting Started

```bash
# Run development server with hot reload
task dev

# Generate all code (signals + OpenAPI + TypeScript)
task gen

# Build production binary
task build
```
