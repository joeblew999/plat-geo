package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/joeblew999/plat-geo/internal/server"
)

// Options defines all CLI flags and env vars for the geo server.
// Flags: --host, --port, --data-dir, --web-dir
// Env vars: SERVICE_HOST, SERVICE_PORT, SERVICE_DATA_DIR, SERVICE_WEB_DIR
type Options struct {
	Host    string `doc:"Host to bind to" default:"0.0.0.0"`
	Port    int    `doc:"Port to listen on" short:"p" default:"8086"`
	DataDir string `doc:"Directory for geo data files" default:".data"`
	WebDir  string `doc:"Path to web/ directory" default:"web"`
}

func newServer(opts *Options) *server.Server {
	return server.New(server.Config{
		Host:    opts.Host,
		Port:    fmt.Sprintf("%d", opts.Port),
		DataDir: opts.DataDir,
		WebDir:  opts.WebDir,
	})
}

func main() {
	cli := humacli.New(func(hooks humacli.Hooks, opts *Options) {
		srv := newServer(opts)

		hooks.OnStart(func() {
			addr := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
			displayHost := opts.Host
			if displayHost == "0.0.0.0" {
				displayHost = "localhost"
			}
			baseURL := fmt.Sprintf("http://%s:%d", displayHost, opts.Port)

			fmt.Println()
			fmt.Printf("plat-geo API server starting...\n")
			fmt.Printf("  Server:  %s\n", baseURL)
			fmt.Printf("  Data:    %s\n", opts.DataDir)
			fmt.Println()
			fmt.Printf("  Pages:   %s/viewer, %s/editor\n", baseURL, baseURL)
			fmt.Printf("  Docs:    %s/docs\n", baseURL)
			fmt.Printf("  OpenAPI: %s/openapi.json\n", baseURL)
			fmt.Println()

			if err := http.ListenAndServe(addr, srv); err != nil {
				log.Fatalf("Server error: %v", err)
			}
		})
	})

	cli.Root().Use = "geo"
	cli.Root().Short = "Geographical system for maps and routing"
	cli.Root().Version = "0.1.0"

	// spec subcommand: export OpenAPI spec
	specCmd := &cobra.Command{
		Use:   "spec",
		Short: "Export OpenAPI spec (JSON by default, --yaml for YAML)",
		Run: humacli.WithOptions(func(cmd *cobra.Command, args []string, opts *Options) {
			srv := newServer(opts)
			spec := srv.OpenAPI()

			useYAML, _ := cmd.Flags().GetBool("yaml")

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
		}),
	}
	specCmd.Flags().BoolP("yaml", "y", false, "Output as YAML instead of JSON")
	cli.Root().AddCommand(specCmd)

	// gen-client subcommand: generate Go client SDK via humaclient
	genClientCmd := &cobra.Command{
		Use:   "gen-client",
		Short: "Generate Go client SDK from the API",
		Run: humacli.WithOptions(func(cmd *cobra.Command, args []string, opts *Options) {
			srv := newServer(opts)
			outDir, _ := cmd.Flags().GetString("output")
			if err := srv.GenerateClient(outDir); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating client: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Client SDK generated in %s/\n", outDir)
		}),
	}
	genClientCmd.Flags().StringP("output", "o", "pkg/geoclient", "Output directory for generated client")
	cli.Root().AddCommand(genClientCmd)

	cli.Run()
}
