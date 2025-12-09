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

package botapiconfig

import (
	"net"
	"net/url"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
	"github.com/gravitational/trace"
	"go.yaml.in/yaml/v3"
)

const ServiceType = "bot-api"

// Config opens an authenticated tunnel for Application
// Access.
type Config struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`
	// Listen is the address on which database tunnel should listen. Example:
	// - "tcp://127.0.0.1:3306"
	// - "tcp://0.0.0.0:3306
	Listen string `yaml:"listen"`

	// Listener overrides "listen" and directly provides an opened listener to
	// use.
	Listener net.Listener `yaml:"-"`
}

// GetName returns the user-given name of the service, used for validation purposes.
func (o *Config) GetName() string {
	return o.Name
}

// SetName sets the service's name to an automatically generated one.
func (o *Config) SetName(name string) {
	o.Name = name
}

func (s *Config) Type() string {
	return ServiceType
}

func (s *Config) MarshalYAML() (any, error) {
	type raw Config
	return encoding.WithTypeHeader((*raw)(s), ServiceType)
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
	switch {
	case s.Listen == "" && s.Listener == nil:
		return trace.BadParameter("listen: should not be empty")
	}
	if _, err := url.Parse(s.Listen); err != nil {
		return trace.Wrap(err, "parsing listen")
	}
	return nil
}

// GetCredentialLifetime returns the credential lifetime configuration.
func (o *Config) GetCredentialLifetime() bot.CredentialLifetime {
	return bot.CredentialLifetime{}
}
