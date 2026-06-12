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

//go:build !windows
// +build !windows

package vnet

import (
	"context"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

func platformAuthenticateProcess(ctx context.Context, req *vnetv1.AuthenticateProcessRequest) error {
	// This is only called in tests, the real method is only used on Windows.
	return nil
}
