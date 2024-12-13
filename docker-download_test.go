package main

import (
	"testing"
)

func TestGetLayers(t *testing.T) {
	filename := "testdata/container-config.json"
	layerIds, err := GetLayers(filename)
	if err != nil {
		t.Fatalf("Failed to open file %s: %s", filename, err)
	}
	if len(layerIds) == 0 {
		t.Fatalf("Failed to find layer ids")
	}
	layerIds, err = GetLayers("swagger-editor/manifest.json")
	if err == nil {
		t.Fatalf("File does not contain layers")
	}
	t.Logf("Error: %v, %s", layerIds, err)
}
