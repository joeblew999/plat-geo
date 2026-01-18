package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/joeblew999/plat-geo/internal/api"
	"github.com/joeblew999/plat-geo/internal/api/editor"
	"github.com/joeblew999/plat-geo/internal/db"
	"github.com/joeblew999/plat-geo/internal/service"
	"github.com/joeblew999/plat-geo/internal/templates"
)

// Config holds the server configuration.
type Config struct {
	Host    string
	Port    string
	DataDir string
	WebDir  string // Path to web/ directory for static files and templates
}

// Server is the geo HTTP server.
type Server struct {
	config   Config
	mux      *http.ServeMux
	humaAPI  huma.API
	db       *sql.DB
	services *api.Services
	renderer *templates.Renderer
}

// New creates a new geo server.
func New(cfg Config) *Server {
	mux := http.NewServeMux()

	// Create Huma API with humago (pure stdlib) adapter
	humaConfig := huma.DefaultConfig("plat-geo API", "1.0.0")
	humaConfig.Info.Description = "Geospatial data platform API for managing map layers, tiles, and sources."
	humaConfig.Servers = []*huma.Server{
		{URL: fmt.Sprintf("http://%s:%s", cfg.Host, cfg.Port), Description: "Local server"},
	}
	// Disable $schema property in responses (cleaner JSON)
	humaConfig.CreateHooks = []func(huma.Config) huma.Config{}

	humaAPI := humago.New(mux, humaConfig)

	// Initialize services
	services := &api.Services{
		Layer:  service.NewLayerService(cfg.DataDir),
		Tile:   service.NewTileService(cfg.DataDir),
		Source: service.NewSourceService(cfg.DataDir),
	}

	// Initialize template renderer for editor SSE handlers
	var renderer *templates.Renderer
	if cfg.WebDir != "" {
		fragmentsDir := filepath.Join(cfg.WebDir, "templates", "fragments")
		if r, err := templates.New(fragmentsDir); err == nil {
			renderer = r
			fmt.Printf("Loaded fragment templates from %s\n", fragmentsDir)
		}
	}

	s := &Server{
		config:   cfg,
		mux:      mux,
		humaAPI:  humaAPI,
		services: services,
		renderer: renderer,
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
	// Register Huma REST API routes (OpenAPI-documented JSON endpoints)
	api.RegisterRoutes(s.humaAPI, s.services)

	// Register Editor SSE routes using Huma + Datastar SDK
	if s.renderer != nil {
		layerHandler := editor.NewLayerHandler(s.services.Layer, s.renderer)
		layerHandler.RegisterRoutes(s.humaAPI)

		tileHandler := editor.NewTileHandler(s.services.Tile, s.renderer)
		tileHandler.RegisterRoutes(s.humaAPI)

		sourceHandler := editor.NewSourceHandler(s.services.Source, s.renderer)
		sourceHandler.RegisterRoutes(s.humaAPI)
	}

	// Additional API routes not yet migrated to Huma
	s.mux.HandleFunc("/api/v1/info", s.handleInfo)
	s.mux.HandleFunc("/api/v1/query", s.handleQuery)
	s.mux.HandleFunc("/api/v1/tables", s.handleTables)

	// Legacy editor routes (tile generation, source upload/delete - keep until migrated)
	s.mux.HandleFunc("/api/v1/editor/tiles/generate", s.handleTileGenerate)
	s.mux.HandleFunc("/api/v1/editor/sources/upload", s.handleSourceUpload)
	s.mux.HandleFunc("/api/v1/editor/sources/", s.handleSourceDelete)

	// Static files and templates
	if s.config.WebDir != "" {
		staticDir := filepath.Join(s.config.WebDir, "static")
		s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

		tilesDir := filepath.Join(s.config.DataDir, "tiles")
		s.mux.Handle("/tiles/", http.StripPrefix("/tiles/", s.handleTiles(tilesDir)))
	}

	// Page routes
	s.mux.HandleFunc("/viewer", s.handleViewer)
	s.mux.HandleFunc("/editor", s.handleEditor)
	s.mux.HandleFunc("/", s.handleRoot)
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

	columns, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

func (s *Server) handleViewer(w http.ResponseWriter, r *http.Request) {
	templatePath := filepath.Join(s.config.WebDir, "templates", "viewer.html")
	http.ServeFile(w, r, templatePath)
}

func (s *Server) handleEditor(w http.ResponseWriter, r *http.Request) {
	templatePath := filepath.Join(s.config.WebDir, "templates", "editor.html")
	http.ServeFile(w, r, templatePath)
}

func (s *Server) handleTiles(tilesDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Range")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		http.FileServer(http.Dir(tilesDir)).ServeHTTP(w, r)
	})
}

