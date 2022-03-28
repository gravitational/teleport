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
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/gravitational/trace"
)

// DestinationConfig configures a user certificate destination.
type DestinationConfig struct {
	DestinationMixin `yaml:",inline"`

	Roles   []string                `yaml:"roles,omitempty"`
	Kinds   []identity.ArtifactKind `yaml:"kinds,omitempty"`
	Configs []TemplateConfig        `yaml:"configs,omitempty"`
}

// destinationDefaults applies defaults for an output sink's destination. Since
// these have no sane defaults, in practice it just returns an error if no
// config is provided.
func destinationDefaults(dm *DestinationMixin) error {
	return trace.BadParameter("destinations require some valid output sink")
}

func (dc *DestinationConfig) CheckAndSetDefaults() error {
	if err := dc.DestinationMixin.CheckAndSetDefaults(destinationDefaults); err != nil {
		return trace.Wrap(err)
	}

	// Note: empty roles is allowed; interpreted to mean "all" at generation
	// time

	if len(dc.Kinds) == 0 && len(dc.Configs) == 0 {
		dc.Kinds = []identity.ArtifactKind{identity.KindSSH}
		dc.Configs = []TemplateConfig{{
			SSHClient: &TemplateSSHClient{},
		}}
	}

	for _, cfg := range dc.Configs {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// ContainsKind determines if this destination contains the given ConfigKind.
func (dc *DestinationConfig) ContainsKind(kind identity.ArtifactKind) bool {
	for _, k := range dc.Kinds {
		if k == kind {
			return true
		}
	}

	return false
}
