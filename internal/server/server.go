package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

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

// OpenAPI returns the OpenAPI spec for code generation.
func (s *Server) OpenAPI() any {
	return s.humaAPI.OpenAPI()
}

func (s *Server) routes() {
	// Register Huma REST API routes (OpenAPI-documented JSON endpoints)
	api.RegisterRoutes(s.humaAPI, s.services)

	// Register Editor SSE routes using Huma + Datastar SDK
	if s.renderer != nil {
		layerHandler := editor.NewLayerHandler(s.services.Layer, s.renderer)
		layerHandler.RegisterRoutes(s.humaAPI)

		tileHandler := editor.NewTileHandler(s.services.Tile, s.renderer)
		tilerService := service.NewTilerService(s.config.DataDir)
		tileHandler.SetTilerService(tilerService)
		tileHandler.RegisterRoutes(s.humaAPI)

		sourceHandler := editor.NewSourceHandler(s.services.Source, s.renderer)
		sourceHandler.RegisterRoutes(s.humaAPI)
	}

	// Register info and database handlers
	infoHandler := api.NewInfoHandler(s.config.DataDir, s.db != nil)
	infoHandler.RegisterRoutes(s.humaAPI)

	dbHandler := api.NewDBHandler(s.db)
	dbHandler.RegisterRoutes(s.humaAPI)

	// Static files and templates
	if s.config.WebDir != "" {
		staticDir := filepath.Join(s.config.WebDir, "static")
		s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

		tilesDir := filepath.Join(s.config.DataDir, "tiles")
		s.mux.Handle("/tiles/", http.StripPrefix("/tiles/", s.handleTiles(tilesDir)))
	}

	// Page routes (must be registered after Huma routes to avoid conflicts)
	s.mux.HandleFunc("/viewer", s.handleViewer)
	s.mux.HandleFunc("/editor", s.handleEditor)
	s.mux.HandleFunc("/", s.handleRoot)
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
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	templatePath := filepath.Join(s.config.WebDir, "templates", "viewer.html")
	http.ServeFile(w, r, templatePath)
}

func (s *Server) handleEditor(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
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
