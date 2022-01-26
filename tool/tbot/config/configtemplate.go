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
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/gravitational/teleport/tool/tbot/utils"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

const CONFIG_TEMPLATE_SSH_CLIENT = "ssh_client"

// AllConfigTemplates lists all valid config templates, intended for help
// messages
var AllConfigTemplates = [...]string{CONFIG_TEMPLATE_SSH_CLIENT}

// FileDescription is a minimal spec needed to create an empty end-user-owned
// file with bot-writable ACLs during `tbot init`.
type FileDescription struct {
	// Name is the name of the file or directory to create.
	Name string

	// IsDir designates whether this describes a subdirectory inside the
	// destination.
	IsDir bool

	// ModeHint describes the intended permissions for this data, for
	// Destination backends where permissions are relevant.
	ModeHint utils.ModeHint
}

// ConfigTemplate defines functions for dynamically writing additional files to
// a Destination.
type ConfigTemplate interface {
	// Describe generates a list of all files this ConfigTemplate will generate
	// at runtime. Currently ConfigTemplates are required to know this
	// statically as this must be callable without any auth clients (or any
	// secrets) for use with `tbot init`. If an arbitrary number of files must
	// be generated, they should be placed in a subdirectory.
	Describe() []FileDescription

	// Render writes the config template to the destination.
	Render(authClient *auth.Client, currentIdentity *identity.Identity, destination *DestinationConfig) error
}

// ConfigTemplateConfig contains all possible config template variants. Exactly one
// variant must be set to be considered valid.
type ConfigTemplateConfig struct {
	SSHClient *ConfigTemplateSSHClient `yaml:"ssh_client,omitempty"`
}

func (c *ConfigTemplateConfig) UnmarshalYAML(node *yaml.Node) error {
	var simpleTemplate string
	if err := node.Decode(&simpleTemplate); err == nil {
		switch simpleTemplate {
		case CONFIG_TEMPLATE_SSH_CLIENT:
			c.SSHClient = &ConfigTemplateSSHClient{}
			fmt.Println("no params, using defaults")
		default:
			return trace.BadParameter(
				"invalid config template '%s' on line %d, expected one of: %s",
				simpleTemplate, node.Line, strings.Join(AllConfigTemplates[:], ", "),
			)
		}
		return nil
	}

	type rawTemplate ConfigTemplateConfig
	if err := node.Decode((*rawTemplate)(c)); err != nil {
		return err
	}

	return nil
}

func (c *ConfigTemplateConfig) CheckAndSetDefaults() error {
	notNilCount := 0

	if c.SSHClient != nil {
		c.SSHClient.CheckAndSetDefaults()
		notNilCount += 1
	}

	if notNilCount == 0 {
		return trace.BadParameter("config template must not be empty")
	} else if notNilCount > 1 {
		return trace.BadParameter("config template must have exactly one configuration")
	}

	return nil
}

func (c *ConfigTemplateConfig) GetConfigTemplate() (ConfigTemplate, error) {
	if c.SSHClient != nil {
		return c.SSHClient, nil
	}

	return nil, trace.BadParameter("no valid config template")
}
