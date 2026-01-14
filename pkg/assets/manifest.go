package assets

import (
	"encoding/json"
	"os"
)

// Manifest maps original asset paths (eg "js/app.js") to fingerprinted
// filenames emitted during a production build (eg "js/app.a1b2c3d4.js").
type Manifest map[string]string

// LoadManifest reads a manifest.json file and returns the mapping.
func LoadManifest(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// AssetFuncFromManifest returns a template-friendly function that resolves an
// asset key into the manifest-mapped path prefixed with prefix. Example:
//   asset := AssetFuncFromManifest(manifest, "/assets/")
//   asset("js/app.js") -> "/assets/js/app.a1b2c3.js"
func AssetFuncFromManifest(man Manifest, prefix string) func(string) string {
	return func(key string) string {
		if man == nil {
			return prefix + key
		}
		if v, ok := man[key]; ok {
			return prefix + v
		}
		return prefix + key
	}
}
