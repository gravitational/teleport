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
	// ClientApplicationServiceAddr is the address of the client application
	// service the admin process connects to.
	ClientApplicationServiceAddr string
	// ServiceCredentialPath is the path to IPC credentials used to authenticate
	// with the client application service.
	ServiceCredentialPath string
}

// RunLinuxAdminProcess must run as root.
func RunLinuxAdminProcess(ctx context.Context, config LinuxAdminProcessConfig) error {
	log.InfoContext(ctx, "Running VNet admin process")

	serviceCreds, err := readCredentials(config.ServiceCredentialPath)
	if err != nil {
		return trace.Wrap(err, "reading service IPC credentials")
	}
	// TODO(tangyatsu): change to gRPC client over unix socket instead of TCP.
	clt, err := newClientApplicationServiceClient(ctx, serviceCreds, config.ClientApplicationServiceAddr)
	if err != nil {
		return trace.Wrap(err, "creating user process client")
	}
	defer clt.close()

	return runUnixAdminProcess(ctx, clt, tunInterfaceName)
}
