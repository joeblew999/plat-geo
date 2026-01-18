package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joeblew999/plat-geo/internal/server"
	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServer()
	case "spec":
		exportSpec()
	case "version":
		fmt.Println("geo v0.1.0")
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`geo - geographical system for maps and routing

Usage:
  geo <command>

Commands:
  serve           Start the geo server
  spec [--yaml]   Export OpenAPI spec (default: JSON)
  version         Print version information
  help            Show this help message

Environment:
  GEO_PORT       Port to listen on (default: 8086)
  GEO_HOST       Host to bind to (default: 0.0.0.0)
  GEO_DATA_DIR   Directory for geo data files (required)
  GEO_LOG_LEVEL  Log level: debug, info, warn, error (default: info)`)
}

func runServer() {
	port := getEnv("GEO_PORT", "8086")
	host := getEnv("GEO_HOST", "0.0.0.0")
	dataDir := getEnv("GEO_DATA_DIR", ".data")
	webDir := getEnv("GEO_WEB_DIR", "web")

	srv := server.New(server.Config{
		Host:    host,
		Port:    port,
		DataDir: dataDir,
		WebDir:  webDir,
	})

	addr := fmt.Sprintf("%s:%s", host, port)
	displayHost := host
	if host == "0.0.0.0" {
		displayHost = "localhost"
	}
	baseURL := fmt.Sprintf("http://%s:%s", displayHost, port)

	fmt.Println()
	fmt.Printf("plat-geo API server starting...\n")
	fmt.Printf("  Server:  %s\n", baseURL)
	fmt.Printf("  Data:    %s\n", dataDir)
	fmt.Println()
	fmt.Printf("  Pages:   %s/viewer, %s/editor\n", baseURL, baseURL)
	fmt.Printf("  Docs:    %s/docs\n", baseURL)
	fmt.Printf("  OpenAPI: %s/openapi.json\n", baseURL)
	fmt.Println()

	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func exportSpec() {
	// Create server without starting it to get the OpenAPI spec
	srv := server.New(server.Config{
		Host:    "localhost",
		Port:    "8086",
		DataDir: ".data",
	})

	spec := srv.OpenAPI()

	// Check for --yaml flag
	useYAML := false
	for _, arg := range os.Args[2:] {
		if arg == "--yaml" || arg == "-y" {
			useYAML = true
			break
		}
	}

	var output []byte
	var err error

	if useYAML {
		output, err = yaml.Marshal(spec)
	} else {
		output, err = json.MarshalIndent(spec, "", "  ")
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling spec: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(output))
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
