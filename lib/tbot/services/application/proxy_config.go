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

// ProxyServiceType is the type string for the ProxyService, used within the
// service header to indicate the service type.
const ProxyServiceType = "application-proxy"

// ProxyServiceConfig is the configuration for the ProxyService.
type ProxyServiceConfig struct {
	// Name of the service for logs and the /readyz endpoint.
	// Optional.
	Name string `yaml:"name,omitempty"`
	// Listen is the address on which application proxy should listen. Example:
	// - "tcp://127.0.0.1:8080"
	// - "tcp://0.0.0.0:8080"
	Listen string `yaml:"listen"`
	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed. For the application
	// proxy, this is primarily an internal detail.
	CredentialLifetime bot.CredentialLifetime `yaml:"credential_lifetime,omitempty"`
	// DelegationSessionID optionally identifies the delegation session the
	// generated credentials will be associated with, enabling the bot to act
	// on a (human) user's behalf.
	DelegationSessionID string `yaml:"delegation_session_id,omitempty"`
	// Listener overrides "listen" and directly provides an opened listener to
	// use. Primarily used for testing.
	Listener net.Listener `yaml:"-"`
}

// GetName returns the user-given name of the service for reporting.
func (p *ProxyServiceConfig) GetName() string {
	return p.Name
}

// SetName sets the service's name to an automatically generated one.
func (p *ProxyServiceConfig) SetName(name string) {
	p.Name = name
}

// Type returns the type of the service.
func (p *ProxyServiceConfig) Type() string {
	return ProxyServiceType
}

// MarshalYAML overrides the YAML representation of the service.
func (p *ProxyServiceConfig) MarshalYAML() (any, error) {
	type raw ProxyServiceConfig
	return encoding.WithTypeHeader((*raw)(p), ProxyServiceType)
}

// UnmarshalYAML is used to override the YAML unmarshaling of the service.
func (p *ProxyServiceConfig) UnmarshalYAML(node *yaml.Node) error {
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw ProxyServiceConfig
	if err := node.Decode((*raw)(p)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CheckAndSetDefaults checks the user-provided configuration against validation
// rules and sets any default values.
func (p *ProxyServiceConfig) CheckAndSetDefaults(scoped bool) error {
	switch {
	case p.Listen == "" && p.Listener == nil:
		return trace.BadParameter("listen: should not be empty")
	case scoped && p.DelegationSessionID != "":
		return trace.BadParameter("delegation_session_id: not supported with scopes")
	}

	if _, err := url.Parse(p.Listen); err != nil {
		return trace.Wrap(err, "parsing listen")
	}

	return nil
}

// GetCredentialLifetime returns the embedded CredentialLifetime.
func (p *ProxyServiceConfig) GetCredentialLifetime() bot.CredentialLifetime {
	return p.CredentialLifetime
}
