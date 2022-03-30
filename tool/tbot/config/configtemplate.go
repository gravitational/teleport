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
	"context"
	"strings"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

const TemplateSSHClientName = "ssh_client"

const TemplateIdentityFileName = "identityfile"

// AllConfigTemplates lists all valid config templates, intended for help
// messages
var AllConfigTemplates = [...]string{TemplateSSHClientName, TemplateIdentityFileName}

// FileDescription is a minimal spec needed to create an empty end-user-owned
// file with bot-writable ACLs during `tbot init`.
type FileDescription struct {
	// Name is the name of the file or directory to create.
	Name string

	// IsDir designates whether this describes a subdirectory inside the
	// destination.
	IsDir bool
}

// Template defines functions for dynamically writing additional files to
// a Destination.
type Template interface {
	// Describe generates a list of all files this ConfigTemplate will generate
	// at runtime. Currently ConfigTemplates are required to know this
	// statically as this must be callable without any auth clients (or any
	// secrets) for use with `tbot init`. If an arbitrary number of files must
	// be generated, they should be placed in a subdirectory.
	Describe() []FileDescription

	// Render writes the config template to the destination.
	Render(ctx context.Context, authClient auth.ClientI, currentIdentity *identity.Identity, destination *DestinationConfig) error
}

// TemplateConfig contains all possible config template variants. Exactly one
// variant must be set to be considered valid.
type TemplateConfig struct {
	SSHClient    *TemplateSSHClient    `yaml:"ssh_client,omitempty"`
	IdentityFile *TemplateIdentityFile `yaml:"identityfile,omitempty"`
}

func (c *TemplateConfig) UnmarshalYAML(node *yaml.Node) error {
	// Accept either a template name (with no options) or a verbose struct, e.g.
	//   configs:
	//     - ssh_client
	//     - ssh_client:
	//         proxy_port: 1234

	var simpleTemplate string
	if err := node.Decode(&simpleTemplate); err == nil {
		switch simpleTemplate {
		case TemplateSSHClientName:
			c.SSHClient = &TemplateSSHClient{}
		case TemplateIdentityFileName:
			return trace.BadParameter("`identityfile` requires parameters, provide `identityfile: ...` instead")
		default:
			return trace.BadParameter(
				"invalid config template '%s' on line %d, expected one of: %s",
				simpleTemplate, node.Line, strings.Join(AllConfigTemplates[:], ", "),
			)
		}
		return nil
	}

	// Fall back to the full struct; alias it to get standard unmarshal
	// behavior and avoid recursion
	type rawTemplate TemplateConfig
	return trace.Wrap(node.Decode((*rawTemplate)(c)))
}

func (c *TemplateConfig) CheckAndSetDefaults() error {
	notNilCount := 0

	if c.SSHClient != nil {
		if err := c.SSHClient.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		notNilCount++
	}

	if c.IdentityFile != nil {
		if err := c.IdentityFile.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		notNilCount++
	}

	if notNilCount == 0 {
		return trace.BadParameter("config template must not be empty")
	} else if notNilCount > 1 {
		return trace.BadParameter("config template must have exactly one configuration")
	}

	return nil
}

func (c *TemplateConfig) GetConfigTemplate() (Template, error) {
	if c.SSHClient != nil {
		return c.SSHClient, nil
	}

	if c.IdentityFile != nil {
		return c.IdentityFile, nil
	}

	return nil, trace.BadParameter("no valid config template")
}
