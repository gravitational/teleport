/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// SessionTrackerService is a realtime session service that has information about
// sessions that are in-flight in the cluster at the moment.
type SessionTrackerService interface {
	// GetActiveSessionTrackers returns a list of active session trackers.
	GetActiveSessionTrackers(ctx context.Context) ([]types.SessionTracker, error)

	// GetSessionTracker returns the current state of a session tracker for an active session.
	GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error)

	// UpdateSessionTracker updates a tracker resource for an active session.
	UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error

	// RemoveSessionTracker removes a tracker resource for an active session.
	RemoveSessionTracker(ctx context.Context, sessionID string) error

	// UpdatePresence updates the presence status of a user in a session.
	UpdatePresence(ctx context.Context, sessionID, user string) error

	// UpsertSessionTracker creates a tracker resource for an active session.
	UpsertSessionTracker(ctx context.Context, tracker types.SessionTracker) error
}

// UpdateSessionTrackerState is a helper function to add a session tracker participant.
func AddSessionTrackerParticipant(ctx context.Context, sts SessionTrackerService, sid string, participant *types.Participant) error {
	req := &proto.UpdateSessionTrackerRequest{
		SessionID: sid,
		Update: &proto.UpdateSessionTrackerRequest_AddParticipant{
			AddParticipant: &proto.SessionTrackerAddParticipant{
				Participant: participant,
			},
		},
	}

	err := sts.UpdateSessionTracker(ctx, req)
	return trace.Wrap(err)
}

// UpdateSessionTrackerState is a helper function to remove a session tracker participant.
func RemoveSessionTrackerParticipant(ctx context.Context, sts SessionTrackerService, sid string, participantID string) error {
	req := &proto.UpdateSessionTrackerRequest{
		SessionID: sid,
		Update: &proto.UpdateSessionTrackerRequest_RemoveParticipant{
			RemoveParticipant: &proto.SessionTrackerRemoveParticipant{
				ParticipantID: participantID,
			},
		},
	}

	err := sts.UpdateSessionTracker(ctx, req)
	return trace.Wrap(err)
}

// UpdateSessionTrackerState is a helper function to update a session tracker state.
func UpdateSessionTrackerState(ctx context.Context, sts SessionTrackerService, sid string, state types.SessionState) error {
	req := &proto.UpdateSessionTrackerRequest{
		SessionID: sid,
		Update: &proto.UpdateSessionTrackerRequest_UpdateState{
			UpdateState: &proto.SessionTrackerUpdateState{
				State: state,
			},
		},
	}

	err := sts.UpdateSessionTracker(ctx, req)
	return trace.Wrap(err)
}

// UpdateSessionTrackerExpiry is a helper function to update a session tracker expiry.
func UpdateSessionTrackerExpiry(ctx context.Context, sts SessionTrackerService, sid string, expires time.Time) error {
	req := &proto.UpdateSessionTrackerRequest{
		SessionID: sid,
		Update: &proto.UpdateSessionTrackerRequest_UpdateExpiry{
			UpdateExpiry: &proto.SessionTrackerUpdateExpiry{
				Expires: &expires,
			},
		},
	}

	err := sts.UpdateSessionTracker(ctx, req)
	return trace.Wrap(err)
}

// UnmarshalSessionTracker unmarshals the Session resource from JSON.
func UnmarshalSessionTracker(bytes []byte) (types.SessionTracker, error) {
	var session types.SessionTrackerV1

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	if err := utils.FastUnmarshal(bytes, &session); err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	if err := session.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &session, nil
}

// MarshalSessionTracker marshals the Session resource to JSON.
func MarshalSessionTracker(session types.SessionTracker) ([]byte, error) {
	if err := session.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	switch session := session.(type) {
	case *types.SessionTrackerV1:
		return utils.FastMarshal(session)
	default:
		return nil, trace.BadParameter("unrecognized session version %T", session)
	}
}
