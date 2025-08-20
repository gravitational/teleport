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

package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// SessionTrackerService is a realtime session service that has information about
// sessions that are in-flight in the cluster at the moment.
type SessionTrackerService interface {
	// GetActiveSessionTrackers returns a list of active session trackers.
	GetActiveSessionTrackers(ctx context.Context) ([]types.SessionTracker, error)

	// GetActiveSessionTrackersWithFilter returns a list of active sessions filtered by a filter.
	GetActiveSessionTrackersWithFilter(ctx context.Context, filter *types.SessionTrackerFilter) ([]types.SessionTracker, error)

	// GetSessionTracker returns the current state of a session tracker for an active session.
	GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error)

	// CreateSessionTracker creates a tracker resource for an active session.
	CreateSessionTracker(ctx context.Context, st types.SessionTracker) (types.SessionTracker, error)

	// UpdateSessionTracker updates a tracker resource for an active session.
	UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error

	// RemoveSessionTracker removes a tracker resource for an active session.
	RemoveSessionTracker(ctx context.Context, sessionID string) error

	// UpdatePresence updates the presence status of a user in a session.
	UpdatePresence(ctx context.Context, sessionID, user string) error
}

// UnmarshalSessionTracker unmarshals the Session resource from JSON.
func UnmarshalSessionTracker(bytes []byte) (types.SessionTracker, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var session types.SessionTrackerV1
	if err := utils.FastUnmarshal(bytes, &session); err != nil {
		return nil, trace.BadParameter("%s", err)
	}

	if err := session.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &session, nil
}

// MarshalSessionTracker marshals the Session resource to JSON.
func MarshalSessionTracker(session types.SessionTracker) ([]byte, error) {
	switch session := session.(type) {
	case *types.SessionTrackerV1:
		if err := session.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(session)
	default:
		return nil, trace.BadParameter("unrecognized session version %T", session)
	}
}
