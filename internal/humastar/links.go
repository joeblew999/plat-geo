package humastar

import (
	"fmt"
	"path"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// linkMap stores the generated RFC 8288 link headers keyed by operation path.
var linkMap map[string][]string

// AutoLinks walks the OpenAPI spec and auto-generates hypermedia links.
// Call after all routes (including AutoPatch) are registered.
func AutoLinks(api huma.API) {
	oapi := api.OpenAPI()
	linkMap = map[string][]string{}

	// Collect collection paths (no {param}) and item paths (have {param}),
	// skipping editor (Datastar SSE) endpoints.
	type pathInfo struct {
		path string
		tags []string
	}
	var collections, items []pathInfo

	for p, pi := range oapi.Paths {
		tags := primaryTags(pi)
		if hasTag(tags, "editor") {
			continue
		}
		info := pathInfo{path: p, tags: tags}
		if strings.Contains(p, "{") {
			items = append(items, info)
		} else {
			collections = append(collections, info)
		}
	}

	// 1. Item → collection (rel="collection") + up (rel="up")
	for _, item := range items {
		parent := path.Dir(item.path)
		if _, ok := oapi.Paths[parent]; ok {
			addLink(item.path, parent, "collection")
			addLink(item.path, parent, "up")
		}
	}

	// 2. Collection → item template (rel="item")
	for _, coll := range collections {
		for _, item := range items {
			if path.Dir(item.path) == coll.path {
				addLink(coll.path, item.path, "item")
			}
		}
	}

	// 2b. Collection → entry point (rel="up")
	for _, coll := range collections {
		if coll.path == "/health" {
			continue
		}
		addLink(coll.path, "/health", "up")
	}

	// 2c. Collection → search (rel="search") via query endpoint
	if _, ok := oapi.Paths["/api/v1/query"]; ok {
		for _, coll := range collections {
			addLink(coll.path, "/api/v1/query", "search")
		}
	}

	// 3. Action rels from HTTP methods (IANA standard)
	for _, coll := range collections {
		pi := oapi.Paths[coll.path]
		if pi.Post != nil {
			addLink(coll.path, coll.path, "create-form")
		}
	}
	for _, item := range items {
		pi := oapi.Paths[item.path]
		if pi.Put != nil || pi.Patch != nil {
			addLink(item.path, item.path, "edit")
			addLink(item.path, item.path, "edit-form")
		}
	}

	// 4. Cross-link collections sharing a tag
	for i, a := range collections {
		for j, b := range collections {
			if i == j {
				continue
			}
			if sharedTag(a.tags, b.tags) != "" {
				rel := lastSegment(b.path)
				addLink(a.path, b.path, rel)
			}
		}
	}

	// 5. Entry points: /health gets links to all collections + IANA discovery rels
	entryPaths := []string{"/health"}
	for _, ep := range entryPaths {
		for _, coll := range collections {
			if coll.path == ep {
				continue
			}
			rel := lastSegment(coll.path)
			addLink(ep, coll.path, rel)
		}
		addLink(ep, "/openapi.json", "describedby")
		addLink(ep, "/openapi.json", "service-desc")
		addLink(ep, "/docs", "service-doc")

		// search rel: link to POST-based query endpoint (IANA "search")
		if _, ok := oapi.Paths["/api/v1/query"]; ok {
			addLink(ep, "/api/v1/query", "search")
		}
	}

	// 6. describedby per-resource: link to JSON Schema fragment in OpenAPI spec
	for _, all := range [][]pathInfo{collections, items} {
		for _, pi := range all {
			schemaRef := getResponseSchemaRef(oapi.Paths[pi.path])
			if schemaRef != "" {
				addLink(pi.path, "/openapi.json#/components/schemas/"+schemaRef, "describedby")
			}
		}
	}

	// 7. Inject OpenAPI Response.Links on operations
	for p, pi := range oapi.Paths {
		headers, ok := linkMap[p]
		if !ok {
			continue
		}
		for _, op := range operationsOf(pi) {
			if op == nil {
				continue
			}
			injectResponseLinks(op, headers)
		}
	}
}

// LinkTransformer returns a Huma Transformer that injects auto-generated
// RFC 8288 Link headers at runtime.
func LinkTransformer() huma.Transformer {
	return func(ctx huma.Context, status string, v any) (any, error) {
		op := ctx.Operation()
		if op == nil {
			return v, nil
		}

		if linkMap != nil {
			for _, link := range linkMap[op.Path] {
				ctx.AppendHeader("Link", link)
			}
		}

		// Item endpoints get a self link with the resolved URL.
		if strings.Contains(op.Path, "{") {
			ctx.AppendHeader("Link", fmt.Sprintf(`<%s>; rel="self"`, ctx.URL().Path))
		}

		// Pagination links from response body.
		if p, ok := v.(Pager); ok {
			for _, link := range p.PaginationLinks(ctx.URL().Path) {
				ctx.AppendHeader("Link", link)
			}
		}

		// State-dependent action links from response body.
		if a, ok := v.(Actor); ok {
			for _, action := range a.Actions() {
				ctx.AppendHeader("Link", action.LinkHeader())
			}
		}

		return v, nil
	}
}

// RootLinks returns the auto-generated Link headers for the root path,
// for use by non-Huma handlers.
func RootLinks() []string {
	if linkMap == nil {
		return nil
	}
	// Reuse /health links for the root (same entry point concept).
	return linkMap["/health"]
}

// --- helpers ---

func addLink(from, to, rel string) {
	val := fmt.Sprintf(`<%s>; rel="%s"`, to, rel)
	for _, existing := range linkMap[from] {
		if existing == val {
			return
		}
	}
	linkMap[from] = append(linkMap[from], val)
}

func primaryTags(pi *huma.PathItem) []string {
	for _, op := range operationsOf(pi) {
		if op != nil && len(op.Tags) > 0 {
			return op.Tags
		}
	}
	return nil
}

func operationsOf(pi *huma.PathItem) []*huma.Operation {
	return []*huma.Operation{pi.Get, pi.Post, pi.Put, pi.Patch, pi.Delete}
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func sharedTag(a, b []string) string {
	for _, at := range a {
		for _, bt := range b {
			if at == bt {
				return at
			}
		}
	}
	return ""
}

func lastSegment(p string) string {
	parts := strings.Split(strings.TrimRight(p, "/"), "/")
	return parts[len(parts)-1]
}

// injectResponseLinks adds OpenAPI Link objects to the operation's success response
// so the spec itself documents the relationships.
func injectResponseLinks(op *huma.Operation, headers []string) {
	if op.Responses == nil {
		return
	}
	// Find the success response (2xx).
	var resp *huma.Response
	for code, r := range op.Responses {
		if strings.HasPrefix(code, "2") {
			resp = r
			break
		}
	}
	if resp == nil {
		return
	}
	if resp.Links == nil {
		resp.Links = map[string]*huma.Link{}
	}
	for _, h := range headers {
		rel, href := parseLinkHeader(h)
		if rel == "" {
			continue
		}
		resp.Links[rel] = &huma.Link{
			OperationRef: href,
			Description:  fmt.Sprintf("Related: %s", rel),
		}
	}
}

func getResponseSchemaRef(pi *huma.PathItem) string {
	if pi.Get == nil || pi.Get.Responses == nil {
		return ""
	}
	for code, resp := range pi.Get.Responses {
		if !strings.HasPrefix(code, "2") || resp.Content == nil {
			continue
		}
		for _, mt := range resp.Content {
			if mt.Schema != nil && mt.Schema.Ref != "" {
				// Extract schema name from $ref like "#/components/schemas/Foo"
				parts := strings.Split(mt.Schema.Ref, "/")
				return parts[len(parts)-1]
			}
		}
	}
	return ""
}

func parseLinkHeader(h string) (rel, href string) {
	// Parse `<url>; rel="name"` format.
	parts := strings.SplitN(h, ";", 2)
	if len(parts) < 2 {
		return "", ""
	}
	href = strings.Trim(strings.TrimSpace(parts[0]), "<>")
	relPart := strings.TrimSpace(parts[1])
	if strings.HasPrefix(relPart, `rel="`) {
		rel = strings.Trim(relPart[4:], `"`)
	}
	return rel, href
}
