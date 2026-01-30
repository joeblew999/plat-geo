# Hypermedia & HATEOAS

This project implements a **fully hypermedia-driven REST API** using RFC 8288 Link headers. Every response tells the client what it can do next — no hardcoded URLs, no out-of-band documentation required.

The `internal/humastar` package makes this automatic. Register your Huma routes normally; the hypermedia layer wires everything together.

## What the client sees

Fetch any resource and the `Link` headers describe the entire API graph:

```
GET /health

Link: </api/v1/layers>; rel="layers"
Link: </api/v1/sources>; rel="sources"
Link: </api/v1/tiles>; rel="tiles"
Link: </openapi.json>; rel="service-desc"
Link: </docs>; rel="service-doc"
Link: </api/v1/query>; rel="search"
```

Follow `rel="layers"`:

```
GET /api/v1/layers

Link: </health>; rel="up"
Link: </api/v1/layers/{id}>; rel="item"
Link: </api/v1/layers>; rel="create-form"
Link: </api/v1/sources>; rel="sources"
Link: </api/v1/tiles>; rel="tiles"
Link: </api/v1/query>; rel="search"
```

Follow an item:

```
GET /api/v1/layers/buildings

Link: </api/v1/layers/buildings>; rel="self"
Link: </api/v1/layers>; rel="collection"
Link: </api/v1/layers>; rel="up"
Link: </api/v1/layers/buildings>; rel="edit"
Link: </api/v1/layers/buildings>; rel="edit-form"
Link: </api/v1/layers/buildings/duplicate>; rel="duplicate"; method="POST"; title="Duplicate"; schema="/schemas/DuplicateInput.json"
Link: </api/v1/layers/buildings>; rel="delete"; method="DELETE"; title="Delete"
Link: </api/v1/layers/buildings/publish>; rel="publish"; method="POST"; title="Publish"
Link: </api/v1/tiles>; rel="related"; title="Tile Files"
```

Publish the layer and fetch it again — the actions change:

```
GET /api/v1/layers/buildings  (now published)

Link: ...
Link: </api/v1/layers/buildings/unpublish>; rel="unpublish"; method="POST"; title="Unpublish"
```

The `publish` link is gone. `unpublish` appeared. **The API tells you what's possible based on the resource's current state.**

## How it works

### 1. Auto-generated structural links (`AutoLinks`)

At startup, `humastar.AutoLinks(api)` walks the OpenAPI spec and generates Link headers for every route:

| Relationship | Rule | Example |
|---|---|---|
| `collection`, `up` | Item → its parent collection | `/layers/buildings` → `/layers` |
| `item` | Collection → item template | `/layers` → `/layers/{id}` |
| `up` | Collection → entry point | `/layers` → `/health` |
| `create-form` | Collection has POST | `/layers` → `/layers` |
| `edit`, `edit-form` | Item has PUT or PATCH | `/layers/{id}` → `/layers/{id}` |
| siblings | Collections sharing a tag | `/layers` ↔ `/sources` ↔ `/tiles` |
| `describedby` | Resource → its JSON Schema | `/layers` → `/openapi.json#/.../LayerConfig` |
| `service-desc` | Entry → OpenAPI spec | `/health` → `/openapi.json` |
| `service-doc` | Entry → docs UI | `/health` → `/docs` |
| `search` | Entry/collection → query | `/health` → `/api/v1/query` |

Zero configuration. Add a route; the links appear.

### 2. Runtime Link injection (`LinkTransformer`)

A Huma transformer runs on every response:

```go
// Registered once at startup
api.UseMiddleware(humastar.LinkTransformer())
```

It does three things:

1. **Static links** — injects the pre-computed `AutoLinks` headers
2. **Pagination** — if the response body implements `Pager`, emits `first`/`prev`/`next`/`last` rels
3. **State-dependent actions** — if the response body implements `Actor`, emits action links

### 3. Pagination links (`PageBody[T]`)

Wrap any list response in `PageBody[T]` and pagination links appear automatically:

```go
type PageBody[T any] struct {
    Total  int `json:"total"`
    Offset int `json:"offset"`
    Limit  int `json:"limit"`
    Data   []T `json:"data"`
}
```

Response headers:

```
Link: </api/v1/sources?offset=0&limit=20>; rel="first"
Link: </api/v1/sources?offset=20&limit=20>; rel="next"
Link: </api/v1/sources?offset=80&limit=20>; rel="last"
```

### 4. State-dependent actions (`Actor` interface)

The real power. Implement `Actions()` on your response body and the available operations change based on resource state:

```go
type LayerBody struct {
    service.LayerConfig
}

func (b LayerBody) Actions() []humastar.Action {
    actions := humastar.ActionsFor(b.ID, layerActions)

    if b.Published {
        actions = append(actions, humastar.Action{
            Rel: "unpublish", Href: fmt.Sprintf("/api/v1/layers/%s/unpublish", b.ID),
            Method: "POST", Title: "Unpublish",
        })
    } else {
        actions = append(actions, humastar.Action{
            Rel: "publish", Href: fmt.Sprintf("/api/v1/layers/%s/publish", b.ID),
            Method: "POST", Title: "Publish",
        })
    }

    // Cross-resource link
    actions = append(actions, humastar.Action{
        Rel: "related", Href: "/api/v1/tiles", Title: "Tile Files",
    })
    return actions
}
```

