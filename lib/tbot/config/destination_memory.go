/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	// Accept either a bool or a raw (in this case empty) struct
	//   memory: {}
	// or:
	//   memory: true

	var boolVal bool
	if err := node.Decode(&boolVal); err == nil {
		if !boolVal {
			return trace.BadParameter("memory must not be false (leave unset to disable)")
		}
		return nil
	}

	type rawMemory DestinationMemory
	return trace.Wrap(node.Decode((*rawMemory)(dm)))
}

func (dm *DestinationMemory) CheckAndSetDefaults() error {
	dm.store = make(map[string][]byte)

	return nil
}

func (dm *DestinationMemory) Init(subdirs []string) error {
	// Nothing to do.
	return nil
}

func (dm *DestinationMemory) Verify(keys []string) error {
	// Nothing to do.
	return nil
}

func (dm *DestinationMemory) Write(name string, data []byte) error {
	dm.store[name] = data

	return nil
}

func (dm *DestinationMemory) Read(name string) ([]byte, error) {
	b, ok := dm.store[name]
	if !ok {
		return nil, trace.BadParameter("not found: %s", name)
	}

	return b, nil
}

func (dm *DestinationMemory) String() string {
	return "[memory]"
}
