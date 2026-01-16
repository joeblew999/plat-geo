package server

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/joeblew999/plat-geo/internal/db"
)

// Config holds the server configuration.
type Config struct {
	Host    string
	Port    string
	DataDir string
}

// Server is the geo HTTP server.
type Server struct {
	config Config
	mux    *http.ServeMux
	db     *sql.DB
}

// New creates a new geo server.
func New(cfg Config) *Server {
	s := &Server{
		config: cfg,
		mux:    http.NewServeMux(),
	}

	// Initialize DuckDB connection
	conn, err := db.Get(db.Config{
		DataDir: cfg.DataDir,
		DBName:  "geo",
	})
	if err == nil {
		s.db = conn
	}

	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Close closes server resources.
func (s *Server) Close() error {
	return db.Close()
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/v1/info", s.handleInfo)
	s.mux.HandleFunc("/api/v1/query", s.handleQuery)
	s.mux.HandleFunc("/api/v1/tables", s.handleTables)
	s.mux.HandleFunc("/", s.handleRoot)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := "ok"
	if s.db == nil {
		status = "degraded"
	}
	json.NewEncoder(w).Encode(map[string]string{
		"status": status,
	})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":     "plat-geo",
		"version":  "0.1.0",
		"data_dir": s.config.DataDir,
		"db":       s.db != nil,
		"features": []string{
			"geoparquet",
			"pmtiles",
			"routing",
			"duckdb",
		},
	})
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	rows, err := s.db.Query(req.Query)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Collect results
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"columns": columns,
		"rows":    results,
		"count":   len(results),
	})
}

func (s *Server) handleTables(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	rows, err := s.db.Query("SHOW TABLES")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tables = append(tables, name)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tables": tables,
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"service": "plat-geo",
		"status":  "running",
	})
}
