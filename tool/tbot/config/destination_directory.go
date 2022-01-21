package config

import (
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

// DestinationDirectory is a Destination that writes to the local filesystem
type DestinationDirectory struct {
	Path string `yaml:"path,omitempty"`
}

func (dd *DestinationDirectory) UnmarshalYAML(node *yaml.Node) error {
	var path string
	if err := node.Decode(&path); err == nil {
		dd.Path = path
		return nil
	}

	// Shenanigans to prevent UnmarshalYAML from recursing back to this
	// override (we want to use standard unmarshal behavior for the full
	// struct)
	type rawDirectory DestinationDirectory
	if err := node.Decode((*rawDirectory)(dd)); err != nil {
		return err
	}

	return nil
}

func (dd *DestinationDirectory) CheckAndSetDefaults() error {
	if dd.Path == "" {
		return trace.BadParameter("destination path must not be empty")
	}

	return nil
}

func (d *DestinationDirectory) Write(name string, data []byte) error {
	if err := os.WriteFile(filepath.Join(d.Path, name), data, 0600); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (d *DestinationDirectory) Read(name string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(d.Path, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}
