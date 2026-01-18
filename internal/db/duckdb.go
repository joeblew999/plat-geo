package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/marcboeker/go-duckdb"
)

var (
	instance *sql.DB
	once     sync.Once
	initErr  error
)

// Config holds database configuration.
type Config struct {
	DataDir string
	DBName  string
}

// Get returns the singleton DuckDB connection.
func Get(cfg Config) (*sql.DB, error) {
	once.Do(func() {
		// Create duckdb subdirectory
		duckdbDir := filepath.Join(cfg.DataDir, "duckdb")
		if err := os.MkdirAll(duckdbDir, 0755); err != nil {
			initErr = fmt.Errorf("failed to create duckdb directory: %w", err)
			return
		}

		dbPath := filepath.Join(duckdbDir, cfg.DBName+".duckdb")
		instance, initErr = sql.Open("duckdb", dbPath)
		if initErr != nil {
			return
		}

		// Load extensions
		extensions := []string{"spatial", "parquet"}
		for _, ext := range extensions {
			if _, err := instance.Exec(fmt.Sprintf("INSTALL %s; LOAD %s;", ext, ext)); err != nil {
				// Extensions might already be installed, continue
			}
		}
	})
	return instance, initErr
}

// Close closes the database connection.
func Close() error {
	if instance != nil {
		return instance.Close()
	}
	return nil
}

// Query executes a query and returns rows.
func Query(db *sql.DB, query string, args ...interface{}) (*sql.Rows, error) {
	return db.Query(query, args...)
}

// Exec executes a statement.
func Exec(db *sql.DB, query string, args ...interface{}) (sql.Result, error) {
	return db.Exec(query, args...)
}
