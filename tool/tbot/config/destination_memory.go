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
	"github.com/gravitational/teleport/tool/tbot/utils"
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

func (d *DestinationMemory) Write(name string, data []byte, _ utils.ModeHint) error {
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

func (d *DestinationMemory) String() string {
	return "[memory]"
}
