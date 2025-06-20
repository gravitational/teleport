/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package config

import (
	"context"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

const SSHHostOutputType = "ssh_host"

const (
	// SSHHostCertPath is the default filename prefix for the SSH host
	// certificate
	SSHHostCertPath = "ssh_host"

	// SSHHostCertSuffix is the suffix appended to the generated host certificate.
	SSHHostCertSuffix = "-cert.pub"

	// SSHHostUserCASuffix is the suffix appended to the user CA file.
	SSHHostUserCASuffix = "-user-ca.pub"
)

var (
	_ ServiceConfig = &SSHHostOutput{}
	_ Initable      = &SSHHostOutput{}
)

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

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime CredentialLifetime `yaml:",inline"`
}

func (o *SSHHostOutput) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

func (o *SSHHostOutput) GetDestination() bot.Destination {
	return o.Destination
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
	return []FileDescription{
		{
			Name: SSHHostCertPath,
		},
		{
			Name: SSHHostCertPath + SSHHostCertSuffix,
		},
		{
			Name: SSHHostCertPath + SSHHostUserCASuffix,
		},
	}
}

func (o *SSHHostOutput) MarshalYAML() (any, error) {
	type raw SSHHostOutput
	return withTypeHeader((*raw)(o), SSHHostOutputType)
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

func (o *SSHHostOutput) Type() string {
	return SSHHostOutputType
}

func (o *SSHHostOutput) GetCredentialLifetime() CredentialLifetime {
	return o.CredentialLifetime
}
