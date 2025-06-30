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

package example

import (
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/internal/marshaling"
)

const ServiceType = "example"

// Config is a temporary example service for testing purposes. It is not
// intended to be used and exists to demonstrate how a user configurable
// service integrates with the tbot service manager.
type Config struct {
	Message string `yaml:"message"`
}

func (s *Config) Type() string {
	return ServiceType
}

func (s *Config) MarshalYAML() (any, error) {
	type raw Config
	return marshaling.WithTypeHeader((*raw)(s), ServiceType)
}

func (s *Config) UnmarshalYAML(node *yaml.Node) error {
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw Config
	if err := node.Decode((*raw)(s)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Config) CheckAndSetDefaults() error {
	if s.Message == "" {
		return trace.BadParameter("message: should not be empty")
	}
	return nil
}

func (s *Config) GetCredentialLifetime() bot.CredentialLifetime {
	return bot.CredentialLifetime{}
}
