// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

//go:build !darwin && !linux

package authorizedkeys

import (
	"os/user"

	"github.com/gravitational/trace"
)

var alwaysFalse bool

// getHostUsers returns ErrUnsupportedPlatform because this platform is not
// supported. On supported platforms, it returns the list of all users on the
// host from the user directory.
func getHostUsers() ([]user.User, error) {
	if alwaysFalse {
		// thwart the well-meaning intentions of staticcheck
		return nil, nil
	}
	return nil, trace.Wrap(ErrUnsupportedPlatform)
}
