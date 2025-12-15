/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package ssh

import (
	"context"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const HostOutputServiceType = "ssh_host"

const (
	// SSHHostCertPath is the default filename prefix for the SSH host
	// certificate
	SSHHostCertPath = "ssh_host"

	// SSHHostCertSuffix is the suffix appended to the generated host certificate.
	SSHHostCertSuffix = "-cert.pub"

	// SSHHostUserCASuffix is the suffix appended to the user CA file.
	SSHHostUserCASuffix = "-user-ca.pub"
)

// HostOutputConfig generates a host certificate signed by the Teleport CA. This
// can be used to allow OpenSSH server to be trusted by Teleport SSH clients.
type HostOutputConfig struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`
	// Destination is where the credentials should be written to.
	Destination destination.Destination `yaml:"destination"`
	// Roles is the list of roles to request for the generated credentials.
	// If empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`

	// Principals is a list of principals to request for the host cert.
	Principals []string `yaml:"principals"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`
}

// GetName returns the user-given name of the service, used for validation purposes.
func (o *HostOutputConfig) GetName() string {
	return o.Name
}

// SetName sets the service's name to an automatically generated one.
func (o *HostOutputConfig) SetName(name string) {
	o.Name = name
}

func (o *HostOutputConfig) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

func (o *HostOutputConfig) GetDestination() destination.Destination {
	return o.Destination
}

func (o *HostOutputConfig) CheckAndSetDefaults() error {
	if o.Destination == nil {
		return trace.BadParameter("no destination configured for output")
	}
	if err := o.Destination.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating destination")
	}
	if len(o.Principals) == 0 {
		return trace.BadParameter("at least one principal must be specified")
	}

	return nil
}

func (o *HostOutputConfig) Describe() []bot.FileDescription {
	return []bot.FileDescription{
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

func (o *HostOutputConfig) MarshalYAML() (any, error) {
	type raw HostOutputConfig
	return encoding.WithTypeHeader((*raw)(o), HostOutputServiceType)
}

func (o *HostOutputConfig) UnmarshalYAML(*yaml.Node) error {
	return trace.NotImplemented("unmarshaling %T with UnmarshalYAML is not supported, use UnmarshalConfig instead", o)
}

func (o *HostOutputConfig) UnmarshalConfig(ctx bot.UnmarshalConfigContext, node *yaml.Node) error {
	dest, err := internal.ExtractOutputDestination(ctx, node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid getting our "not implemented" error
	type raw HostOutputConfig
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *HostOutputConfig) Type() string {
	return HostOutputServiceType
}

func (o *HostOutputConfig) GetCredentialLifetime() bot.CredentialLifetime {
	return o.CredentialLifetime
}
