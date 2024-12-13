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
}
