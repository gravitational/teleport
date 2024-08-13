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

	"github.com/gravitational/trace"
)

func configureOS(ctx context.Context, cfg *osConfig) error {
	return trace.Wrap(ErrVnetNotImplemented)
}

func (c *osConfigurator) doWithDroppedRootPrivileges(ctx context.Context, fn func() error) (err error) {
	return trace.Wrap(ErrVnetNotImplemented)
}