Each action becomes a Link header with extension parameters:

```
<url>; rel="publish"; method="POST"; title="Publish"; schema="/schemas/..."
```

Extension parameters beyond RFC 8288:
- `method` — HTTP method to use (so the client knows it's POST, not GET)
- `title` — human-readable label
- `schema` — JSON Schema URL for the request body (self-describing forms)

### 5. Reusable action templates (`ActionDef`)

Define actions once, resolve per resource:

```go
var layerActions = []humastar.ActionDef{
    {Rel: "duplicate", Pattern: "/api/v1/layers/%s/duplicate", Method: "POST",
     Title: "Duplicate", Schema: "/schemas/DuplicateInput.json"},
    {Rel: "delete", Pattern: "/api/v1/layers/%s", Method: "DELETE", Title: "Delete"},
}

// In Actions():
actions := humastar.ActionsFor(b.ID, layerActions) // resolves %s → actual ID
```

## The Explorer

Visit `/explorer` to see the hypermedia graph in action. The explorer is a pure HATEOAS client — it fetches `/health`, reads the Link headers, and lets you navigate the entire API without knowing any URLs upfront.

### What the explorer shows

**Mesh graph** — a spatial visualization of the link topology around the current resource:

```
           [health]  ← parent (rel="up")
              ↓
  [sources] → [layers] ← [tiles]    ← YOU ARE HERE
           [+ Add]
              ↓
           [{id}]    ← item template (rel="item")
```

Navigate to an item:

```
           [layers]  ← collection (rel="up")
              ↓
  [tiles] → [buildings]    ← YOU ARE HERE
           [Publish] [Duplicate] [Delete] [Edit]
              ↓
           [styles]  ← sub-resource
```

Click **Publish** — the mesh redraws:

```
           [layers]
              ↓
  [tiles] → [buildings]
           [Unpublish] [Duplicate] [Delete] [Edit]
              ↓
           [styles]
```

The action badges changed because the Link headers changed. No client-side logic decided this — the server told the explorer what's available.

### Explorer features

- **Discovery banner** — shows the URL fetched, link count, and explains RFC 8288
- **Mesh graph** — CSS grid layout with parent/sibling/child spatial zones
- **Action badges** — color-coded by type (create=green, edit=blue, delete=red, custom=purple)
- **Schema-driven forms** — reads OpenAPI schemas to build create/edit forms dynamically
- **Pagination** — follows `first`/`prev`/`next`/`last` rels
- **Raw JSON toggle** — inspect the actual response body
- **Zero hardcoded URLs** — everything discovered via Link headers

## Sub-resources

Layers have a `styles` sub-collection, demonstrating depth in the hypermedia graph:

```
GET /api/v1/layers/{id}/styles      → list styles
POST /api/v1/layers/{id}/styles     → add style
DELETE /api/v1/layers/{id}/styles/{styleId} → remove style
```

The explorer shows styles as a child node below the layer, with its own action badges. Navigate down to styles, then back up to the layer — the mesh rebuilds at each step from Link headers alone.

## Cross-resource links

Each layer emits a `related` rel pointing to `/api/v1/tiles`:

```
Link: </api/v1/tiles>; rel="related"; title="Tile Files"
```

This appears as a sibling node in the explorer mesh, linking the layer to its tile files. Any resource can link to any other resource — the graph is not limited to parent/child hierarchies.

## IANA link relations used

| Rel | Purpose |
|-----|---------|
| `self` | Current resource URL |
| `collection` | Item → parent collection |
| `up` | Navigate to parent |
| `item` | Collection → item template |
| `create-form` | Collection accepts POST |
| `edit` | Item accepts PUT/PATCH |
| `edit-form` | Item has editable form |
| `first`, `prev`, `next`, `last` | Pagination |
| `describedby` | Link to JSON Schema |
| `service-desc` | Link to OpenAPI spec |
| `service-doc` | Link to API docs |
| `search` | Link to query endpoint |
| `related` | Cross-resource link |
| `publish`, `unpublish` | State-dependent actions |
| `duplicate`, `delete` | Resource actions |

## For CLI clients (restish)

[Restish](https://rest.sh/) understands Link headers natively:

```bash
# Follow links from the entry point
restish localhost:8086/health

# Restish shows Link headers in output
# Navigate via rels
restish localhost:8086/api/v1/layers
restish localhost:8086/api/v1/layers/buildings
```

## File map

```
internal/humastar/
├── links.go       # AutoLinks — walks OpenAPI, generates structural Link headers
├── actions.go     # Action, Actor — state-dependent action links
├── resource.go    # ActionDef, ActionsFor — reusable action templates
├── pagination.go  # PageBody[T], Pager — automatic pagination links
└── signals.go     # Datastar signal parsing (separate concern)
```
