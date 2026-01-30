// pagination.go — HATEOAS pagination via RFC 8288 Link headers.
//
// Response bodies implement the Pager interface to emit next/prev/first/last
// Link headers. Huma middleware reads these and sets the headers automatically.
package humastar

import "fmt"

// Pager is implemented by response bodies that carry pagination metadata.
type Pager interface {
	PaginationLinks(basePath string) []string
}

// PageBody is a generic paginated response envelope.
// Any handler returning PageBody[T] gets automatic pagination Link headers.
type PageBody[T any] struct {
	Total  int `json:"total" doc:"Total number of items"`
	Offset int `json:"offset" doc:"Current offset"`
	Limit  int `json:"limit" doc:"Page size"`
	Data   []T `json:"data" doc:"Items"`
}

// PaginationLinks returns RFC 8288 Link header values for pagination rels.
func (p PageBody[T]) PaginationLinks(basePath string) []string {
	var links []string

	links = append(links, fmt.Sprintf(`<%s?offset=0&limit=%d>; rel="first"`, basePath, p.Limit))

	if p.Offset > 0 {
		prev := p.Offset - p.Limit
		if prev < 0 {
			prev = 0
		}
		links = append(links, fmt.Sprintf(`<%s?offset=%d&limit=%d>; rel="prev"`, basePath, prev, p.Limit))
	}

	if p.Offset+p.Limit < p.Total {
		links = append(links, fmt.Sprintf(`<%s?offset=%d&limit=%d>; rel="next"`, basePath, p.Offset+p.Limit, p.Limit))
	}

	lastOffset := ((p.Total - 1) / p.Limit) * p.Limit
	if lastOffset < 0 {
		lastOffset = 0
	}
	links = append(links, fmt.Sprintf(`<%s?offset=%d&limit=%d>; rel="last"`, basePath, lastOffset, p.Limit))

	return links
}
