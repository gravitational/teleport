//go:build !verified_accesslists

/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Package verified provides a formally verified implementation of access list
// membership checking. This is the stub version for builds without the
// verified_accesslists build tag.
package verified

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
)

// UserMeetsRequirements is a stub that returns an error when the verified
// implementation is not available. Build with -tags verified_accesslists
// to use the Rust FFI implementation.
func UserMeetsRequirements(_ types.User, _ accesslist.Requires) (bool, error) {
	return false, trace.NotImplemented("verified access list checking requires the verified_accesslists build tag")
}
