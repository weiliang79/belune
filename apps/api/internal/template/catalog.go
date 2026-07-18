package template

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// catalogFS holds the embedded manifests (repo convention: JSON) and the JSON
// Schema. Manifests live under catalog/manifests/*.json; the catalog works
// offline because everything is baked into the binary.
//
//go:embed catalog
var catalogFS embed.FS

// isManifestFile reports whether a catalog entry is a manifest. Both formats are
// accepted (repo convention is JSON, but YAML is supported); a single yaml.v3
// parser reads either.
func isManifestFile(name string) bool {
	return strings.HasSuffix(name, ".json") ||
		strings.HasSuffix(name, ".yaml") ||
		strings.HasSuffix(name, ".yml")
}

// Parse decodes a manifest from JSON or YAML. JSON is a strict subset of YAML,
// so a single yaml.v3 parser reads both formats; the repo convention is JSON.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// Catalog is the validated, immutable set of templates loaded at startup.
type Catalog struct {
	byID  map[string]*Manifest
	order []string // ids sorted by name, for stable listing
}

var (
	defaultOnce sync.Once
	defaultCat  *Catalog
	defaultErr  error
)

// Default returns the embedded catalog, loaded and validated once. A malformed
// or invalid embedded manifest is a build-time bug caught by the catalog test,
// so callers may treat a non-nil error as fatal at startup.
func Default() (*Catalog, error) {
	defaultOnce.Do(func() {
		defaultCat, defaultErr = load(catalogFS)
	})
	return defaultCat, defaultErr
}

// load reads and validates every manifest under catalog/manifests.
func load(fsys fs.FS) (*Catalog, error) {
	entries, err := fs.ReadDir(fsys, "catalog/manifests")
	if err != nil {
		return nil, fmt.Errorf("read manifests dir: %w", err)
	}

	cat := &Catalog{byID: make(map[string]*Manifest)}
	for _, e := range entries {
		if e.IsDir() || !isManifestFile(e.Name()) {
			continue
		}
		p := path.Join("catalog/manifests", e.Name())
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		m, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if err := Validate(m); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if _, dup := cat.byID[m.ID]; dup {
			return nil, fmt.Errorf("%s: duplicate template id %q", e.Name(), m.ID)
		}
		cat.byID[m.ID] = m
	}

	cat.order = make([]string, 0, len(cat.byID))
	for id := range cat.byID {
		cat.order = append(cat.order, id)
	}
	sort.Slice(cat.order, func(i, j int) bool {
		return strings.ToLower(cat.byID[cat.order[i]].Name) < strings.ToLower(cat.byID[cat.order[j]].Name)
	})
	return cat, nil
}

// List returns all templates sorted by display name.
func (c *Catalog) List() []*Manifest {
	out := make([]*Manifest, 0, len(c.order))
	for _, id := range c.order {
		out = append(out, c.byID[id])
	}
	return out
}

// Get returns the template with the given id, or false if absent.
func (c *Catalog) Get(id string) (*Manifest, bool) {
	m, ok := c.byID[id]
	return m, ok
}

// Schema returns the embedded JSON Schema (editor autocomplete / contributor
// reference; the Go validator is the authoritative enforcement).
func Schema() ([]byte, error) {
	return catalogFS.ReadFile("catalog/schema.json")
}
