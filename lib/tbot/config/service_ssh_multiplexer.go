/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

// SSHMultiplexerServiceType is the type of the `ssh-proxy` service.
const SSHMultiplexerServiceType = "ssh-multiplexer"

// SSHMultiplexerService is the configuration for the `ssh-proxy` service
type SSHMultiplexerService struct {
	// Destination is where the config and tunnel should be written to. It
	// should be a DestinationDirectory.
	Destination bot.Destination `yaml:"destination"`
	// EnableResumption controls whether to enable session resumption for the
	// SSH proxy.
	// Call `SessionResumptionEnabled` to get the value with defaults applied.
	EnableResumption *bool `yaml:"enable_resumption"`
	// ProxyTemplatesPath is the path to the directory containing the templates
	// for the SSH proxy.
	// This field is optional, if not provided, no templates will be used.
	// This file is loaded once on start, so changes to the templates will
	// require a restart of tbot.
	ProxyTemplatesPath string `yaml:"proxy_templates_path"`
	// ProxyCommand is the base command to configure OpenSSH to invoke to
	// connect to the SSH multiplexer. The path to the socket and the target
	// will be automatically appended.
	// Optional: If not provided, it will default to the `tbot` binary.
	ProxyCommand []string `yaml:"proxy_command,omitempty"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime CredentialLifetime `yaml:",inline"`
}

func (s *SSHMultiplexerService) SessionResumptionEnabled() bool {
	if s.EnableResumption == nil {
		return true
	}
	return *s.EnableResumption
}

func (s *SSHMultiplexerService) Type() string {
	return SSHMultiplexerServiceType
}

func (s *SSHMultiplexerService) MarshalYAML() (interface{}, error) {
	type raw SSHMultiplexerService
	return withTypeHeader((*raw)(s), SSHMultiplexerServiceType)
}

func (s *SSHMultiplexerService) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw SSHMultiplexerService
	if err := node.Decode((*raw)(s)); err != nil {
		return trace.Wrap(err)
	}
	s.Destination = dest
	return nil
}

func (s *SSHMultiplexerService) CheckAndSetDefaults() error {
	if s.Destination == nil {
		return trace.BadParameter("destination: must be specified")
	}
	_, ok := s.Destination.(*DestinationDirectory)
	if !ok {
		return trace.BadParameter("destination: must be of type `directory`")
	}
	if err := validateOutputDestination(s.Destination); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (o *SSHMultiplexerService) GetCredentialLifetime() CredentialLifetime {
	return o.CredentialLifetime
}
