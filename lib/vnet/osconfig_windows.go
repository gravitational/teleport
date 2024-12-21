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

//go:build windows
// +build windows

package vnet

import (
	"context"

	"github.com/gravitational/trace"
)

var (
	// ErrVnetNotImplemented is an error indicating that VNet is not implemented on the host OS.
	ErrVnetNotImplemented = &trace.NotImplementedError{Message: "VNet is not implemented on windows"}
)

func configureOS(ctx context.Context, cfg *osConfig) error {
	// TODO(nklaassen): implement configureOS on Windows.
	return trace.Wrap(ErrVnetNotImplemented)
}

func (c *osConfigurator) doWithDroppedRootPrivileges(ctx context.Context, fn func() error) (err error) {
	// TODO(nklaassen): implement doWithDroppedPrivileges on Windows.
	return trace.Wrap(ErrVnetNotImplemented)
}
