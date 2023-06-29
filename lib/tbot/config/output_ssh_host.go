/*
Copyright 2023 Gravitational, Inc.

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
	"fmt"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const SSHHostOutputType = "ssh_host"

// SSHHostOutput generates a host certificate signed by the Teleport CA. This
// can be used to allow OpenSSH server to be trusted by Teleport SSH clients.
type SSHHostOutput struct {
	// Destination is where the credentials should be written to.
	Destination bot.Destination `yaml:"destination"`
	// Roles is the list of roles to request for the generated credentials.
	// If empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`

	// Principals is a list of principals to request for the host cert.
	Principals []string `yaml:"principals"`
}

func (o *SSHHostOutput) templates() []template {
	return []template{
		&templateSSHHostCert{
			principals: o.Principals,
		},
	}
}

func (o *SSHHostOutput) Render(ctx context.Context, p provider, ident *identity.Identity) error {
	for _, t := range o.templates() {
		if err := t.render(ctx, p, ident, o.Destination); err != nil {
			return trace.Wrap(err, "rendering template %s", t.name())
		}
	}

	return nil
}

func (o *SSHHostOutput) Init() error {
	subDirs, err := listSubdirectories(o.templates())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(o.Destination.Init(subDirs))
}

func (o *SSHHostOutput) GetDestination() bot.Destination {
	return o.Destination
}

func (o *SSHHostOutput) GetRoles() []string {
	return o.Roles
}

func (o *SSHHostOutput) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	if len(o.Principals) == 0 {
		return trace.BadParameter("at least one principal must be specified")
	}

	return nil
}

func (o *SSHHostOutput) Describe() []FileDescription {
	var fds []FileDescription
	for _, t := range o.templates() {
		fds = append(fds, t.describe()...)
	}

	return fds
}

func (o SSHHostOutput) MarshalYAML() (interface{}, error) {
	type raw SSHHostOutput
	return withTypeHeader(raw(o), SSHHostOutputType)
}

func (o *SSHHostOutput) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw SSHHostOutput
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *SSHHostOutput) String() string {
	return fmt.Sprintf("%s (%s)", SSHHostOutputType, o.GetDestination())
}
