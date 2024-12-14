package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"
)

const (
	registryUri = "https://registry-1.docker.io"
)

type Config struct {
	image   string
	tag     string
	verbose bool
	basedir string
	os      string
	arch    string
}

var (
	config Config
)

func main() {
	var (
		imageRef string
		showHelp bool
	)
	flag.StringVar(&imageRef, "i", "", "The `image` to download of the form image:tag")
	flag.BoolVar(&config.verbose, "v", false, "Display `verbose` logs")
	flag.StringVar(&config.basedir, "out", "", "Target `directory` to download")
	flag.StringVar(&config.os, "os", "linux", "Operating system")
	flag.StringVar(&config.arch, "arch", "amd64", "Architecture")
	flag.BoolVar(&showHelp, "h", false, "Show command line `help`")
	flag.Parse()
	if showHelp {
		flag.PrintDefaults()
		return
	}
	if imageRef == "" {
		log.Fatalf("Not a valid image reference")
	}
	splits := strings.Split(imageRef, ":")
	config.image = splits[0]
	if len(splits) > 1 {
		config.tag = splits[1]
	} else {
		config.tag = "latest"
	}
	if len(strings.Split(config.image, "/")) == 1 {
		config.image = "library/" + config.image
	}
	if config.basedir == "" {
		config.basedir = path.Base(config.image)
	}
	bi, err := os.Stat(config.basedir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("Failed to stat %s: %s", config.basedir, err)
	}
	if err == nil {
		if bi.IsDir() {
			log.Printf("Directory already exists. Contents will be overwritten")
		} else {
			log.Fatalf("Only output directory supported")
		}
	}
	if config.verbose {
		log.Printf("%+v", config)
	}
	dc := NewClient(registryUri, config)

	// 1. access the registry get a 401
	err = dc.PreAuth()
	if err != nil {
		log.Fatalf("Get docker authentication endpoint: %s", err)
	}
	// 2. extract authUrl & service
	err = dc.Authenticate()
	if err != nil {
		log.Fatalf("Failed to get token: %s", err)
	}
	// Fetch manifest v2
	m, err := dc.Manifest()
	if err != nil {
		log.Fatalf("Failed to get manifest: %s", err)
	}
	// Create directory
	err = os.MkdirAll(config.basedir, 0755)
	if err != nil {
		log.Fatalf("Failed to create folder %s: %s", config.basedir, err)
	}

	m.WriteTo(path.Join(config.basedir, "manifest.json"))
	// Write config json
	dc.WriteLayer(m.Config)
	containerConfigJson := path.Join(config.basedir, m.Config.Filename())
	layerIds, err := GetLayers(containerConfigJson)
	if err != nil {
		log.Fatalf("Failed to get layer ids: %s", err)
	}
	if config.verbose {
		log.Printf("Loaded %d layers from config json", len(layerIds))
	}
	// Write layers
	parentId := ""
	for i, layer := range m.Layers {
		layerId := layerIds[i]
		layerJson := map[string]string{
			"id":     layerId,
			"parent": parentId,
		}
		parentId = layerId
		ldir := path.Join(config.basedir, layerId)
		os.MkdirAll(ldir, 0755)
		// Write VERSION file
		err := WriteVersion(ldir)
		if err != nil {
			log.Printf("Failed to write version: %s", err)
		}
		// Write json file
		err = WriteJson(ldir, layerJson)
		if err != nil {
			log.Printf("Failed to write json: %s", err)
		}
		// Write layer.tar
		layer.id = layerId
		err = dc.WriteLayer(layer)
		if err != nil {
			log.Printf("Failed to download layer %s: %s", layer.Digest[7:], err)
		}
	}
}

// Write layer json
func WriteJson(destDir string, content map[string]string) error {
	f, err := os.Create(path.Join(destDir, "json"))
	defer f.Close()
	if err != nil {
		return fmt.Errorf("Failed to create json file: %w", err)
	}
	err = json.NewEncoder(f).Encode(content)
	if err != nil {
		return fmt.Errorf("Failed to write json file: %w", err)
	}
	return nil
}

// Write version file
func WriteVersion(destDir string) error {
	f, err := os.Create(path.Join(destDir, "VERSION"))
	defer f.Close()
	if err != nil {
		return fmt.Errorf("Failed to create version file: %w", err)
	}
	_, err = io.WriteString(f, "1.0")
	if err != nil {
		return fmt.Errorf("Failed to write version file: %w", err)
	}
	return nil
}

// Get the layer ids from the image config json
func GetLayers(filename string) ([]string, error) {
	var layerIds []string
	f, err := os.Open(filename)
	if err != nil {
		return layerIds, fmt.Errorf("Failed to open file %s: %w", filename, err)
	}
	defer f.Close()
	var m map[string]interface{}
	err = json.NewDecoder(f).Decode(&m)
	if err != nil {
		return layerIds, fmt.Errorf("Failed to decode json: %w", err)
	}
	rootfs, ok := m["rootfs"]
	if ok {
		rfs := rootfs.(map[string]interface{})
		diff_ids, ok := rfs["diff_ids"]
		if ok {
			dids := diff_ids.([]interface{})
			layerIds = make([]string, len(dids))
			for i, iid := range dids {
				layerIds[i] = iid.(string)[7:]
			}
			return layerIds, nil
		}
	}
	return layerIds, fmt.Errorf("Failed to find layer ids")
}
