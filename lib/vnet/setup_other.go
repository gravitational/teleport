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

//go:build !darwin
// +build !darwin

package vnet

import (
	"context"
	"net"
	"os"
	"runtime"

	"github.com/gravitational/trace"
	"golang.zx2c4.com/wireguard/tun"

	"github.com/gravitational/teleport/lib/vnet/daemon"
)

var (
	// ErrVnetNotImplemented is an error indicating that VNet is not implemented on the host OS.
	ErrVnetNotImplemented = &trace.NotImplementedError{Message: "VNet is not implemented on " + runtime.GOOS}
)

func createUnixSocket() (*net.UnixListener, string, error) {
	return nil, "", trace.Wrap(ErrVnetNotImplemented)
}

func sendTUNNameAndFd(socketPath, tunName string, tunFile *os.File) error {
	return trace.Wrap(ErrVnetNotImplemented)
}

func receiveTUNDevice(socket *net.UnixListener) (tun.Device, error) {
	return nil, trace.Wrap(ErrVnetNotImplemented)
}

func execAdminProcess(ctx context.Context, config daemon.Config) error {
	return trace.Wrap(ErrVnetNotImplemented)
}

func DaemonSubcommand(ctx context.Context) error {
	return trace.Wrap(ErrVnetNotImplemented)
}
