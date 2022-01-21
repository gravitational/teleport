package config

import (
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

// DestinationMemory is a memory certificate destination
type DestinationMemory struct {
	store map[string][]byte `yaml:"-"`
}

func (dm *DestinationMemory) UnmarshalYAML(node *yaml.Node) error {
	var boolVal bool
	if err := node.Decode(&boolVal); err == nil {
		if !boolVal {
			return trace.BadParameter("memory must not be false (leave unset to disable)")
		}
		return nil
	}

	type rawMemory DestinationMemory
	if err := node.Decode((*rawMemory)(dm)); err != nil {
		return err
	}

	return nil
}

func (dm *DestinationMemory) CheckAndSetDefaults() error {
	dm.store = make(map[string][]byte)

	return nil
}

func (d *DestinationMemory) Write(name string, data []byte) error {
	d.store[name] = data

	return nil
}

func (d *DestinationMemory) Read(name string) ([]byte, error) {
	b, ok := d.store[name]
	if !ok {
		return nil, trace.BadParameter("not found: %s", name)
	}

	return b, nil
}
