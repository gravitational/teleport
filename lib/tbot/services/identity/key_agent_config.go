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

package identity

import (
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal"
)

// KeyAgentServiceType is the service type for the key agent service.
const KeyAgentServiceType = "identity/key-agent"

// KeyAgentConfig contains configuration for the key agent service.
type KeyAgentConfig struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`

	// DelegationSessionID optionally identifies the delegation session the
	// generated credentials will be associated with, enabling the bot to act
	// on a (human) user's behalf.
	//
	// It is mutually exclusive with Roles.
	DelegationSessionID string `yaml:"delegation_session_id,omitempty"`

	// Roles is the list of roles to request for the generated credentials. If
	// empty, it defaults to all the bot's roles.
	//
	// It is mutually exclusive with DelegationSessionID.
	Roles []string `yaml:"roles,omitempty"`

	// Cluster allows certificates to be generated for a leaf cluster of the
	// cluster that the bot is connected to. These certificates can be used
	// to directly connect to a Teleport proxy of that leaf cluster, or used
	// with the root cluster's proxy which will forward the request to the
	// leaf cluster.
	Cluster string `yaml:"cluster,omitempty"`

	// AllowReissue controls whether the generated credentials can be used to
	// reissue further credentials (e.g to produce a certificate for application
	// access). It is recommended to leave this disabled to prevent the scope of
	// issued credentials from being increased, however, it can be useful in
	// scenarios where credentials are desired to be reissued in a dynamic way.
	//
	// Defaults to false.
	AllowReissue bool `yaml:"allow_reissue,omitempty"`

	// Destination is where the key agent socket, certificate, and identity file
	// should be written. Configure tsh and tctl to use the agent by setting the
	// TELEPORT_KEY_AGENT_DIR environment variable to the given directory path.
	Destination destination.Destination `yaml:"destination"`
}

// CheckAndSetDefaults satisfies the config.ServiceConfig interface.
func (c *KeyAgentConfig) CheckAndSetDefaults(scoped bool) error {
	// TODO(boxofrad): Add support for scopes (and support for the WithPrivateKey
	// identity generator option to GenerateScoped). Scope support was omitted
	// from the initial version because we do not need it in Beams yet.
	if scoped {
		return trace.BadParameter("service type %q is not supported in scoped mode", KeyAgentServiceType)
	}

	if c.Destination == nil {
		return trace.BadParameter("destination: is required")
	}
	if _, isDir := c.Destination.(*destination.Directory); !isDir {
		return trace.BadParameter("destination: must be a filesystem directory")
	}

	if c.DelegationSessionID != "" && len(c.Roles) > 0 {
		return trace.BadParameter("delegation_session_id: is mutually-exclusive with roles")
	}

	return nil
}

// GetCredentialLifetime satisfies the config.ServiceConfig interface.
func (c *KeyAgentConfig) GetCredentialLifetime() bot.CredentialLifetime {
	return c.CredentialLifetime
}

// GetName satisfies the config.ServiceConfig interface.
func (c *KeyAgentConfig) GetName() string {
	return c.Name
}

// SetName satisfies the config.ServiceConfig interface.
func (c *KeyAgentConfig) SetName(name string) {
	c.Name = name
}

// Type satisfies the config.ServiceConfig interface.
func (*KeyAgentConfig) Type() string { return KeyAgentServiceType }

// UnmarshalYAML prevents incorrect YAML unmarshaling.
func (c *KeyAgentConfig) UnmarshalYAML(*yaml.Node) error {
	return trace.NotImplemented("unmarshaling %T with UnmarshalYAML is not supported, use UnmarshalConfig instead", c)
}

// UnmarshalConfig unmarshals the service configuration from YAML.
func (c *KeyAgentConfig) UnmarshalConfig(ctx bot.UnmarshalConfigContext, node *yaml.Node) error {
	dest, err := internal.ExtractOutputDestination(ctx, node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid getting our "not
	// implemented" error.
	type raw KeyAgentConfig
	if err := node.Decode((*raw)(c)); err != nil {
		return trace.Wrap(err)
	}
	c.Destination = dest
	return nil
}

// KeyAgentOpt provides additional dependencies to the key agent service.
type KeyAgentOpt func(*KeyAgentService)

// WithDefaultCredentialLifetime sets the default credential lifetime.
func WithDefaultCredentialLifetime(lifetime bot.CredentialLifetime) KeyAgentOpt {
	return func(svc *KeyAgentService) {
		svc.defaultCredentialLifetime = lifetime
	}
}
