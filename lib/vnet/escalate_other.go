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

//go:build !darwin && !windows
// +build !darwin,!windows

package vnet

import (
	"context"
	"runtime"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/vnet/daemon"
)

var (
	// ErrVnetNotImplemented is an error indicating that VNet is not implemented on the host OS.
	ErrVnetNotImplemented = &trace.NotImplementedError{Message: "VNet is not implemented on " + runtime.GOOS}
)

// execAdminProcess is called from the normal user process to execute the admin
// subcommand as root.
func execAdminProcess(ctx context.Context, config daemon.Config) error {
	return trace.Wrap(ErrVnetNotImplemented)
}
