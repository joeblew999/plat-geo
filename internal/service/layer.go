package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// LayerService manages layer configurations.
type LayerService struct {
	dataDir string
	layers  map[string]LayerConfig
	mu      sync.RWMutex
}

// NewLayerService creates a new layer service.
func NewLayerService(dataDir string) *LayerService {
	s := &LayerService{
		dataDir: dataDir,
		layers:  make(map[string]LayerConfig),
	}
	s.loadFromDisk()
	return s
}

// List returns all layer configurations.
func (s *LayerService) List() map[string]LayerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]LayerConfig, len(s.layers))
	for k, v := range s.layers {
		result[k] = v
	}
	return result
}

// Get returns a layer by ID.
func (s *LayerService) Get(id string) (LayerConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	layer, ok := s.layers[id]
	return layer, ok
}

// Create adds a new layer configuration.
func (s *LayerService) Create(layer LayerConfig) (LayerConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate ID from name if not provided
	if layer.ID == "" {
		layer.ID = generateID(layer.Name)
	}

	// Check for duplicate
	if _, exists := s.layers[layer.ID]; exists {
		return LayerConfig{}, fmt.Errorf("layer with ID %q already exists", layer.ID)
	}

	s.layers[layer.ID] = layer
	if err := s.saveToDisk(); err != nil {
		return LayerConfig{}, err
	}

	return layer, nil
}

// Update replaces a layer configuration by ID.
func (s *LayerService) Update(id string, layer LayerConfig) (LayerConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.layers[id]; !exists {
		return LayerConfig{}, fmt.Errorf("layer %q not found", id)
	}

	layer.ID = id
	s.layers[id] = layer
	if err := s.saveToDisk(); err != nil {
		return LayerConfig{}, err
	}

	return layer, nil
}

// Delete removes a layer by ID.
func (s *LayerService) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.layers[id]; !exists {
		return fmt.Errorf("layer %q not found", id)
	}

	delete(s.layers, id)
	return s.saveToDisk()
}

// configFile returns the path to the layers config file.
func (s *LayerService) configFile() string {
	return filepath.Join(s.dataDir, "layers.json")
}

// loadFromDisk loads layer configurations from disk.
func (s *LayerService) loadFromDisk() {
	data, err := os.ReadFile(s.configFile())
	if err != nil {
		return // File doesn't exist yet, start empty
	}

	var layers map[string]LayerConfig
	if err := json.Unmarshal(data, &layers); err != nil {
		return // Invalid JSON, start empty
	}

	s.layers = layers
}

// saveToDisk persists layer configurations to disk.
func (s *LayerService) saveToDisk() error {
	// Ensure data directory exists
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.layers, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.configFile(), data, 0644)
}

// generateID creates a URL-safe ID from a name.
func generateID(name string) string {
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "_")
	// Remove any characters that aren't alphanumeric or underscore
	var result strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
