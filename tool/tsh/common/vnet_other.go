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

package common

import (
	"context"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/vnet"
)

// Satisfy unused linter.
var _ = newVnetClientApplication

func newPlatformVnetAdminSetupCommand(app *kingpin.Application) vnetCLICommand {
	return vnetCommandNotSupported{}
}

func newPlatformVnetServiceCommand(app *kingpin.Application) vnetCLICommand {
	return vnetCommandNotSupported{}
}

//nolint:staticcheck // SA4023. runVnetDiagnostics on unsupported platforms always returns err.
func runVnetDiagnostics(ctx context.Context, nsi vnet.NetworkStackInfo) error {
	return trace.NotImplemented("diagnostics are not implemented yet on this platform")
}
