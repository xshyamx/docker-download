package main

import (
	"testing"
)

func TestGetLayers(t *testing.T) {
	filename := "swagger-editor/4bb03c4be64939387ad2f6730d16f59a9516214d92e6b04295f20bc8043fae7f.json"
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
