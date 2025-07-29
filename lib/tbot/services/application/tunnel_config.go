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
	"net"
	"net/url"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const TunnelServiceType = "application-tunnel"

// TunnelConfig opens an authenticated tunnel for Application
// Access.
type TunnelConfig struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`
	// Listen is the address on which database tunnel should listen. Example:
	// - "tcp://127.0.0.1:3306"
	// - "tcp://0.0.0.0:3306
	Listen string `yaml:"listen"`
	// Roles is the list of roles to request for the tunnel.
	// If empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`
	// AppName should be the name of the application as registered in Teleport
	// that you wish to tunnel to.
	AppName string `yaml:"app_name"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`

	// Listener overrides "listen" and directly provides an opened listener to
	// use.
	Listener net.Listener `yaml:"-"`
}

// GetName returns the user-given name of the service, used for validation purposes.
func (o *TunnelConfig) GetName() string {
	return o.Name
}

func (s *TunnelConfig) Type() string {
	return TunnelServiceType
}

func (s *TunnelConfig) MarshalYAML() (any, error) {
	type raw TunnelConfig
	return encoding.WithTypeHeader((*raw)(s), TunnelServiceType)
}

func (s *TunnelConfig) UnmarshalYAML(node *yaml.Node) error {
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw TunnelConfig
	if err := node.Decode((*raw)(s)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *TunnelConfig) CheckAndSetDefaults() error {
	switch {
	case s.Listen == "" && s.Listener == nil:
		return trace.BadParameter("listen: should not be empty")
	case s.AppName == "":
		return trace.BadParameter("app_name: should not be empty")
	}
	if _, err := url.Parse(s.Listen); err != nil {
		return trace.Wrap(err, "parsing listen")
	}
	return nil
}

func (o *TunnelConfig) GetCredentialLifetime() bot.CredentialLifetime {
	return o.CredentialLifetime
}
