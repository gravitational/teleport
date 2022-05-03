/*
Copyright 2022 Gravitational, Inc.

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

package srv

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/trace"
)

// SessionTracker is a session tracker for a specific session. It tracks
// the session in memory and broadcasts updates to the given service (backend).
type SessionTracker struct {
	// t is the in memory session tracker
	t types.SessionTracker

	// service is used to share session updates with the service
	service services.SessionTrackerService

	// stateUpdate is used to synchronize state logic in a session
	stateUpdate *sync.Cond

	closeC chan struct{}
}

// NewSessionTracker returns a new SessionTracker for the given types.SessionTracker
func NewSessionTracker(ctx context.Context, trackerSpec types.SessionTrackerSpecV1, service services.SessionTrackerService) (*SessionTracker, error) {
	if service == nil {
		return nil, trace.BadParameter("missing parameter service")
	}

	t, err := types.NewSessionTracker(trackerSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if t, err = service.CreateSessionTracker(ctx, t); err != nil {
		return nil, trace.Wrap(err)
	}

	return &SessionTracker{
		service:     service,
		t:           t,
		stateUpdate: sync.NewCond(&sync.Mutex{}),
		closeC:      make(chan struct{}),
	}, nil
}

// Close closes the session tracker and sets the tracker state to terminated
func (s *SessionTracker) Close(ctx context.Context) error {
	close(s.closeC)
	err := s.UpdateState(ctx, types.SessionState_SessionStateTerminated)
	return trace.Wrap(err)
}

const sessionTrackerExpirationUpdateInterval = apidefaults.SessionTrackerTTL / 6

// UpdateExpirationLoop extends the session tracker expiration by 1 hour every 10 minutes
// until the given ctx is closed.
func (s *SessionTracker) UpdateExpirationLoop(ctx context.Context, clock clockwork.Clock) error {
	for {
		select {
		// We use clock.After rather than clock.Ticker here because ticker
		// does not work with clock.BlockUntil, which is useful in tests.
		case time := <-clock.After(sessionTrackerExpirationUpdateInterval):
			expiry := time.Add(apidefaults.SessionTrackerTTL)
			if err := s.UpdateExpiration(ctx, expiry); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *SessionTracker) UpdateExpiration(ctx context.Context, expiry time.Time) error {
	s.t.SetExpiry(expiry)
	err := s.service.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: s.t.GetSessionID(),
		Update: &proto.UpdateSessionTrackerRequest_UpdateExpiry{
			UpdateExpiry: &proto.SessionTrackerUpdateExpiry{
				Expires: &expiry,
			},
		},
	})
	return trace.Wrap(err)
}

func (s *SessionTracker) AddParticipant(ctx context.Context, p *types.Participant) error {
	s.t.AddParticipant(*p)
	err := s.service.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: s.t.GetSessionID(),
		Update: &proto.UpdateSessionTrackerRequest_AddParticipant{
			AddParticipant: &proto.SessionTrackerAddParticipant{
				Participant: p,
			},
		},
	})
	return trace.Wrap(err)
}

func (s *SessionTracker) RemoveParticipant(ctx context.Context, participantID string) error {
	s.t.RemoveParticipant(participantID)
	err := s.service.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: s.t.GetSessionID(),
		Update: &proto.UpdateSessionTrackerRequest_RemoveParticipant{
			RemoveParticipant: &proto.SessionTrackerRemoveParticipant{
				ParticipantID: participantID,
			},
		},
	})
	return trace.Wrap(err)
}

func (s *SessionTracker) LockState() {
	s.stateUpdate.L.Lock()
}

func (s *SessionTracker) UnlockState() {
	s.stateUpdate.L.Unlock()
}

func (s *SessionTracker) UpdateState(ctx context.Context, state types.SessionState) error {
	s.LockState()
	defer s.UnlockState()

	err := s.UpdateStateUnderLock(ctx, state)
	return trace.Wrap(err)
}

// Must be called under StateLock
func (s *SessionTracker) UpdateStateUnderLock(ctx context.Context, state types.SessionState) error {
	s.t.SetState(state)
	s.stateUpdate.Broadcast()
	err := s.service.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: s.t.GetSessionID(),
		Update: &proto.UpdateSessionTrackerRequest_UpdateState{
			UpdateState: &proto.SessionTrackerUpdateState{
				State: state,
			},
		},
	})
	return trace.Wrap(err)
}

// WaitForStateUpdate waits for the tracker's state to be updated and returns the new state.
func (s *SessionTracker) WaitForStateUpdate() types.SessionState {
	s.LockState()
	defer s.UnlockState()

	currentState := s.t.GetState()
	for {
		s.stateUpdate.Wait()
		if s.t.GetState() != currentState {
			return s.t.GetState()
		}
	}
}

// Must be called under StateLock
func (s *SessionTracker) GetStateUnderLock() types.SessionState {
	return s.t.GetState()
}

func (s *SessionTracker) GetParticipants() []types.Participant {
	return s.t.GetParticipants()
}
