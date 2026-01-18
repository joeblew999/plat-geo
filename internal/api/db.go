package api

import (
	"context"
	"database/sql"

	"github.com/danielgtaylor/huma/v2"
)

// DBHandler handles database-related endpoints.
type DBHandler struct {
	db *sql.DB
}

// NewDBHandler creates a new database handler.
func NewDBHandler(db *sql.DB) *DBHandler {
	return &DBHandler{db: db}
}

// RegisterRoutes registers database routes with Huma.
func (h *DBHandler) RegisterRoutes(api huma.API) {
	huma.Get(api, "/api/v1/tables", h.ListTables)
	huma.Post(api, "/api/v1/query", h.Query)
}

// TablesOutput is the response for listing tables.
type TablesOutput struct {
	Body struct {
		Tables []string `json:"tables" doc:"List of table names"`
	}
}

// ListTables returns all DuckDB tables.
func (h *DBHandler) ListTables(ctx context.Context, input *struct{}) (*TablesOutput, error) {
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

	return &TablesOutput{
		Body: struct {
			Tables []string `json:"tables" doc:"List of table names"`
		}{
			Tables: tables,
		},
	}, nil
}

// QueryInput is the input for SQL queries.
type QueryInput struct {
	Body struct {
		Query string `json:"query" required:"true" doc:"SQL query to execute"`
	}
}

// QueryOutput is the response for SQL queries.
type QueryOutput struct {
	Body struct {
		Columns []string                 `json:"columns" doc:"Column names"`
		Rows    []map[string]interface{} `json:"rows" doc:"Query results"`
		Count   int                      `json:"count" doc:"Number of rows returned"`
	}
}

// Query executes a SQL query against DuckDB.
func (h *DBHandler) Query(ctx context.Context, input *QueryInput) (*QueryOutput, error) {
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

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	return &QueryOutput{
		Body: struct {
			Columns []string                 `json:"columns" doc:"Column names"`
			Rows    []map[string]interface{} `json:"rows" doc:"Query results"`
			Count   int                      `json:"count" doc:"Number of rows returned"`
		}{
			Columns: columns,
			Rows:    results,
			Count:   len(results),
		},
	}, nil
}
