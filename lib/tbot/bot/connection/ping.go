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

package connection

import (
	"context"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
)

// ProxyPinger can be used to ping the proxy or auth server to discover connection
// information (e.g. whether TLS routing is enabled).
type ProxyPinger interface {
	// Ping the proxy.
	Ping(ctx context.Context) (*ProxyPong, error)
}

// ProxyPong is the response of a proxy ping request.
type ProxyPong struct {
	*webclient.PingResponse

	// StaticProxyAddress is the user-configured proxy address when the user
	// requests their given address is preferred over pinging the proxy or auth
	// server.
	StaticProxyAddress string
}

// ProxyWebAddr returns the address to use to connect to the proxy web port.
// In TLS routing mode, this address should be used for most/all connections.
// This function takes into account the TBOT_USE_PROXY_ADDR environment
// variable, which can be used to force the use of the proxy address explicitly
// provided by the user rather than use the one fetched from the proxy ping.
func (p *ProxyPong) ProxyWebAddr() (string, error) {
	if p.StaticProxyAddress != "" {
		return p.StaticProxyAddress, nil
	}
	return p.Proxy.SSH.PublicAddr, nil
}

// ProxySSHAddr returns the address to use to connect to the proxy SSH service.
// Includes potential override via TBOT_USE_PROXY_ADDR.
func (p *ProxyPong) ProxySSHAddr() (string, error) {
	if p.Proxy.TLSRoutingEnabled && p.StaticProxyAddress != "" {
		return p.StaticProxyAddress, nil
	}
	// SSHProxyHostPort returns the host and port to use to connect to the
	// proxy's SSH service. If TLS routing is enabled, this will return the
	// proxy's web address, if not, the proxy SSH listener.
	host, port, err := p.Proxy.SSHProxyHostPort()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return net.JoinHostPort(host, port), nil
}
