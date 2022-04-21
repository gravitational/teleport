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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// SessionTrackerService is a realtime session service that has information about
// sessions that are in-flight in the cluster at the moment.
type SessionTrackerService interface {
	// GetActiveSessionTrackers returns a list of active session trackers.
	GetActiveSessionTrackers(ctx context.Context) ([]types.SessionTracker, error)

	// GetSessionTracker returns the current state of a session tracker for an active session.
	GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error)

	// CreateSessionTracker creates a tracker resource for an active session.
	CreateSessionTracker(ctx context.Context, req *proto.CreateSessionTrackerRequest) (types.SessionTracker, error)

	// UpdateSessionTracker updates a tracker resource for an active session.
	UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error

	// RemoveSessionTracker removes a tracker resource for an active session.
	RemoveSessionTracker(ctx context.Context, sessionID string) error

	// UpdatePresence updates the presence status of a user in a session.
	UpdatePresence(ctx context.Context, sessionID, user string) error
}
