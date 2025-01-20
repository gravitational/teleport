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

//go:build !vnetdaemon || !darwin
// +build !vnetdaemon !darwin

package common

import (
	"github.com/alecthomas/kingpin/v2"
)

// The vnet-daemon command is only supported with the vnetdaemon tag on darwin.
func newPlatformVnetDaemonCommand(app *kingpin.Application) vnetCommandNotSupported {
	return vnetCommandNotSupported{}
}
