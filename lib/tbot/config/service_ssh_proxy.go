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

const SSHProxyServiceType = "ssh-proxy"

// SSHProxyService does TODO
type SSHProxyService struct {
	// Destination is where the config and tunnel should be written to. It
	// should be a DestinationDirectory.
	Destination bot.Destination `yaml:"destination"`
}

func (s *SSHProxyService) Type() string {
	return SSHProxyServiceType
}

func (s *SSHProxyService) MarshalYAML() (interface{}, error) {
	type raw SSHProxyService
	return withTypeHeader((*raw)(s), SSHProxyServiceType)
}

func (s *SSHProxyService) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw SSHProxyService
	if err := node.Decode((*raw)(s)); err != nil {
		return trace.Wrap(err)
	}
	s.Destination = dest
	return nil
}

func (s *SSHProxyService) CheckAndSetDefaults() error {
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
