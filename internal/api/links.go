package api

import (
	"fmt"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// links maps operation paths to their RFC 8288 Link header values.
// Enables restish hypermedia navigation via `restish links <url>`.
var links = map[string][]string{
	"/health": {
		`</api/v1/info>; rel="info"`,
		`</api/v1/layers>; rel="layers"`,
		`</api/v1/sources>; rel="sources"`,
		`</api/v1/tiles>; rel="tiles"`,
	},
	"/api/v1/info": {
		`</health>; rel="health"`,
		`</api/v1/layers>; rel="layers"`,
	},
	"/api/v1/layers": {
		`</api/v1/sources>; rel="sources"`,
		`</api/v1/tiles>; rel="tiles"`,
	},
	"/api/v1/layers/{id}": {
		`</api/v1/layers>; rel="collection"`,
	},
	"/api/v1/sources": {
		`</api/v1/layers>; rel="layers"`,
		`</api/v1/tiles>; rel="tiles"`,
	},
	"/api/v1/tiles": {
		`</api/v1/layers>; rel="layers"`,
		`</api/v1/sources>; rel="sources"`,
	},
	"/api/v1/tables": {
		`</api/v1/query>; rel="query"`,
	},
}

// LinkTransformer returns a Huma Transformer that injects RFC 8288 Link headers.
func LinkTransformer() huma.Transformer {
	return func(ctx huma.Context, status string, v any) (any, error) {
		op := ctx.Operation()
		if op == nil {
			return v, nil
		}

		for _, link := range links[op.Path] {
			ctx.AppendHeader("Link", link)
		}

		// Item endpoints get a self link
		if strings.Contains(op.Path, "{") {
			ctx.AppendHeader("Link", fmt.Sprintf(`<%s>; rel="self"`, ctx.URL().Path))
		}

		return v, nil
	}
}
