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

package local

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const (
	sessionPrefix           = "session_tracker"
	retryDelay              = time.Second
	terminatedTTL           = 3 * time.Minute
	updateRetryLimit        = 7
	updateRetryLimitMessage = "Update retry limit reached"
)

type sessionTracker struct {
	bk backend.Backend
}

func NewSessionTrackerService(bk backend.Backend) (services.SessionTrackerService, error) {
	return &sessionTracker{bk}, nil
}

func (s *sessionTracker) loadSession(ctx context.Context, sessionID string) (types.SessionTracker, error) {
	sessionJSON, err := s.bk.Get(ctx, backend.NewKey(sessionPrefix, sessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := services.UnmarshalSessionTracker(sessionJSON.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// UpdatePresence updates the presence status of a user in a session.
func (s *sessionTracker) UpdatePresence(ctx context.Context, sessionID, user string) error {
	for i := 0; i < updateRetryLimit; i++ {
		sessionItem, err := s.bk.Get(ctx, backend.NewKey(sessionPrefix, sessionID))
		if err != nil {
			return trace.Wrap(err)
		}

		session, err := services.UnmarshalSessionTracker(sessionItem.Value)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := session.UpdatePresence(user, s.bk.Clock().Now().UTC()); err != nil {
			return trace.Wrap(err)
		}

		sessionJSON, err := services.MarshalSessionTracker(session)
		if err != nil {
			return trace.Wrap(err)
		}

		item := backend.Item{
			Key:      backend.NewKey(sessionPrefix, sessionID),
			Value:    sessionJSON,
			Expires:  session.Expiry(),
			Revision: sessionItem.Revision,
		}
		_, err = s.bk.ConditionalUpdate(ctx, item)
		if trace.IsCompareFailed(err) {
			select {
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			case <-time.After(retryDelay):
				continue
			}
		}

		return trace.Wrap(err)
	}

	return trace.CompareFailed(updateRetryLimitMessage)
}

// GetSessionTracker returns the current state of a session tracker for an active session.
func (s *sessionTracker) GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error) {
	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

func (s *sessionTracker) getActiveSessionTrackers(ctx context.Context, filter *types.SessionTrackerFilter) ([]types.SessionTracker, error) {
	prefix := backend.ExactKey(sessionPrefix)
	result, err := s.bk.GetRange(ctx, prefix, backend.RangeEnd(prefix), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessions := make([]types.SessionTracker, 0, len(result.Items))

	// We don't overallocate expired since cleaning up sessions here should be rare.
	var noExpiry []backend.Item
	now := s.bk.Clock().Now()
	for _, item := range result.Items {
		session, err := services.UnmarshalSessionTracker(item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// NOTE: This is the session expiry timestamp, not the backend timestamp stored in `item.Expires`.
		exp := session.GetExpires()
		if session.Expiry().After(exp) {
			exp = session.Expiry()
		}

		after := exp.After(now)

		switch {
		case after:
			// Keep any items that aren't expired and which match the filter.
			if filter == nil || (filter != nil && filter.Match(session)) {
				sessions = append(sessions, session)
			}
		case !after && item.Expires.IsZero():
			// Clear item if expiry is not set on the backend.
			noExpiry = append(noExpiry, item)
		default:
			// If we take this branch, the expiry is set and the backend is responsible for cleaning up the item.
		}
	}

	if len(noExpiry) > 0 {
		go func() {
			for _, item := range noExpiry {
				if err := s.bk.Delete(ctx, item.Key); err != nil {
					if !trace.IsNotFound(err) {
						slog.ErrorContext(ctx, "Failed to remove stale session tracker", "error", err)
					}
				}
			}
		}()
	}

	return sessions, nil
}

// GetActiveSessionTrackers returns a list of active session trackers.
func (s *sessionTracker) GetActiveSessionTrackers(ctx context.Context) ([]types.SessionTracker, error) {
	return s.getActiveSessionTrackers(ctx, nil)
}

// GetActiveSessionTrackersWithFilter returns a list of active sessions filtered by a filter.
func (s *sessionTracker) GetActiveSessionTrackersWithFilter(ctx context.Context, filter *types.SessionTrackerFilter) ([]types.SessionTracker, error) {
	return s.getActiveSessionTrackers(ctx, filter)
}

// CreateSessionTracker creates a tracker resource for an active session.
func (s *sessionTracker) CreateSessionTracker(ctx context.Context, tracker types.SessionTracker) (types.SessionTracker, error) {
	json, err := services.MarshalSessionTracker(tracker)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.NewKey(sessionPrefix, tracker.GetSessionID()),
		Value:   json,
		Expires: tracker.Expiry(),
	}
	_, err = s.bk.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return tracker, nil
}

// UpdateSessionTracker updates a tracker resource for an active session.
func (s *sessionTracker) UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error {
	for i := 0; i < updateRetryLimit; i++ {
		sessionItem, err := s.bk.Get(ctx, backend.NewKey(sessionPrefix, req.SessionID))
		if err != nil {
			return trace.Wrap(err)
		}

		session, err := services.UnmarshalSessionTracker(sessionItem.Value)
		if err != nil {
			return trace.Wrap(err)
		}

		switch session := session.(type) {
		case *types.SessionTrackerV1:
			switch update := req.Update.(type) {
			case *proto.UpdateSessionTrackerRequest_UpdateState:
				// Since we are using a CAS loop, we can safely check the state of the session
				// before updating it. If the session is already closed, we can return an error
				// to the caller to indicate that the session is no longer active and the update
				// should not be applied.
				// Before, we were relying on the caller to send the updates in the correct order
				// and to not send updates for closed sessions. This was error prone and led to
				// sessions getting stuck as active. The expiry of the session was correctly stored
				// in the backend, but since dynamodb deletion is eventually consistent, the session
				// could still be returned by GetActiveSessionTrackers for days if a
				// running event is received after the session termination event.
				if session.GetState() == types.SessionState_SessionStateTerminated {
					return trace.BadParameter("session %q is already closed", session.GetSessionID())
				}

				if err := session.SetState(update.UpdateState.State); err != nil {
					return trace.Wrap(err)
				}
			case *proto.UpdateSessionTrackerRequest_AddParticipant:
				session.AddParticipant(*update.AddParticipant.Participant)
			case *proto.UpdateSessionTrackerRequest_RemoveParticipant:
				if err := session.RemoveParticipant(update.RemoveParticipant.ParticipantID); err != nil {
					return trace.Wrap(err)
				}
			case *proto.UpdateSessionTrackerRequest_UpdateExpiry:
				session.SetExpiry(*update.UpdateExpiry.Expires)
			}
		default:
			return trace.BadParameter("unrecognized session version %T", session)
		}

		sessionJSON, err := services.MarshalSessionTracker(session)
		if err != nil {
			return trace.Wrap(err)
		}

		expiry := session.Expiry()

		// Terminated sessions don't need to stick around for the full TTL.
		// Instead of explicitly deleting the item from the backend the TTL
		// is set to a sooner time so that the backend can automatically
		// clean it up.
		if session.GetState() == types.SessionState_SessionStateTerminated {
			expiry = s.bk.Clock().Now().UTC().Add(terminatedTTL)
		}

		item := backend.Item{
			Key:      backend.NewKey(sessionPrefix, req.SessionID),
			Value:    sessionJSON,
			Expires:  expiry,
			Revision: sessionItem.Revision,
		}
		_, err = s.bk.ConditionalUpdate(ctx, item)
		if trace.IsCompareFailed(err) {
			select {
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			case <-time.After(retryDelay):
				continue
			}
		}

		return trace.Wrap(err)
	}

	return trace.CompareFailed(updateRetryLimitMessage)
}

// RemoveSessionTracker removes a tracker resource for an active session.
func (s *sessionTracker) RemoveSessionTracker(ctx context.Context, sessionID string) error {
	return trace.Wrap(s.bk.Delete(ctx, backend.NewKey(sessionPrefix, sessionID)))
}