// handleTileGenerate triggers tile generation from GeoJSON using Tippecanoe.
func (s *Server) handleTileGenerate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	if r.Method != "POST" {
		s.sseError(w, "Method not allowed")
		return
	}

	var signals map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&signals); err != nil {
		s.sseError(w, "Invalid request data: "+err.Error())
		return
	}

	sourceFile := getString(signals, "tilesourcefile")
	outputName := getString(signals, "tileoutputname")
	layerName := getString(signals, "tilelayername")
	minZoom := getString(signals, "tileminzoom")
	maxZoom := getString(signals, "tilemaxzoom")

	if sourceFile == "" {
		s.sseError(w, "Source file is required")
		return
	}
	if outputName == "" {
		s.sseError(w, "Output name is required")
		return
	}

	if layerName == "" {
		layerName = "default"
	}
	if minZoom == "" {
		minZoom = "0"
	}
	if maxZoom == "" {
		maxZoom = "14"
	}

	if !strings.HasSuffix(outputName, ".pmtiles") {
		outputName = outputName + ".pmtiles"
	}

	sourcePath := filepath.Join(s.config.DataDir, "sources", sourceFile)
	outputPath := filepath.Join(s.config.DataDir, "tiles", outputName)

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		s.sseError(w, "Source file not found: "+sourceFile)
		return
	}

	s.sseProgress(w, "Starting tile generation...", 10)

	args := []string{
		"-o", outputPath,
		"-l", layerName,
		"-Z", minZoom,
		"-z", maxZoom,
		"--force",
		"--drop-densest-as-needed",
		sourcePath,
	}

	s.sseProgress(w, "Running Tippecanoe...", 30)

	cmd := exec.Command("tippecanoe", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		errMsg := string(output)
		if errMsg == "" {
			errMsg = err.Error()
		}
		if strings.Contains(err.Error(), "executable file not found") {
			s.sseError(w, "Tippecanoe is not installed. Run 'task tippecanoe:install' to install it.")
		} else {
			s.sseError(w, "Tile generation failed: "+errMsg)
		}
		return
	}

	s.sseProgress(w, "Tiles generated successfully!", 100)

	w.Write([]byte("event: datastar-patch-signals\n"))
	w.Write([]byte("data: signals {\"tileStatus\": \"Complete: " + outputName + "\", \"tileProgress\": 100, \"success\": \"Tiles generated: " + outputName + "\"}\n"))
	w.Write([]byte("\n"))

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) sseProgress(w http.ResponseWriter, status string, progress int) {
	w.Write([]byte("event: datastar-patch-signals\n"))
	w.Write([]byte(fmt.Sprintf("data: signals {\"tileStatus\": \"%s\", \"tileProgress\": %d}\n", status, progress)))
	w.Write([]byte("\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) sseError(w http.ResponseWriter, msg string) {
	w.Write([]byte("event: datastar-patch-signals\n"))
	w.Write([]byte("data: signals {\"error\": \"" + msg + "\"}\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// handleSourceUpload handles GeoJSON file uploads.
func (s *Server) handleSourceUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		s.sseError(w, "Failed to parse upload: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.sseError(w, "No file provided")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".geojson" && ext != ".json" && ext != ".parquet" && ext != ".geoparquet" {
		s.sseError(w, "Only .geojson, .json, .parquet, or .geoparquet files are allowed")
		return
	}

	sourcesDir := filepath.Join(s.config.DataDir, "sources")
	destPath := filepath.Join(sourcesDir, header.Filename)

	dest, err := os.Create(destPath)
	if err != nil {
		s.sseError(w, "Failed to save file: "+err.Error())
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		s.sseError(w, "Failed to write file: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	w.Write([]byte("event: datastar-patch-signals\n"))
	w.Write([]byte("data: signals {\"success\": \"File uploaded: " + header.Filename + "\"}\n"))
	w.Write([]byte("\n"))

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// handleSourceDelete deletes a source file.
func (s *Server) handleSourceDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	prefix := "/api/v1/editor/sources/"
	if !strings.HasPrefix(path, prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	filename := strings.TrimPrefix(path, prefix)
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	sourcesDir := filepath.Join(s.config.DataDir, "sources")
	filePath := filepath.Join(sourcesDir, filename)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete file: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Deleted"))
}
