package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/danielgtaylor/huma/v2/autopatch"
	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
	"github.com/danielgtaylor/humaclient"

	"reflect"

	"github.com/joeblew999/plat-geo/internal/api"
	"github.com/joeblew999/plat-geo/internal/api/editor"
	"github.com/joeblew999/plat-geo/internal/db"
	"github.com/joeblew999/plat-geo/internal/service"
	"github.com/joeblew999/plat-geo/internal/humastar"
)

// Config holds the server configuration.
type Config struct {
	Host    string
	Port    string
	DataDir string
	WebDir  string
}

// Server is the geo HTTP server.
type Server struct {
	config         Config
	mux            *http.ServeMux
	humaAPI        huma.API
	db             *sql.DB
	services       *api.Services
	renderer       *humastar.Renderer
	datastarSchemas []humastar.DatastarSchemaConfig
}

// New creates a new geo server.
func New(cfg Config) *Server {
	mux := http.NewServeMux()

	humaConfig := huma.DefaultConfig("plat-geo API", "1.0.0")
	humaConfig.DocsPath = "" // Custom docs handler below (Scalar + Stoplight)
	humaConfig.Info.Description = "Geospatial data platform API for managing map layers, tiles, and sources."
	humaConfig.Servers = []*huma.Server{
		{URL: fmt.Sprintf("http://%s:%s", cfg.Host, cfg.Port), Description: "Local server"},
	}
	humaConfig.Transformers = append(humaConfig.Transformers, humastar.LinkTransformer())

	humaAPI := humago.New(mux, humaConfig)

	// OpenAPI tags
	humaAPI.OpenAPI().Tags = append(humaAPI.OpenAPI().Tags,
		&huma.Tag{Name: "health", Description: "Health check endpoints"},
		&huma.Tag{Name: "layers", Description: "Layer management operations"},
		&huma.Tag{Name: "sources", Description: "Source file management"},
		&huma.Tag{Name: "tiles", Description: "Tile serving and management"},
		&huma.Tag{Name: "database", Description: "Database query endpoints"},
		&huma.Tag{Name: "editor", Description: "Editor SSE endpoints (Datastar)"},
	)

	services := &api.Services{
		Layer:  service.NewLayerService(cfg.DataDir),
		Tile:   service.NewTileService(cfg.DataDir),
		Source: service.NewSourceService(cfg.DataDir),
	}

	var renderer *humastar.Renderer
	if cfg.WebDir != "" {
		fragmentsDir := filepath.Join(cfg.WebDir, "templates", "fragments")
		if r, err := humastar.NewRenderer(fragmentsDir); err == nil {
			renderer = r
			log.Printf("Loaded fragment templates from %s", fragmentsDir)
		}
	}

	s := &Server{
		config:   cfg,
		mux:      mux,
		humaAPI:  humaAPI,
		services: services,
		renderer: renderer,
	}

	conn, err := db.Get(db.Config{DataDir: cfg.DataDir, DBName: "geo"})
	if err == nil {
		s.db = conn
	}

	s.routes()

	// AutoPatch: auto-generate PATCH from GET+PUT (JSON Merge Patch, JSON Patch, Shorthand)
	autopatch.AutoPatch(s.humaAPI)

	// Datastar schema configs — used for x- extensions + signal codegen
	s.datastarSchemas = []humastar.DatastarSchemaConfig{
		{
			Type:     reflect.TypeFor[service.LayerConfig](),
			Prefix:   "newlayer",
			FormTmpl: "layer-form",
			BasePath: "/api/v1/editor/layers",
			GoPkg:    "editor",
			GoOut:    "internal/api/editor/signals_gen.go",
		},
	}

	// Inject x-datastar, x-signal, x-input, x-sse extensions into OpenAPI schemas
	humastar.InjectExtensions(s.humaAPI, s.datastarSchemas)

	// Register runtime form templates from OpenAPI schemas (replaces static HTML codegen)
	if renderer != nil {
		humastar.RegisterFormTemplates(s.humaAPI, renderer)
	}

	// AutoLinks: auto-generate hypermedia Link headers from OpenAPI paths
	humastar.AutoLinks(s.humaAPI)

	// humaclient: register for SDK generation
	humaclient.Register(s.humaAPI)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Close() error {
	return db.Close()
}

func (s *Server) OpenAPI() any {
	return s.humaAPI.OpenAPI()
}

func (s *Server) GenerateDatastar() error {
	return humastar.GenerateSignals(s.humaAPI, s.datastarSchemas)
}

func (s *Server) GenerateClient(outputDir string) error {
	return humaclient.GenerateClientWithOptions(s.humaAPI, humaclient.Options{
		PackageName:     "geoclient",
		OutputDirectory: outputDir,
	})
}

func (s *Server) routes() {
	// AutoRegister discovers all Register* methods on handler structs
	huma.AutoRegister(s.humaAPI, api.NewAPIHandler(s.services))
	huma.AutoRegister(s.humaAPI, api.NewInfoHandler(s.config.DataDir, s.db != nil))
	huma.AutoRegister(s.humaAPI, api.NewDBHandler(s.db))

	if s.renderer != nil {
		layerHandler := editor.NewLayerHandler(s.services.Layer, s.renderer)
		huma.AutoRegister(s.humaAPI, layerHandler)

		tileHandler := editor.NewTileHandler(s.services.Tile, service.NewTilerService(s.config.DataDir), s.renderer)
		huma.AutoRegister(s.humaAPI, tileHandler)

		sourceHandler := editor.NewSourceHandler(s.services.Source, s.renderer)
		huma.AutoRegister(s.humaAPI, sourceHandler)

		eventHandler := editor.NewEventHandler(s.services.Layer, s.renderer)
		huma.AutoRegister(s.humaAPI, eventHandler)
	}

	// Static files
	if s.config.WebDir != "" {
		staticDir := filepath.Join(s.config.WebDir, "static")
		s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

		tilesDir := filepath.Join(s.config.DataDir, "tiles")
		s.mux.Handle("/tiles/", http.StripPrefix("/tiles/", s.handleTiles(tilesDir)))
	}

	// Pages
	s.mux.HandleFunc("/docs", handleDocs)
	s.mux.HandleFunc("/viewer", s.handleViewer)
	s.mux.HandleFunc("/editor", s.handleEditor)
	s.mux.HandleFunc("/editor-gen", s.handleEditorGen)
	s.mux.HandleFunc("/explorer", s.handleExplorer)
	s.mux.HandleFunc("/", s.handleRoot)
}

func handleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!doctype html>
<html>
  <head>
    <title>plat-geo API Reference</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script id="api-reference" data-url="/openapi.json"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`))
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	// Use auto-generated links from the OpenAPI spec.
	for _, link := range humastar.RootLinks() {
		w.Header().Add("Link", link)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"service": "plat-geo",
		"status":  "running",
	})
}

func (s *Server) handleViewer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	http.ServeFile(w, r, filepath.Join(s.config.WebDir, "templates", "viewer.html"))
}

func (s *Server) handleEditor(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	http.ServeFile(w, r, filepath.Join(s.config.WebDir, "templates", "editor.html"))
}

func (s *Server) handleExplorer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	http.ServeFile(w, r, filepath.Join(s.config.WebDir, "templates", "explorer.html"))
}

func (s *Server) handleEditorGen(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	if s.renderer == nil {
		http.Error(w, "template renderer not initialized", http.StatusInternalServerError)
		return
	}

	// Build page data from OpenAPI spec — no hardcoded signal names or URLs
	pageData := humastar.BuildPageData(s.humaAPI, s.datastarSchemas[0], map[string]any{
		"_editingLayer": false,
		"error":         "",
		"success":       "",
	})

	html, err := s.renderer.RenderPage(
		filepath.Join(s.config.WebDir, "templates", "editor-gen.html"),
		pageData,
	)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
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
