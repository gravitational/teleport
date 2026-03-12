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

	"github.com/gravitational/teleport/lib/vnet/daemon"
)

const (
	tunInterfaceName = "utun"
)

// RunDarwinAdminProcess must run as root. It creates and sets up a TUN device
// and passes the file descriptor for that device over the unix socket found at
// config.socketPath.
//
// It also handles host OS configuration that must run as root, and stays alive
// to keep the host configuration up to date. It will stay running until the
// socket at config.socketPath is deleted, ctx is canceled, or until
// encountering an unrecoverable error.
func RunDarwinAdminProcess(ctx context.Context, config daemon.Config) error {
	log.InfoContext(ctx, "Running VNet admin process")
	if err := config.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "checking daemon process config")
	}

	serviceCreds, err := readCredentials(config.ServiceCredentialPath)
	if err != nil {
		return trace.Wrap(err, "reading service IPC credentials")
	}
	clt, err := newClientApplicationServiceClient(ctx, serviceCreds, config.ClientApplicationServiceAddr)
	if err != nil {
		return trace.Wrap(err, "creating user process client")
	}
	defer clt.close()

	return runUnixAdminProcess(ctx, clt, tunInterfaceName)
}
