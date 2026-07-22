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
	"time"

	"github.com/gravitational/trace"
)

// Config contains fields necessary to start a daemon process for VNet running as root.
// Changes to this string must be reflected in protocol.h and service.h.
type Config struct {
	// ServiceCredentialPath is the path where credentials for IPC with the
	// client application are found.
	ServiceCredentialPath string
	// ClientApplicationServiceAddr is the local TCP address of the client
	// application gRPC service.
	ClientApplicationServiceAddr string
}

func (c *Config) CheckAndSetDefaults() error {
	switch {
	case c.ClientApplicationServiceAddr == "":
		return trace.BadParameter("missing client application service address")
	case c.ServiceCredentialPath == "":
		return trace.BadParameter("missing service credential path")
	}
	return nil
}

// CheckUnprivilegedProcessInterval denotes how often the admin process should check if the
// unprivileged process has quit.
const CheckUnprivilegedProcessInterval = time.Second
