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

//go:build vnetdaemon
// +build vnetdaemon

package vnet

import (
	"context"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/vnet/daemon"
)

func execAdminProcess(ctx context.Context, socketPath, ipv6Prefix, dnsAddr string) error {
	// TODO(ravicious): Remove the feature env var after the daemon gets implemented.
	if os.Getenv("VNETDAEMON") == "yes" {
		return trace.Wrap(daemon.RegisterAndCall(ctx, socketPath, ipv6Prefix, dnsAddr))
	}

	return trace.Wrap(execAdminSubcommand(ctx, socketPath, ipv6Prefix, dnsAddr))
}
