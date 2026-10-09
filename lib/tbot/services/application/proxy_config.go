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
	"time"

	"github.com/gravitational/trace"
	"go.yaml.in/yaml/v3"

	apidefaults "github.com/gravitational/teleport/api/defaults"
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

	// HTTP settings for incoming and upstream http requests
	HTTP ProxyServiceHTTPConfig `yaml:"http,omitempty"`

	// Listener overrides "listen" and directly provides an opened listener to
	// use. Primarily used for testing.
	Listener net.Listener `yaml:"-"`
}

type ProxyServiceHTTPConfig struct {
	// DialTimeout is the maximum duration for establishing a new upstream
	// connection, including CA retrieval, DNS resolution, connection setup, and
	// TLS handshake. Does not limit the subsequent HTTP exchange.
	//
	// Defaults to Teleport’s default I/O timeout.
	DialTimeout time.Duration `yaml:"dial_timeout,omitempty"`

	// IdleTimeout is the maximum wait for the next incoming HTTP request on a
	// keep-alive connection. Does not affect upstream requests.
	//
	// If zero, the value of ReadTimeout is used.
	// If zero and ReadTimeout is zero, there is no timeout.
	IdleTimeout time.Duration `yaml:"idle_timeout,omitempty"`

	// ReadHeaderTimeout is the maximum time to read the incoming HTTP request headers.
	//
	// If zero, no specific timeout is applied to reading the request headers and
	// the overall maximum time to read the entire incoming HTTP request is set by
	// ReadTimeout.
	// If zero and ReadTimeout is zero, there is no timeout.
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout,omitempty"`

	// ReadTimeout is the maximum time to read the entire incoming HTTP request,
	// including headers and body. Does not reset after each read.
	//
	// Defaults to Teleport’s default I/O timeout if omitted.
	// If zero, there will be no timeout.
	ReadTimeout *time.Duration `yaml:"read_timeout,omitempty"`

	// WriteTimeout is the maximum time to write the response, measured from
	// completion of incoming request header reading. Expiry alone does not cancel
	// upstream requests.
	//
	// Defaults to Teleport’s default I/O timeout if omitted.
	// If zero, there will be no timeout.
	WriteTimeout *time.Duration `yaml:"write_timeout,omitempty"`

	// UpstreamRequestTimeout is the maximum duration for the upstream HTTP exchange,
	// including connection establishment, redirects, and reading the response body.
	// Does not include application certificate acquisition.
	// Client-facing reads and writes remain subject to read_timeout and write_timeout.
	//
	// If zero, there will be no timeout.
	UpstreamRequestTimeout time.Duration `yaml:"upstream_request_timeout,omitempty"`

	// UpstreamResponseHeaderTimeout is the maximum wait for upstream HTTP response
	// headers after the request, including its body, has been fully written.
	// Does not limit reading the response body.
	//
	// If zero, there will be no timeout.
	UpstreamResponseHeaderTimeout time.Duration `yaml:"upstream_response_header_timeout,omitempty"`
}

// GetName returns the user-given name of the service for reporting.
func (c *ProxyServiceConfig) GetName() string {
	return c.Name
}

// SetName sets the service's name to an automatically generated one.
func (o *ProxyServiceConfig) SetName(name string) {
	o.Name = name
}

// Type returns the type of the service.
func (c *ProxyServiceConfig) Type() string {
	return ProxyServiceType
}

// MarshalYAML overrides the YAML representation of the service.
func (c *ProxyServiceConfig) MarshalYAML() (any, error) {
	type raw ProxyServiceConfig
	return encoding.WithTypeHeader((*raw)(c), ProxyServiceType)
}

// UnmarshalYAML is used to override the YAML unmarshaling of the service.
func (c *ProxyServiceConfig) UnmarshalYAML(node *yaml.Node) error {
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw ProxyServiceConfig
	if err := node.Decode((*raw)(c)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CheckAndSetDefaults checks the user-provided configuration against validation
// rules and sets any default values.
func (c *ProxyServiceConfig) CheckAndSetDefaults(scoped bool) error {
	if scoped {
		if c.DelegationSessionID != "" {
			return trace.BadParameter("delegation_session_id: not supported with scopes")
		}
	}

	if c.Listen == "" && c.Listener == nil {
		return trace.BadParameter("listen: should not be empty")
	}

	if _, err := url.Parse(c.Listen); err != nil {
		return trace.Wrap(err, "parsing listen")
	}

	if c.HTTP.DialTimeout < 0 {
		return trace.BadParameter("http.dial_timeout: cannot be negative")
	}
	if c.HTTP.DialTimeout == 0 {
		c.HTTP.DialTimeout = apidefaults.DefaultIOTimeout
	}

	if c.HTTP.IdleTimeout < 0 {
		return trace.BadParameter("http.idle_timeout: cannot be negative")
	}

	if c.HTTP.ReadHeaderTimeout < 0 {
		return trace.BadParameter("http.read_header_timeout: cannot be negative")
	}

	if c.HTTP.ReadTimeout == nil {
		c.HTTP.ReadTimeout = new(apidefaults.DefaultIOTimeout)
	}
	if *c.HTTP.ReadTimeout < 0 {
		return trace.BadParameter("http.read_timeout: cannot be negative")
	}

	if c.HTTP.WriteTimeout == nil {
		c.HTTP.WriteTimeout = new(apidefaults.DefaultIOTimeout)
	}
	if *c.HTTP.WriteTimeout < 0 {
		return trace.BadParameter("http.write_timeout: cannot be negative")
	}

	if c.HTTP.UpstreamRequestTimeout < 0 {
		return trace.BadParameter("http.upstream_request_timeout: cannot be negative")
	}

	if c.HTTP.UpstreamResponseHeaderTimeout < 0 {
		return trace.BadParameter("http.upstream_response_header_timeout: cannot be negative")
	}

	return nil
}

// GetCredentialLifetime returns the embedded CredentialLifetime.
func (c *ProxyServiceConfig) GetCredentialLifetime() bot.CredentialLifetime {
	return c.CredentialLifetime
}
