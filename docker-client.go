package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

type Layer struct {
	id     string
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

type ManifestList struct {
	Version   int            `json:"schemaVersion"`
	Type      string         `json:"mediaType"`
	Manifests []ManifestItem `json:"manifests"`
}
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"architecture"`
}
type ManifestItem struct {
	Layer
	Platform Platform `json:"platform"`
}
type DockerClient struct {
	client  http.Client
	baseUri *url.URL
	authUri *url.URL
	service string
	token   string
	config  Config
}

const (
	HeaderAuthenticate      = "WWW-Authenticate"
	ContentTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	ContentTypeManifest     = "application/vnd.docker.distribution.manifest.v2+json"
)

func NewClient(baseUrl string, config Config) *DockerClient {
	var err error
	c := DockerClient{
		client: http.Client{},
		config: config,
	}
	c.baseUri, err = url.Parse(baseUrl)
	if err != nil {
		log.Fatalf("Failed to parse baseUrl=%s : %s", baseUrl, err)
	}
	return &c
}

func (c *DockerClient) PreAuth() error {
	if c.token != "" {
		// token already exists
		return nil
	}
	if c.service == "" {
		regUrl := c.baseUri.JoinPath("/v2").String()
		res, err := c.client.Get(regUrl)
		if err != nil {
			return fmt.Errorf("Failed to authenticate to registry : %w", err)
		}
		if res.StatusCode == http.StatusUnauthorized {
			authHeader := res.Header.Get(HeaderAuthenticate)
			if authHeader == "" {
				return fmt.Errorf("Failed to get authenticate header")
			}
			parts := strings.Split(authHeader, "\"")
			c.service = parts[3]
			c.authUri, err = url.Parse(parts[1])
			if err != nil {
				return fmt.Errorf("Failed to parse authUrl=%s: %w", parts[1], err)
			}
		}
	}
	return nil
}

func (c *DockerClient) Authenticate() error {
	if c.authUri == nil || c.service == "" || c.config.image == "" {
		return fmt.Errorf("AuthURI/service/image cannot be empty ")
	}
	v := url.Values{}
	v.Add("service", c.service)
	v.Add("scope", fmt.Sprintf("repository:%s:pull", c.config.image))
	aUrl := c.authUri.String() + "?" + v.Encode()
	if c.config.verbose {
		log.Printf("Get access token from %s", aUrl)
	}
	res, err := c.client.Get(aUrl)
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
	c.token = at.(string)
	if c.config.verbose {
		log.Printf("token: %s..", c.token[0:10])
	}
	return nil
}

func (c *DockerClient) Manifest() (Manifest, error) {
	var m Manifest
	mUrl := c.baseUri.JoinPath(fmt.Sprintf("/v2/%s/manifests/%s", c.config.image, c.config.tag)).String()
	if c.config.verbose {
		log.Printf("Fetching manifest from %s", mUrl)
	}
	req, err := http.NewRequest(http.MethodGet, mUrl, nil)
	if err != nil {
		log.Fatalf("Failed to create manifest request: %s", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.client.Do(req)
	if err != nil {
		return m, err
	}
	if res.StatusCode == http.StatusOK {
		// valid
		contentType := res.Header.Get("content-type")
		if contentType == ContentTypeManifestList {
			// manifest list
			var ml ManifestList
			defer res.Body.Close()
			err := json.NewDecoder(res.Body).Decode(&ml)
			if err != nil {
				return m, fmt.Errorf("Failed to decode manifest list: %w", err)
			}
			if len(ml.Manifests) == 0 {
				return m, fmt.Errorf("Manifests list empty")
			}
			found := false
			var mmi ManifestItem
			for _, mi := range ml.Manifests {
				if mi.Platform.OS == c.config.os && mi.Platform.Arch == c.config.arch {
					// found match
					mmi = mi
					found = true
				}
			}
			if !found {
				return m, fmt.Errorf("No matching manifest for (%s / %s)", c.config.os, c.config.arch)
			}

			mUrl := c.baseUri.JoinPath("v2", c.config.image, "manifests", mmi.Digest).String()
			if c.config.verbose {
				log.Printf("manifest url: %s", mUrl)
			}
			req, err := http.NewRequest(http.MethodGet, mUrl, nil)
			if err != nil {
				return m, fmt.Errorf("Failed to create manifest request: %w", err)
			}
			req.Header.Add("Accept", mmi.Type)
			req.Header.Add("Authorization", "Bearer "+c.token)
			res, err := c.client.Do(req)
			if err != nil {
				return m, fmt.Errorf("Failed to download manifest: %w", err)
			}
			if res.StatusCode == http.StatusOK {
				// success
				defer res.Body.Close()
				err := json.NewDecoder(res.Body).Decode(&m)
				if err != nil {
					return m, fmt.Errorf("Failed to decode manifest file: %w", err)
				}
			} else {
				return m, fmt.Errorf("Manifest request failed with %s", res.Status)
			}
		} else if contentType == ContentTypeManifest {
			defer res.Body.Close()
			err := json.NewDecoder(res.Body).Decode(&m)
			if err != nil {
				return m, fmt.Errorf("Failed to decode manifest item: %w", err)
			}
		} else {
			return m, fmt.Errorf("Not a valid content type: %s", contentType)
		}
	}
	return m, nil
}

// Write the layer to file
func (c *DockerClient) WriteLayer(l Layer) error {
	if l.Digest == "" {
		return fmt.Errorf("Layer digest is empty")
	}
	layerShort := l.Digest[7:17]
	bUrl := c.baseUri.JoinPath("v2", c.config.image, "blobs", l.Digest).String()
	if c.config.verbose {
		log.Printf("Fetching blob layer from %s", bUrl)
	}
	req, err := http.NewRequest(http.MethodGet, bUrl, nil)
	if err != nil {
		return fmt.Errorf("Failed to create blob request for %s: %w", layerShort, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to download blob %s: %w", layerShort, err)
	}
	defer res.Body.Close()
	var (
		reader   io.ReadCloser
		filename string
	)

	if strings.HasSuffix(l.Type, "gzip") {
		reader, err = gzip.NewReader(res.Body)
		if err != nil {
			return fmt.Errorf("Failed to create gunzip reader: %w", err)
		}
		if l.id == "" {
			return fmt.Errorf("Layer id is empty")
		}
		filename = path.Join(c.config.basedir, l.id, l.Filename())
	} else {
		reader = res.Body
		filename = path.Join(c.config.basedir, l.Filename())

	}
	defer reader.Close()
	if c.config.verbose {
		log.Printf("Writing to %s", filename)
	}
	f, err := os.Create(filename)
	defer f.Close()
	n, err := io.Copy(f, reader)
	if err != nil {
		return fmt.Errorf("Failed to copy %s to file: %w", layerShort, err)
	}
	if c.config.verbose {
		log.Printf("Downloaded %d bytes for %s", n, layerShort)
	}
	return nil
}

func (l Layer) Filename() string {
	if strings.HasSuffix(l.Type, "gzip") {
		return "layer.tar"
	} else {
		return l.Digest[7:] + ".json"
	}
}

// Write manifest to file
func (m Manifest) WriteTo(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Failed to create %s: %w", filename, err)
	}
	return json.NewEncoder(f).Encode(m)
}
