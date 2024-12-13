package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func mockServer(t *testing.T) (svr *httptest.Server) {
	svr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("URL: %s", r.URL.Path)
		switch r.URL.Path {
		case "/v2":
			authHeader := fmt.Sprintf("Bearer realm=\"%s/token\",service=\"registry.docker.io\"", svr.URL)
			w.Header().Add(HeaderAuthenticate, authHeader)
			w.Header().Add("content-type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			f, err := os.Open("testdata/unauthorized.json")
			defer f.Close()
			if err != nil {
				io.WriteString(w, err.Error())
			}
			io.Copy(w, f)
		case "/token":
			w.Header().Add("content-type", "application/json")
			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(map[string]string{
				"token": "token-0123456789",
			})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, err.Error())
			}
		case "/v2/swaggerapi/swagger-editor/manifests/v4-latest":
			w.Header().Add("content-type", ContentTypeManifest)
			w.WriteHeader(http.StatusOK)
			f, err := os.Open("testdata/manifest-single.json")
			defer f.Close()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, err.Error())
			}
			io.Copy(w, f)
		case "/v2/swaggerapi/swagger-editor/manifests/next-v5":
			w.Header().Add("content-type", ContentTypeManifestList)
			w.WriteHeader(http.StatusOK)
			f, err := os.Open("testdata/manifest-multiple.json")
			defer f.Close()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, err.Error())
			}
			io.Copy(w, f)
		case "/v2/swaggerapi/swagger-editor/manifests/sha256:5a90044ce5a42518dda0e528c0c846d68ad3e25ef512dd64ea48daf7c9a52403":
			w.Header().Add("content-type", ContentTypeManifestList)
			w.WriteHeader(http.StatusOK)
			f, err := os.Open("testdata/manifest-linux.json")
			defer f.Close()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, err.Error())
			}
			io.Copy(w, f)
		default:
			io.WriteString(w, "{\"message\":\"Hello World\"}")
		}
	}))
	return svr
}
func TestDockerClient(t *testing.T) {
	svr := mockServer(t)
	defer svr.Close()
	t.Logf("Server URL: %s", svr.URL)
	c := NewClient(svr.URL, Config{
		arch:  "amd64",
		image: "swaggerapi/swagger-editor",
	})
	table := []struct {
		name string
		test func(*testing.T)
	}{
		{"PreAuth", func(t *testing.T) {
			err := c.PreAuth()
			if err != nil {
				t.Errorf("Expected err to be nil got %v", err)
			}
		}},
		{"Authenticate", func(t *testing.T) {
			err := c.Authenticate()
			if err != nil {
				t.Errorf("Expected err to be nil got %s:%s ->  %v", c.config.image, c.config.tag, err)
			}
		}},
		{"Manifest", func(t *testing.T) {
			c.config.tag = "v4-latest"
			m, err := c.Manifest()
			if err != nil {
				t.Errorf("Failed to load manifest: %s", err)
			}
			if m.Type == "" {
				t.Errorf("Manifest mediaType cannot be empty")
			}
		}},
		{"No matching manifest", func(t *testing.T) {
			c.config.tag = "darwin"
			c.config.tag = "next-v5"
			_, err := c.Manifest()
			if err == nil || !strings.HasPrefix(err.Error(), "No matching manifest") {
				t.Errorf("Expected: No matching manifest")
			}
		}},
		{"Manifest List", func(t *testing.T) {
			c.config.os = "linux"
			c.config.tag = "next-v5"
			m, err := c.Manifest()
			if err != nil {
				t.Errorf("Failed to load manifest: %s", err)
			}
			if m.Type == "" {
				t.Errorf("Manifest mediaType cannot be empty")
			}
		}},
	}
	for _, tt := range table {
		t.Run(tt.name, tt.test)
	}
}
