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
	"log/slog"
	"net/netip"
	"time"
)

// NewOSUpstreamNameserverSource returns an upstream nameserver source for the
// current platform. Its results will be cached for 10 seconds.
func NewOSUpstreamNameserverSource(logger *slog.Logger) (UpstreamNameserverSource, error) {
	return CachingUpstreamNameserverSource(
		platformUpstreamNameserverSource(logger),
		10*time.Second,
	)
}

// AddrWithDNSPort returns addr with DNS port 53.
func AddrWithDNSPort(addr netip.Addr) string {
	return netip.AddrPortFrom(addr, 53).String()
}
