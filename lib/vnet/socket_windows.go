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
	"os"

	"github.com/gravitational/trace"
	"golang.zx2c4.com/wireguard/tun"
)

func createSocket() (*noSocket, string, error) {
	// TODO(nklaassen): implement createSocket on windows.
	return nil, "", trace.Wrap(ErrVnetNotImplemented)
}

func sendTUNNameAndFd(socketPath, tunName string, tunFile *os.File) error {
	// TODO(nklaassen): implement sendTUNNameAndFd on windows.
	return trace.Wrap(ErrVnetNotImplemented)
}

func receiveTUNDevice(_ *noSocket) (tun.Device, error) {
	// TODO(nklaassen): receiveTUNDevice on windows.
	return nil, trace.Wrap(ErrVnetNotImplemented)
}

type noSocket struct{}

func (_ noSocket) Close() error {
	return trace.Wrap(ErrVnetNotImplemented)
}
