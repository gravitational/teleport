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

package services

import (
	"context"

	"github.com/gravitational/teleport/lib/backend"
)

// StableUNIXUsersInternal is the (auth-only) storage service to interact with
// stable UNIX users.
type StableUNIXUsersInternal interface {
	// ListStableUNIXUsers returns a page of username/UID pairs. The returned
	// next page token is empty when fetching the last page in the list;
	// otherwise it can be passed to ListStableUNIXUsers to fetch the next page.
	ListStableUNIXUsers(ctx context.Context, pageSize int, pageToken string) (_ []StableUNIXUser, nextPageToken string, _ error)

	// GetUIDForUsername returns the stable UID associated with the given
	// username, if one exists, or a NotFound.
	GetUIDForUsername(context.Context, string) (int32, error)
	// SearchFreeUID returns the first available UID in the range between first
	// and last (included). If no stored UIDs are within the range, it returns
	// first. If no UIDs are available in the range, the returned bool will be
	// false.
	SearchFreeUID(ctx context.Context, first, last int32) (int32, bool, error)

	// AppendCreateStableUNIXUser appends some atomic write actions to the given
	// slice that will create a username/UID pair for a stable UNIX user. The
	// backend to which the actions are applied should be the same backend used
	// by the StableUNIXUsersInternal.
	AppendCreateStableUNIXUser(actions []backend.ConditionalAction, username string, uid int32) ([]backend.ConditionalAction, error)
}

// StableUNIXUser is a username/UID pair representing a stable UNIX user.
type StableUNIXUser struct {
	Username string
	UID      int32
}
