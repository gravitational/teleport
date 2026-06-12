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

package application

import (
	"context"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const OutputServiceType = "application"

type OutputConfig struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`
	// Destination is where the credentials should be written to.
	Destination destination.Destination `yaml:"destination"`
	// Roles is the list of roles to request for the generated credentials.
	// If empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`

	AppName string `yaml:"app_name"`

	// SpecificTLSExtensions creates additional outputs named `tls.crt`,
	// `tls.key` and `tls.cas`. This is unneeded for most clients which can
	// be configured with specific paths to use, but exists for compatibility.
	SpecificTLSExtensions bool `yaml:"specific_tls_naming"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`
}

func (o *OutputConfig) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

func (o *OutputConfig) CheckAndSetDefaults() error {
	if o.Destination == nil {
		return trace.BadParameter("no destination configured for output")
	}
	if err := o.Destination.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating configured destination")
	}
	if o.AppName == "" {
		return trace.BadParameter("app_name must not be empty")
	}

	return nil
}

// GetName returns the user-given name of the service, used for validation purposes.
func (o *OutputConfig) GetName() string {
	return o.Name
}

// SetName sets the service's name to an automatically generated one.
func (o *OutputConfig) SetName(name string) {
	o.Name = name
}

func (o *OutputConfig) GetDestination() destination.Destination {
	return o.Destination
}

func (o *OutputConfig) Describe() []bot.FileDescription {
	out := []bot.FileDescription{
		{
			Name: internal.IdentityFilePath,
		},
		{
			Name: internal.HostCAPath,
		},
		{
			Name: internal.UserCAPath,
		},
		{
			Name: internal.DatabaseCAPath,
		},
	}
	if o.SpecificTLSExtensions {
		out = append(out, []bot.FileDescription{
			{
				Name: internal.DefaultTLSPrefix + ".crt",
			},
			{
				Name: internal.DefaultTLSPrefix + ".key",
			},
			{
				Name: internal.DefaultTLSPrefix + ".cas",
			},
		}...)
	}
	return out
}

func (o *OutputConfig) MarshalYAML() (any, error) {
	type raw OutputConfig
	return encoding.WithTypeHeader((*raw)(o), OutputServiceType)
}

func (o *OutputConfig) UnmarshalYAML(*yaml.Node) error {
	return trace.NotImplemented("unmarshaling %T with UnmarshalYAML is not supported, use UnmarshalConfig instead", o)
}

func (o *OutputConfig) UnmarshalConfig(ctx bot.UnmarshalConfigContext, node *yaml.Node) error {
	dest, err := internal.ExtractOutputDestination(ctx, node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid getting our "not implemented" error
	type raw OutputConfig
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *OutputConfig) Type() string {
	return OutputServiceType
}

func (o *OutputConfig) GetCredentialLifetime() bot.CredentialLifetime {
	return o.CredentialLifetime
}
