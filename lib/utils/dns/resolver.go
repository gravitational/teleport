// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package dns

import (
	"context"
	"log/slog"
	"net"
)

// NewResolver creates and returns a DNS resolver
//
// DNSServerAddress, if set, will be used to resolve DNS requests. If
// it is left blank, the default system resolver will be used.
func NewResolver(ctx context.Context, dnsServerAddress string, logger *slog.Logger) *net.Resolver {
	if logger == nil {
		logger = slog.Default()
	}

	dialer := net.Dialer{}
	dial := func(dialCtx context.Context, network, address string) (net.Conn, error) {
		return dialer.DialContext(dialCtx, network, address)
	}

	if dnsServerAddress != "" {
		logger.DebugContext(ctx, "Using custom DNS resolver address", "address", dnsServerAddress)

		host, port, err := net.SplitHostPort(dnsServerAddress)
		if err != nil {
			host = dnsServerAddress
			port = "53"
		}

		customResolverAddr := net.JoinHostPort(host, port)
		dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, customResolverAddr)
		}
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial:     dial,
	}

	return resolver
}
