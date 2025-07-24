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
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

var (
	_ ServiceConfig = &ApplicationOutput{}
	_ Initable      = &ApplicationOutput{}
)

const ApplicationOutputType = "application"

type ApplicationOutput struct {
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

func (o *ApplicationOutput) Init(ctx context.Context) error {
	return trace.Wrap(o.Destination.Init(ctx, []string{}))
}

func (o *ApplicationOutput) CheckAndSetDefaults() error {
	if err := validateOutputDestination(o.Destination); err != nil {
		return trace.Wrap(err)
	}
	if o.AppName == "" {
		return trace.BadParameter("app_name must not be empty")
	}

	return nil
}

// GetName returns the user-given name of the service, used for validation purposes.
func (o *ApplicationOutput) GetName() string {
	return o.Name
}

func (o *ApplicationOutput) GetDestination() destination.Destination {
	return o.Destination
}

func (o *ApplicationOutput) Describe() []bot.FileDescription {
	out := []bot.FileDescription{
		{
			Name: IdentityFilePath,
		},
		{
			Name: HostCAPath,
		},
		{
			Name: UserCAPath,
		},
		{
			Name: DatabaseCAPath,
		},
	}
	if o.SpecificTLSExtensions {
		out = append(out, []bot.FileDescription{
			{
				Name: DefaultTLSPrefix + ".crt",
			},
			{
				Name: DefaultTLSPrefix + ".key",
			},
			{
				Name: DefaultTLSPrefix + ".cas",
			},
		}...)
	}
	return out
}

func (o *ApplicationOutput) MarshalYAML() (any, error) {
	type raw ApplicationOutput
	return encoding.WithTypeHeader((*raw)(o), ApplicationOutputType)
}

func (o *ApplicationOutput) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw ApplicationOutput
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (o *ApplicationOutput) Type() string {
	return ApplicationOutputType
}

func (o *ApplicationOutput) GetCredentialLifetime() bot.CredentialLifetime {
	return o.CredentialLifetime
}
