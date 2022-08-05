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
	closeC chan struct{}
	// tracker is the in memory session tracker
	tracker types.SessionTracker
	// trackerCond is used to provide synchronized access to tracker
	// and to broadcast state changes.
	trackerCond *sync.Cond
	// service is used to share session tracker updates with the service
	service services.SessionTrackerService
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
		tracker:     t,
		trackerCond: sync.NewCond(&sync.Mutex{}),
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
// until the SessionTracker or ctx is closed.
func (s *SessionTracker) UpdateExpirationLoop(ctx context.Context, clock clockwork.Clock) error {
	ticker := clock.NewTicker(sessionTrackerExpirationUpdateInterval)
	defer ticker.Stop()
	return s.updateExpirationLoop(ctx, ticker)
}

// updateExpirationLoop is used in tests
func (s *SessionTracker) updateExpirationLoop(ctx context.Context, ticker clockwork.Ticker) error {
	for {
		select {
		case time := <-ticker.Chan():
			expiry := time.Add(apidefaults.SessionTrackerTTL)
			if err := s.UpdateExpiration(ctx, expiry); err != nil {
				return trace.Wrap(err)
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-s.closeC:
			return nil
		}
	}
}

func (s *SessionTracker) UpdateExpiration(ctx context.Context, expiry time.Time) error {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	s.tracker.SetExpiry(expiry)
	s.trackerCond.Broadcast()

	err := s.service.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: s.tracker.GetSessionID(),
		Update: &proto.UpdateSessionTrackerRequest_UpdateExpiry{
			UpdateExpiry: &proto.SessionTrackerUpdateExpiry{
				Expires: &expiry,
			},
		},
	})
	return trace.Wrap(err)
}

func (s *SessionTracker) AddParticipant(ctx context.Context, p *types.Participant) error {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	s.tracker.AddParticipant(*p)
	s.trackerCond.Broadcast()

	err := s.service.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: s.tracker.GetSessionID(),
		Update: &proto.UpdateSessionTrackerRequest_AddParticipant{
			AddParticipant: &proto.SessionTrackerAddParticipant{
				Participant: p,
			},
		},
	})
	return trace.Wrap(err)
}

func (s *SessionTracker) RemoveParticipant(ctx context.Context, participantID string) error {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	s.tracker.RemoveParticipant(participantID)
	s.trackerCond.Broadcast()

	err := s.service.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: s.tracker.GetSessionID(),
		Update: &proto.UpdateSessionTrackerRequest_RemoveParticipant{
			RemoveParticipant: &proto.SessionTrackerRemoveParticipant{
				ParticipantID: participantID,
			},
		},
	})
	return trace.Wrap(err)
}

func (s *SessionTracker) UpdateState(ctx context.Context, state types.SessionState) error {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	s.tracker.SetState(state)
	s.trackerCond.Broadcast()

	err := s.service.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: s.tracker.GetSessionID(),
		Update: &proto.UpdateSessionTrackerRequest_UpdateState{
			UpdateState: &proto.SessionTrackerUpdateState{
				State: state,
			},
		},
	})
	return trace.Wrap(err)
}

// WaitForStateUpdate waits for the tracker's state to be updated and returns the new state.
func (s *SessionTracker) WaitForStateUpdate(initialState types.SessionState) types.SessionState {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()

	for {
		if state := s.tracker.GetState(); state != initialState {
			return state
		}
		s.trackerCond.Wait()
	}
}

func (s *SessionTracker) GetState() types.SessionState {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	return s.tracker.GetState()
}

func (s *SessionTracker) GetParticipants() []types.Participant {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	return s.tracker.GetParticipants()
}
