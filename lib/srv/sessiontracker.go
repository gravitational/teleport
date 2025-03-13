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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
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
	t, err := types.NewSessionTracker(trackerSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if service != nil {
		if t, err = service.CreateSessionTracker(ctx, t); err != nil {
			return nil, trace.Wrap(err)
		}
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

const sessionTrackerExpirationUpdateInterval = apidefaults.SessionTrackerTTL / 3

// UpdateExpirationLoop extends the session tracker expiration by 30 minutes every 10 minutes
// until the SessionTracker or ctx is closed. If there is a failure to write the updated
// SessionTracker to the backend, the write is retried with exponential backoff up until the original
// SessionTracker expiry.
func (s *SessionTracker) UpdateExpirationLoop(ctx context.Context, clock clockwork.Clock) error {
	// Use a timer and reset every loop (instead of a ticker) for two reasons:
	// 1. We always want to wait the full interval after a successful update,
	//    whether or not a retry was necessary.
	// 2. It's easier for tests to wait for the timer to be reset, tickers
	//    always count as a blocker.
	timer := clock.NewTimer(sessionTrackerExpirationUpdateInterval)
	defer timer.Stop()

	for {
		select {
		case t := <-timer.Chan():
			expiry := t.Add(apidefaults.SessionTrackerTTL)
			if err := s.UpdateExpiration(ctx, expiry); err != nil {
				// If the tracker doesn't exist in the backend then
				// the update loop will never succeed.
				if trace.IsNotFound(err) {
					return trace.Wrap(err)
				}

				if err := s.retryUpdate(ctx, clock); err != nil {
					return trace.Wrap(err)
				}
			}
			// Tracker was updated, reset the timer to wait another full
			// update interval and proceed with the update loop.
			timer.Reset(sessionTrackerExpirationUpdateInterval)
		case <-ctx.Done():
			return nil
		case <-s.closeC:
			return nil
		}
	}
}

// retryUpdate attempts to periodically retry updating the session tracker
func (s *SessionTracker) retryUpdate(ctx context.Context, clock clockwork.Clock) error {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Clock:  clock,
		Max:    3 * time.Minute,
		First:  time.Minute,
		Step:   time.Minute,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	originalExpiry := s.tracker.Expiry()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.closeC:
			return nil
		case <-retry.After():
			retry.Inc()

			// try sending another update
			err := s.UpdateExpiration(ctx, clock.Now().Add(apidefaults.SessionTrackerTTL))

			// update was successful return
			if err == nil {
				return nil
			}

			// the tracker wasn't found which means we were
			// able to reach the auth server, but the tracker
			// no longer exists and likely expired
			if trace.IsNotFound(err) {
				return trace.Wrap(err)
			}

			// the tracker has grown stale and retrying
			// can be aborted
			if clock.Now().UTC().After(originalExpiry.UTC()) {
				return trace.Wrap(err)
			}
		}
	}
}

func (s *SessionTracker) UpdateExpiration(ctx context.Context, expiry time.Time) error {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	s.tracker.SetExpiry(expiry)
	s.trackerCond.Broadcast()

	if s.service != nil {
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

	return nil
}

func (s *SessionTracker) AddParticipant(ctx context.Context, p *types.Participant) error {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	s.tracker.AddParticipant(*p)
	s.trackerCond.Broadcast()

	if s.service != nil {
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

	return nil
}

func (s *SessionTracker) UpdateChatlog(ctx context.Context, chatlog []string) error {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	s.tracker.SetChatLog(chatlog)
	s.trackerCond.Broadcast()

	fmt.Printf("\n\n\n\nUPDATED CHATLOG 2: %v\n\n\n\n", chatlog)

	if s.service != nil {
		err := s.service.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
			SessionID: s.tracker.GetSessionID(),
			Update: &proto.UpdateSessionTrackerRequest_UpdateChatlog{
				UpdateChatlog: &proto.SessionTrackerUpdateChatlog{
					ChatLog: chatlog,
				},
			},
		})

		return trace.Wrap(err)
	}

	return nil
}

func (s *SessionTracker) RemoveParticipant(ctx context.Context, participantID string) error {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	if err := s.tracker.RemoveParticipant(participantID); err != nil {
		return trace.Wrap(err)
	}
	s.trackerCond.Broadcast()

	if s.service != nil {
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

	return nil
}

func (s *SessionTracker) UpdateState(ctx context.Context, state types.SessionState) error {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	s.tracker.SetState(state)
	s.trackerCond.Broadcast()

	if s.service != nil {
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

	return nil
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

// WaitOnState waits until the desired state is reached or the context is canceled.
func (s *SessionTracker) WaitOnState(ctx context.Context, wanted types.SessionState) error {
	go func() {
		<-ctx.Done()
		s.trackerCond.Broadcast()
	}()

	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if s.tracker.GetState() == wanted {
				return nil
			}

			s.trackerCond.Wait()
		}
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

func (s *SessionTracker) GetChatLog() []string {
	s.trackerCond.L.Lock()
	defer s.trackerCond.L.Unlock()
	return s.tracker.GetChatLog()
}
