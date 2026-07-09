// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package login

import (
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal"
)

// AgentServiceType identifies the login agent service.
const AgentServiceType = "login-agent"

// AgentConfig contains configuration for the login agent service.
type AgentConfig struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`

	// Destination is where the API socket and server certificate should be
	// written to.
	Destination destination.Destination `yaml:"destination"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`
}

// UnmarshalYAML prevents incorrect YAML unmarshaling.
func (c *AgentConfig) UnmarshalYAML(*yaml.Node) error {
	return trace.NotImplemented("unmarshaling %T with UnmarshalYAML is not supported, use UnmarshalConfig instead", c)
}

// UnmarshalConfig unmarshals the service configuration from YAML.
func (c *AgentConfig) UnmarshalConfig(ctx bot.UnmarshalConfigContext, node *yaml.Node) error {
	dest, err := internal.ExtractOutputDestination(ctx, node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid getting our "not
	// implemented" error.
	type raw AgentConfig
	if err := node.Decode((*raw)(c)); err != nil {
		return trace.Wrap(err)
	}
	c.Destination = dest
	return nil
}

// CheckAndSetDefaults satisfies the config.ServiceConfig interface.
func (c *AgentConfig) CheckAndSetDefaults(scoped bool) error {
	if scoped {
		return trace.BadParameter("service type %q is not supported in scoped mode", AgentServiceType)
	}

	if c.Destination == nil {
		return trace.BadParameter("destination: is required")
	}
	if err := c.Destination.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating destination")
	}
	if _, isDir := c.Destination.(*destination.Directory); !isDir {
		return trace.BadParameter("destination: must be a filesystem directory")
	}

	return nil
}

// GetCredentialLifetime satisfies the config.ServiceConfig interface.
func (c *AgentConfig) GetCredentialLifetime() bot.CredentialLifetime {
	return c.CredentialLifetime
}

// GetName satisfies the config.ServiceConfig interface.
func (c *AgentConfig) GetName() string {
	return c.Name
}

// SetName satisfies the config.ServiceConfig interface.
func (c *AgentConfig) SetName(name string) {
	c.Name = name
}

// Type satisfies the config.ServiceConfig interface.
func (*AgentConfig) Type() string { return AgentServiceType }

// AgentOpt can be passed to AgentServiceBuilder to customize the service.
type AgentOpt func(*AgentService)

// WithDefaultCredentialLifetime sets the default credential lifetime.
func WithDefaultCredentialLifetime(lifetime bot.CredentialLifetime) AgentOpt {
	return func(svc *AgentService) {
		svc.defaultCredentialLifetime = lifetime
	}
}
