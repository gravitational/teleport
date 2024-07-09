// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package daemon

import (
	"github.com/gravitational/trace"
)

// Config contains fields necessary to start a daemon process for VNet running as root.
// Changes to this string must be reflected in protocol.h and service.h.
type Config struct {
	// SocketPath is a path to a unix socket used for passing a TUN device from the admin process to
	// the unprivileged process.
	SocketPath string
	// IPv6Prefix is the IPv6 prefix for the VNet.
	IPv6Prefix string
	// DNSAddr is the IP address for the VNet DNS server.
	DNSAddr string
	// HomePath points to TELEPORT_HOME that will be used by the admin process.
	HomePath string
}

func (c *Config) CheckAndSetDefaults() error {
	if c.SocketPath == "" {
		return trace.BadParameter("missing socket path")
	}

	if c.IPv6Prefix == "" {
		return trace.BadParameter("missing IPv6 prefix")
	}

	if c.DNSAddr == "" {
		return trace.BadParameter("missing DNS address")
	}

	if c.HomePath == "" {
		return trace.BadParameter("missing home path")
	}

	return nil
}
