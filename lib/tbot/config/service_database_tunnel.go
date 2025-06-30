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
	"net"
	"net/url"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

const DatabaseTunnelServiceType = "database-tunnel"

// DatabaseTunnelService opens an authenticated tunnel for Database Access.
type DatabaseTunnelService struct {
	// Listen is the address on which database tunnel should listen. Example:
	// - "tcp://127.0.0.1:3306"
	// - "tcp://0.0.0.0:3306
	Listen string `yaml:"listen"`
	// Roles is the list of roles to request for the tunnel.
	// If empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`
	// Service is the service name of the Teleport database. Generally this is
	// the name of the Teleport resource. This field is required for all types
	// of database.
	Service string `yaml:"service"`
	// Database is the name of the database to proxy to.
	Database string `yaml:"database"`
	// Username is the database username to proxy as.
	Username string `yaml:"username"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime CredentialLifetime `yaml:",inline"`

	// Listener overrides "listen" and directly provides an opened listener to
	// use.
	Listener net.Listener `yaml:"-"`
}

func (s *DatabaseTunnelService) Type() string {
	return DatabaseTunnelServiceType
}

func (s *DatabaseTunnelService) MarshalYAML() (any, error) {
	type raw DatabaseTunnelService
	return withTypeHeader((*raw)(s), DatabaseTunnelServiceType)
}

func (s *DatabaseTunnelService) UnmarshalYAML(node *yaml.Node) error {
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw DatabaseTunnelService
	if err := node.Decode((*raw)(s)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *DatabaseTunnelService) CheckAndSetDefaults() error {
	switch {
	case s.Listen == "" && s.Listener == nil:
		return trace.BadParameter("listen: should not be empty")
	case s.Service == "":
		return trace.BadParameter("service: should not be empty")
	case s.Database == "":
		return trace.BadParameter("database: should not be empty")
	case s.Username == "":
		return trace.BadParameter("username: should not be empty")
	}
	if _, err := url.Parse(s.Listen); err != nil {
		return trace.Wrap(err, "parsing listen")
	}
	return nil
}

func (s *DatabaseTunnelService) GetCredentialLifetime() CredentialLifetime {
	return s.CredentialLifetime
}
