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
)

// ErrVnetNotImplemented is an error indicating that VNet is not implemented on the host OS.
var ErrVnetNotImplemented = &trace.NotImplementedError{Message: "VNet is not implemented on " + runtime.GOOS}

func runPlatformUserProcess(_ context.Context, _ *UserProcessConfig) (*ProcessManager, error) {
	return nil, trace.Wrap(ErrVnetNotImplemented)
}

func platformConfigureOS(_ context.Context, _ *osConfig) error {
	return trace.Wrap(ErrVnetNotImplemented)
}

// Satisfy unused linter.
var (
	_ = osConfig{
		tunName:    "",
		tunIPv4:    "",
		tunIPv6:    "",
		dnsAddr:    "",
		cidrRanges: nil,
		dnsZones:   nil,
	}
	_ = newOSConfigurator
	_ = (*osConfigurator).runOSConfigurationLoop
)
