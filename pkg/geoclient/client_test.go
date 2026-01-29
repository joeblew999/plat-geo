//go:build integration

// Integration test for the generated client SDK.
// Requires a running server: task run
//
// Run: go test -tags=integration ./pkg/geoclient/
package geoclient_test

import (
	"context"
	"os"
	"testing"

	"github.com/joeblew999/plat-geo/pkg/geoclient"
)

func baseURL() string {
	if u := os.Getenv("GEO_BASE_URL"); u != "" {
		return u
	}
	return "http://localhost:8086"
}

func client() geoclient.PlatGeoAPIClient {
	return geoclient.New(baseURL())
}

func TestHealth(t *testing.T) {
	_, body, err := client().Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" {
		t.Fatalf("status=%q, want ok", body.Status)
	}
}

func TestGetInfo(t *testing.T) {
	_, body, err := client().GetInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if body.Name != "plat-geo" {
		t.Fatalf("name=%q, want plat-geo", body.Name)
	}
}

func TestListSources(t *testing.T) {
	_, _, err := client().ListSources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestListTiles(t *testing.T) {
	_, _, err := client().ListTiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestLayerCRUD(t *testing.T) {
	c := client()
	ctx := context.Background()

	_, _, err := c.ListLayers(ctx)
	if err != nil {
		t.Fatal("list:", err)
	}

	_, created, err := c.CreateLayer(ctx, geoclient.LayerConfig{
		Name:           "Integration Test",
		File:           "test.pmtiles",
		GeomType:       "polygon",
		Fill:           "#ff0000",
		Stroke:         "#cc0000",
		Opacity:        0.5,
		DefaultVisible: true,
	})
	if err != nil {
		t.Fatal("create:", err)
	}

	_, layer, err := c.GetLayer(ctx, created.ID)
	if err != nil {
		t.Fatal("get:", err)
	}
	if layer.Name != "Integration Test" {
		t.Fatalf("name=%q, want Integration Test", layer.Name)
	}

	_, _, err = c.DeleteLayer(ctx, created.ID)
	if err != nil {
		t.Fatal("delete:", err)
	}
}

func TestQuery(t *testing.T) {
	_, body, err := client().Query(context.Background(), geoclient.QueryInputBody{
		Query: "SELECT 1 as ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 {
		t.Fatalf("count=%d, want 1", body.Count)
	}
}

func TestListTables(t *testing.T) {
	_, _, err := client().ListTables(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}
