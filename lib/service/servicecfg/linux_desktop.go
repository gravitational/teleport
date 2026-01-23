// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package servicecfg

import (
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/utils"
)

// LinuxDesktopConfig specifies the configuration for the Linux Desktop
// Access service.
type LinuxDesktopConfig struct {
	Enabled bool
	// ListenAddr is the address to listed on for incoming desktop connections.
	ListenAddr utils.NetAddr
	// PublicAddrs is a list of advertised public addresses of the service.
	PublicAddrs []utils.NetAddr

	// ConnLimiter limits the connection and request rates.
	ConnLimiter limiter.Config

	Labels map[string]string
}
