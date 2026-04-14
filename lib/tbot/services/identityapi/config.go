/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package identityapi

import (
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const ServiceType = "identity-api"

// Config produces a Teleport identity file whose private key is backed by a
// colocated local signing API.
type Config struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`
	// Destination is where the identity file and identity-api socket files will
	// be written.
	Destination destination.Destination `yaml:"destination"`
	// Roles is the list of roles to request for the generated credentials. If
	// empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`
	// Cluster allows certificates to be generated for a leaf cluster.
	Cluster string `yaml:"cluster,omitempty"`
	// AllowReissue controls whether the generated credentials can be used to
	// reissue further credentials.
	AllowReissue bool `yaml:"allow_reissue,omitempty"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`
}

func (c *Config) GetName() string {
	return c.Name
}

func (c *Config) SetName(name string) {
	c.Name = name
}

func (c *Config) Type() string {
	return ServiceType
}

func (c *Config) GetCredentialLifetime() bot.CredentialLifetime {
	return c.CredentialLifetime
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Destination == nil {
		return trace.BadParameter("no destination configured for output")
	}
	if err := c.Destination.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating destination")
	}
	if _, ok := c.Destination.(*destination.Directory); !ok {
		return trace.BadParameter("identity-api destination must be a directory")
	}
	return nil
}

func (c *Config) MarshalYAML() (any, error) {
	type raw Config
	return encoding.WithTypeHeader((*raw)(c), ServiceType)
}

func (c *Config) UnmarshalYAML(*yaml.Node) error {
	return trace.NotImplemented("unmarshaling %T with UnmarshalYAML is not supported, use UnmarshalConfig instead", c)
}

func (c *Config) UnmarshalConfig(ctx bot.UnmarshalConfigContext, node *yaml.Node) error {
	dest, err := internal.ExtractOutputDestination(ctx, node)
	if err != nil {
		return trace.Wrap(err)
	}
	type raw Config
	if err := node.Decode((*raw)(c)); err != nil {
		return trace.Wrap(err)
	}
	c.Destination = dest
	return nil
}
