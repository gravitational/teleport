//go:build !linux || !cgo

// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package user

import (
	osu "os/user"

	"github.com/gravitational/trace"
)

// Lookup wraps [os/user.Lookup].
func Lookup(username string) (*osu.User, error) { return osu.Lookup(username) }

// LookupId wraps [os/user.LookupId].
func LookupId(id string) (*osu.User, error) { return osu.LookupId(id) }

// LookupGroup wraps [os/user.LookupGroup].
func LookupGroup(name string) (*osu.Group, error) { return osu.LookupGroup(name) }

// LookupGroupId wraps [os/user.LookupGroupId].
func LookupGroupId(id string) (*osu.Group, error) { return osu.LookupGroupId(id) }

// Current wraps [os/user.Current].
func Current() (*osu.User, error) { return osu.Current() }

// GroupIds wraps [os/user.User.GroupIds].
func GroupIds(u *osu.User) ([]string, error) {
	if u == nil {
		return nil, trace.BadParameter("user cannot be nil")
	}
	return u.GroupIds()
}
