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

// DatabaseConfig is the config for a database access request.
type DatabaseConfig struct {
	// Service is the service name of the
	Service string `yaml:"service,omitempty"`

	// Database is the name of the database to request access to.
	Database string `yaml:"database,omitempty"`

	// Username is the database username to request access as.
	Username string `yaml:"username,omitempty"`
}

func (dc *DatabaseConfig) CheckAndSetDefaults() error {
	if dc.Service == "" {
		return trace.BadParameter("database `service` field must specify a database service name")
	}

	return nil
}

// DestinationConfig configures a user certificate destination.
type DestinationConfig struct {
	DestinationMixin `yaml:",inline"`

	Roles   []string                `yaml:"roles,omitempty"`
	Kinds   []identity.ArtifactKind `yaml:"kinds,omitempty"`
	Configs []TemplateConfig        `yaml:"configs,omitempty"`

	Database *DatabaseConfig `yaml:"database,omitempty"`
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

	if dc.Database != nil {
		if err := dc.Database.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
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
