package api

import (
	"context"
	"database/sql"

	"github.com/danielgtaylor/huma/v2"
)

type DBHandler struct {
	db *sql.DB
}

func NewDBHandler(db *sql.DB) *DBHandler {
	return &DBHandler{db: db}
}

func (h *DBHandler) RegisterRoutes(api huma.API) {
	huma.Get(api, "/api/v1/tables", h.ListTables, huma.OperationTags("database"))
	huma.Post(api, "/api/v1/query", h.Query, huma.OperationTags("database"))
}

type TablesBody struct {
	Tables []string `json:"tables" doc:"List of table names"`
}

type QueryBody struct {
	Columns []string         `json:"columns" doc:"Column names"`
	Rows    []map[string]any `json:"rows" doc:"Query results"`
	Count   int              `json:"count" doc:"Number of rows returned"`
}

func (h *DBHandler) ListTables(ctx context.Context, input *struct{}) (*struct{ Body TablesBody }, error) {
	if h.db == nil {
		return nil, huma.Error503ServiceUnavailable("Database not available")
	}

	rows, err := h.db.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to list tables", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tables = append(tables, name)
		}
	}
	if tables == nil {
		tables = []string{}
	}

	return &struct{ Body TablesBody }{Body: TablesBody{Tables: tables}}, nil
}

func (h *DBHandler) Query(ctx context.Context, input *struct {
	Body struct {
		Query string `json:"query" required:"true" doc:"SQL query to execute"`
	}
}) (*struct{ Body QueryBody }, error) {
	if h.db == nil {
		return nil, huma.Error503ServiceUnavailable("Database not available")
	}

	rows, err := h.db.QueryContext(ctx, input.Body.Query)
	if err != nil {
		return nil, huma.Error400BadRequest("Query failed: " + err.Error())
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to get columns", err)
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}
		row := make(map[string]any)
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}
	if results == nil {
		results = []map[string]any{}
	}

	return &struct{ Body QueryBody }{Body: QueryBody{
		Columns: columns,
		Rows:    results,
		Count:   len(results),
	}}, nil
}
