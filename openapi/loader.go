package openapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

const maxSpecBytes = 20 * 1024 * 1024

type loadedSpec struct {
	Config SpecConfig
	Source string
	Doc    *openapi3.T
}

func loadSpecs(ctx context.Context, root string, cfg Config) ([]loadedSpec, []error) {
	var out []loadedSpec
	var errs []error
	for i, spec := range cfg.Specs {
		loaded, err := loadSpec(ctx, root, spec)
		if err != nil {
			errs = append(errs, fmt.Errorf("specs[%d]: %w", i, err))
			continue
		}
		out = append(out, loaded)
	}
	return out, errs
}

func loadSpec(ctx context.Context, root string, cfg SpecConfig) (loadedSpec, error) {
	loader := openapi3.NewLoader()
	loader.Context = ctx
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = readFromURI(ctx, root)
	var (
		data     []byte
		location *url.URL
		source   string
		err      error
	)
	if cfg.URL != "" {
		location, err = url.Parse(cfg.URL)
		if err != nil {
			return loadedSpec{}, fmt.Errorf("parse url: %w", err)
		}
		data, err = readRemote(ctx, location)
		source = cfg.URL
	} else {
		data, location, source, err = readFile(root, cfg.File)
	}
	if err != nil {
		return loadedSpec{}, err
	}
	doc, err := loader.LoadFromDataWithPath(data, location)
	if err != nil {
		return loadedSpec{}, fmt.Errorf("parse openapi: %w", err)
	}
	if strings.TrimSpace(doc.OpenAPI) == "" {
		return loadedSpec{}, fmt.Errorf("openapi version is empty")
	}
	if doc.Paths == nil {
		return loadedSpec{}, fmt.Errorf("openapi paths are empty")
	}
	return loadedSpec{Config: cfg, Source: source, Doc: doc}, nil
}

func readFromURI(ctx context.Context, root string) openapi3.ReadFromURIFunc {
	return func(_ *openapi3.Loader, location *url.URL) ([]byte, error) {
		switch strings.ToLower(location.Scheme) {
		case "http", "https":
			return readRemote(ctx, location)
		case "", "file":
			data, _, _, err := readFile(root, location.Path)
			return data, err
		default:
			return nil, fmt.Errorf("unsupported openapi ref scheme %q", location.Scheme)
		}
	}
}

func readRemote(ctx context.Context, location *url.URL) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/yaml, application/json, text/yaml, text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", location.String(), err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch %s: http %d", location.String(), resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSpecBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSpecBytes {
		return nil, fmt.Errorf("fetch %s: response exceeds %d bytes", location.String(), maxSpecBytes)
	}
	return data, nil
}

func readFile(root, raw string) ([]byte, *url.URL, string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "file://") {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, nil, "", err
		}
		raw = u.Path
	}
	if raw == "" {
		return nil, nil, "", fmt.Errorf("file is empty")
	}
	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, "", err
	}
	rel, err := filepath.Rel(cleanRoot, abs)
	if err != nil {
		return nil, nil, "", err
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return nil, nil, "", fmt.Errorf("read %s: outside root", raw)
	}
	file, err := os.Open(abs)
	if err != nil {
		return nil, nil, "", fmt.Errorf("read %s: %w", raw, err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxSpecBytes+1))
	if err != nil {
		return nil, nil, "", fmt.Errorf("read %s: %w", raw, err)
	}
	if len(data) > maxSpecBytes {
		return nil, nil, "", fmt.Errorf("read %s: file exceeds %d bytes", raw, maxSpecBytes)
	}
	return data, &url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}, filepath.ToSlash(rel), nil
}
