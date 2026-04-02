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

package vnet

import (
	"context"

	"github.com/gravitational/trace"
)

const (
	tunInterfaceName = "TeleportVNet"
)

// LinuxAdminProcessConfig configures RunLinuxAdminProcess.
type LinuxAdminProcessConfig struct {
	// ClientApplicationServiceSocketPath is the unix socket path of the client
	// application service.
	ClientApplicationServiceSocketPath string
}

// RunLinuxAdminProcess must run as root.
func RunLinuxAdminProcess(ctx context.Context, config LinuxAdminProcessConfig) error {
	log.InfoContext(ctx, "Running VNet admin process")

	clt, err := newUnixClientApplicationServiceClient(ctx, config.ClientApplicationServiceSocketPath)
	if err != nil {
		return trace.Wrap(err, "creating user process client")
	}
	defer clt.close()

	return runUnixAdminProcess(ctx, clt, tunInterfaceName)
}
