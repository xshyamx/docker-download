package main

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

const (
	registry = "registry-1.docker.io"
)

type Layer struct {
	Type   string `json:"mediaType"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}
type Manifest struct {
	Version int     `json:"schemaVersion"`
	Type    string  `json:"mediaType"`
	Config  Layer   `json:"config"`
	Layers  []Layer `json:"layers"`
}

var (
	imageRef string
	verbose  bool
	showHelp bool
	basedir  string
	image    string
	tag      string
)

func main() {
	flag.StringVar(&imageRef, "i", "", "The `image` to download of the form image:tag")
	flag.BoolVar(&verbose, "v", false, "Display `verbose` logs")
	flag.StringVar(&basedir, "o", "", "Target `directory` to download")
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
	image = splits[0]
	if len(splits) > 1 {
		tag = splits[1]
	} else {
		tag = "latest"
	}
	if basedir == "" {
		basedir = path.Base(image)
	}
	bi, err := os.Stat(basedir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("Failed to stat %s: %s", basedir, err)
	}
	if err == nil {
		if bi.IsDir() {
			log.Printf("Directory already exists. Contents will be overwritten")
		} else {
			log.Fatalf("Only output directory supported")
		}
	}
	if verbose {
		log.Printf("%+v", map[string]interface{}{
			"imageRef": imageRef,
			"image":    image,
			"tag":      tag,
			"output":   basedir,
		})
	}

	// 1. access the registry get a 401
	res, err := http.Get(fmt.Sprintf("https://%s/v2/", registry))
	if err != nil {
		log.Fatalf("Get docker authentication endpoint: %s", err)
	}
	// 2. extract authUrl & service
	var authUrl, regService string
	if res.StatusCode == http.StatusUnauthorized {
		parts := strings.Split(res.Header.Get("WWW-Authenticate"), "\"")
		authUrl = parts[1]
		regService = parts[3]
	}
	// 3. Get access token using authUrl & service
	aUrl := fmt.Sprintf("%s?service=%s&scope=repository:%s:pull", authUrl, regService, image)
	if verbose {
		log.Printf("Get access token from %s", aUrl)
	}
	res, err = http.Get(aUrl)
	if err != nil {
		log.Fatalf("Failed to get token: %s", err)
	}
	var tp map[string]interface{}
	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&tp)
	if err != nil {
		log.Fatalf("Failed to read token response: %s", err)
	}
	at, ok := tp["token"]
	if !ok {
		log.Fatalf("Failed to get token")
	}
	accessToken := at.(string)
	if verbose {
		log.Printf("token: %s..", accessToken[0:10])
	}
	// Fetch manifest v2
	mUrl := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, image, tag)
	if verbose {
		log.Printf("Fetching manifest from %s", mUrl)
	}
	req, err := http.NewRequest(http.MethodGet, mUrl, nil)
	if err != nil {
		log.Fatalf("Failed to create manifest request: %s", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to get manifest: %s", err)
	}
	defer res.Body.Close()
	var m Manifest
	err = json.NewDecoder(res.Body).Decode(&m)
	if err != nil {
		log.Fatalf("Failed to parse manifest: %s", err)
	}
	// Create directory
	err = os.MkdirAll(basedir, 0755)
	if err != nil {
		log.Fatalf("Failed to create folder %s: %s", basedir, err)
	}
	m.WriteTo(path.Join(basedir, "manifest.json"))
	// Write config json
	blobUri := fmt.Sprintf("https://%s/v2/%s/blobs/", registry, image)
	err = m.Config.WriteTo(blobUri, accessToken, basedir)
	if err != nil {
		log.Fatalf("Failed to download config: %s", err)
	}
	if verbose {
		log.Printf("Wrote config json file")
	}
	layerIds, err := GetLayers(path.Join(basedir, m.Config.Filename()))
	if err != nil {
		log.Fatalf("Failed to get layer ids: %s", err)
	}
	if verbose {
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
		ldir := path.Join(basedir, layerId)
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
		err = layer.WriteTo(blobUri, accessToken, ldir)
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

// Write manifest to file
func (m Manifest) WriteTo(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Failed to create %s: %w", filename, err)
	}
	return json.NewEncoder(f).Encode(m)
}

func (l Layer) Filename() string {
	if strings.HasSuffix(l.Type, "gzip") {
		return "layer.tar"
	} else {
		return l.Digest[7:] + ".json"
	}
}

// Write the layer to file
func (l Layer) WriteTo(baseUri, accessToken, basedir string) error {
	layerShort := l.Digest[7:17]
	if verbose {
		log.Printf("Downloading from %s", baseUri+l.Digest)
	}
	req, err := http.NewRequest(http.MethodGet, baseUri+l.Digest, nil)
	if err != nil {
		return fmt.Errorf("Failed to request for %s: %w", layerShort, err)
	}
	req.Header.Add("Accept", l.Type)
	req.Header.Add("Authorization", "Bearer "+accessToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to download %s: %w", layerShort, err)
	}
	defer res.Body.Close()
	var reader io.ReadCloser

	if strings.HasSuffix(l.Type, "gzip") {
		reader, err = gzip.NewReader(res.Body)
		if err != nil {
			return fmt.Errorf("Failed to create gunzip reader: %w", err)
		}
	} else {
		reader = res.Body
	}
	filename := l.Filename()
	defer reader.Close()
	if verbose {
		log.Printf("Writing to %s", filename)
	}
	f, err := os.Create(path.Join(basedir, filename))
	defer f.Close()
	n, err := io.Copy(f, reader)
	if err != nil {
		return fmt.Errorf("Failed to copy %s to file: %w", layerShort, err)
	}
	if verbose {
		log.Printf("Copied %d/%d bytes for %s", n, l.Size, layerShort)
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
