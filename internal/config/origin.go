package config

import (
	"path/filepath"
)

// Layer names one level of the ADR-0011 precedence order.
type Layer string

// The layers, listed by LayerOrder from the lowest to the highest precedence.
const (
	LayerGlobal    Layer = "global"
	LayerRoot      Layer = "root config"
	LayerRootLocal Layer = "root local"
	LayerApp       Layer = "app config"
	LayerAppLocal  Layer = "app local"
	LayerFlag      Layer = "flag"
)

// MaskedValue replaces a value that came from a local.toml layer.
const MaskedValue = "****"

// LayerOrder lists the layers lowest precedence first; the last layer holding a
// key wins.
var LayerOrder = []Layer{LayerGlobal, LayerRoot, LayerRootLocal, LayerApp, LayerAppLocal, LayerFlag}

// Origin is the effective value of one key and the layer it came from. Layer is
// empty when no layer sets the key. A value from a local layer is masked
// because local.toml is where secrets live (ADR-0011 decision 1).
type Origin struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Layer  Layer  `json:"layer"`
	Masked bool   `json:"masked"`
}

// isLocal reports whether a layer is a gitignored local.toml.
func (l Layer) isLocal() bool { return l == LayerRootLocal || l == LayerAppLocal }

// layerFiles maps each file-backed layer to its path. The root layers exist only
// when a workspace root other than the project directory is found, and the app
// layers only when a project directory is in play.
func (s Store) layerFiles() map[Layer]string {
	files := map[Layer]string{LayerGlobal: filepath.Join(s.GlobalDir, FileName)}
	if rootDir := s.findWorkspaceRoot(); rootDir != "" && rootDir != s.ProjectRoot {
		files[LayerRoot] = filepath.Join(rootDir, DirName, FileName)
		files[LayerRootLocal] = filepath.Join(rootDir, DirName, LocalName)
	}
	if s.ProjectRoot != "" {
		files[LayerApp] = filepath.Join(s.ProjectRoot, DirName, FileName)
		files[LayerAppLocal] = filepath.Join(s.ProjectRoot, DirName, LocalName)
	}
	return files
}

// Origins reports, for every key in Keys, its effective value and the highest
// layer that sets it. flags holds per-invocation overrides by key; it may be
// nil. A missing layer file contributes nothing.
func (s Store) Origins(flags map[string]string) ([]Origin, error) {
	files := s.layerFiles()
	layers := map[Layer]Config{}
	for layer, path := range files {
		c, err := s.loadFromPath(path)
		if err != nil {
			return nil, err
		}
		layers[layer] = c
	}
	origins := make([]Origin, 0, len(Keys))
	for _, key := range Keys {
		o := Origin{Key: key}
		for _, layer := range LayerOrder {
			v, err := layerValue(layer, key, layers, flags)
			if err != nil {
				return nil, err
			}
			if v == "" {
				continue
			}
			o.Value, o.Layer = v, layer
		}
		if o.Layer.isLocal() {
			o.Value, o.Masked = MaskedValue, true
		}
		origins = append(origins, o)
	}
	return origins, nil
}

// layerValue returns the value one layer holds for a key, or "".
func layerValue(layer Layer, key string, layers map[Layer]Config, flags map[string]string) (string, error) {
	if layer == LayerFlag {
		return flags[key], nil
	}
	c, ok := layers[layer]
	if !ok {
		return "", nil
	}
	return c.Get(key)
}
