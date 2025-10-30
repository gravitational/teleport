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
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

// MultiplexerServiceType is the type of the `ssh-proxy` service.
const MultiplexerServiceType = "ssh-multiplexer"

// MultiplexerConfig is the configuration for the `ssh-proxy` service
type MultiplexerConfig struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`
	// Destination is where the config and tunnel should be written to. It
	// should be a Directory.
	Destination destination.Destination `yaml:"destination"`
	// EnableResumption controls whether to enable session resumption for the
	// SSH proxy.
	// Call `SessionResumptionEnabled` to get the value with defaults applied.
	EnableResumption *bool `yaml:"enable_resumption,omitempty"`
	// ProxyTemplatesPath is the path to the directory containing the templates
	// for the SSH proxy.
	// This field is optional, if not provided, no templates will be used.
	// This file is loaded once on start, so changes to the templates will
	// require a restart of tbot.
	ProxyTemplatesPath string `yaml:"proxy_templates_path,omitempty"`
	// ProxyCommand is the base command to configure OpenSSH to invoke to
	// connect to the SSH multiplexer. The path to the socket and the target
	// will be automatically appended.
	// Optional: If not provided, it will default to the `tbot` binary.
	ProxyCommand []string `yaml:"proxy_command,omitempty"`
	// RelayAddress specifies the address of a relay transport server to use for
	// all the SSH connections going through this mux.
	RelayAddress string `yaml:"relay_server,omitempty"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`
}

// GetName returns the user-given name of the service, used for validation purposes.
func (o *MultiplexerConfig) GetName() string {
	return o.Name
}

// SetName sets the service's name to an automatically generated one.
func (o *MultiplexerConfig) SetName(name string) {
	o.Name = name
}

func (s *MultiplexerConfig) SessionResumptionEnabled() bool {
	if s.EnableResumption == nil {
		return true
	}
	return *s.EnableResumption
}

func (s *MultiplexerConfig) Type() string {
	return MultiplexerServiceType
}

func (s *MultiplexerConfig) MarshalYAML() (any, error) {
	type raw MultiplexerConfig
	return encoding.WithTypeHeader((*raw)(s), MultiplexerServiceType)
}

func (o *MultiplexerConfig) UnmarshalYAML(*yaml.Node) error {
	return trace.NotImplemented("unmarshaling %T with UnmarshalYAML is not supported, use UnmarshalConfig instead", o)
}

func (o *MultiplexerConfig) UnmarshalConfig(ctx bot.UnmarshalConfigContext, node *yaml.Node) error {
	dest, err := internal.ExtractOutputDestination(ctx, node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid getting our "not implemented" error
	type raw MultiplexerConfig
	if err := node.Decode((*raw)(o)); err != nil {
		return trace.Wrap(err)
	}
	o.Destination = dest
	return nil
}

func (s *MultiplexerConfig) CheckAndSetDefaults() error {
	if s.Destination == nil {
		return trace.BadParameter("destination: must be specified")
	}
	_, ok := s.Destination.(*destination.Directory)
	if !ok {
		return trace.BadParameter("destination: must be of type `directory`")
	}
	if err := s.Destination.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating destination")
	}
	return nil
}

func (o *MultiplexerConfig) GetCredentialLifetime() bot.CredentialLifetime {
	return o.CredentialLifetime
}
