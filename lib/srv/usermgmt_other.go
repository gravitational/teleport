//go:build !linux
// +build !linux

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package srv

import (
	"github.com/gravitational/trace"
)

//nolint:staticcheck // intended to always return an error for non-linux builds
func newHostUsersBackend() (HostUsersBackend, error) {
	return nil, trace.NotImplemented("Host user creation management is only supported on linux")
}

//nolint:staticcheck // intended to always return an error for non-linux builds
func newHostSudoersBackend(_ string) (HostSudoersBackend, error) {
	return nil, trace.NotImplemented("Host user creation management is only supported on linux")
}
