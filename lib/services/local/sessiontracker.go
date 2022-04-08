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

package local

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	sessionPrefix                      = "session_tracker"
	retryDelay           time.Duration = time.Second
	casRetryLimit        int           = 7
	casErrorMessage      string        = "CompareAndSwap reached retry limit"
	defaultSessionExpiry time.Duration = time.Hour * 24
)

type sessionTracker struct {
	bk backend.Backend
}

func NewSessionTrackerService(bk backend.Backend) (services.SessionTrackerService, error) {
	return &sessionTracker{bk}, nil
}

func (s *sessionTracker) loadSession(ctx context.Context, sessionID string) (types.SessionTracker, error) {
	sessionJSON, err := s.bk.Get(ctx, backend.Key(sessionPrefix, sessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := unmarshalSession(sessionJSON.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// UpdatePresence updates the presence status of a user in a session.
func (s *sessionTracker) UpdatePresence(ctx context.Context, sessionID, user string) error {
	for i := 0; i < casRetryLimit; i++ {
		sessionItem, err := s.bk.Get(ctx, backend.Key(sessionPrefix, sessionID))
		if err != nil {
			return trace.Wrap(err)
		}

		session, err := unmarshalSession(sessionItem.Value)
		if err != nil {
			return trace.Wrap(err)
		}

		err = session.UpdatePresence(user)
		if err != nil {
			return trace.Wrap(err)
		}

		sessionJSON, err := marshalSession(session)
		if err != nil {
			return trace.Wrap(err)
		}

		item := backend.Item{Key: backend.Key(sessionPrefix, sessionID), Value: sessionJSON}
		_, err = s.bk.CompareAndSwap(ctx, *sessionItem, item)
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

	return trace.CompareFailed(casErrorMessage)
}

// GetSessionTracker returns the current state of a session tracker for an active session.
func (s *sessionTracker) GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error) {
	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// GetActiveSessionTrackers returns a list of active session trackers.
func (s *sessionTracker) GetActiveSessionTrackers(ctx context.Context) ([]types.SessionTracker, error) {
	prefix := backend.Key(sessionPrefix)
	result, err := s.bk.GetRange(ctx, prefix, backend.RangeEnd(prefix), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessions := make([]types.SessionTracker, 0)
	expired := make([]backend.Item, 0)
	now := time.Now().UTC()
	for _, item := range result.Items {
		session, err := unmarshalSession(item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		after := session.GetExpires().After(now)

		switch {
		case after:
			// Keep any items that aren't expired.
			sessions = append(sessions, session)
		case !after && item.Expires.IsZero():
			// Clear item if expiry is not set.
			// We shouldn't need this ideally but we currently do not set
			// tracker expiries. We will however do this in the future.
			expired = append(expired, item)
		default:
			// If the tracker has expired and there is an expiry set, we can never take this branch
			// as the backend implementation is responsible for cleaning up expired items.
			return nil, trace.AlreadyExists("Received expired key from backend, this shouldn't happen.")
		}
	}

	go func() {
		for _, item := range expired {
			if err := s.bk.Delete(ctx, item.Key); err != nil {
				if !trace.IsNotFound(err) {
					logrus.WithError(err).Error("Failed to remove stale session tracker")
				}
			}
		}
	}()

	return sessions, nil
}

// CreateSessionTracker creates a tracker resource for an active session.
func (s *sessionTracker) CreateSessionTracker(ctx context.Context, req *proto.CreateSessionTrackerRequest) (types.SessionTracker, error) {
	// Don't allow sessions that require moderation without the enterprise feature enabled.
	for _, policySet := range req.HostPolicies {
		if len(policySet.RequireSessionJoin) != 0 {
			if !modules.GetModules().Features().ModeratedSessions {
				return nil, trace.AccessDenied(
					"this Teleport cluster is not licensed for moderated sessions, please contact the cluster administrator")
			}
		}
	}

	now := time.Now().UTC()

	spec := types.SessionTrackerSpecV1{
		SessionID:         req.ID,
		Kind:              req.Type,
		State:             types.SessionState_SessionStatePending,
		Created:           now,
		Reason:            req.Reason,
		Invited:           req.Invited,
		Hostname:          req.Hostname,
		Address:           req.Address,
		ClusterName:       req.ClusterName,
		Login:             req.Login,
		Participants:      []types.Participant{*req.Initiator},
		Expires:           req.Expires,
		KubernetesCluster: req.KubernetesCluster,
		HostUser:          req.HostUser,
	}

	if spec.Expires.IsZero() {
		spec.Expires = now.Add(defaultSessionExpiry)
	}

	session, err := types.NewSessionTracker(spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	json, err := marshalSession(session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{Key: backend.Key(sessionPrefix, session.GetSessionID()), Value: json}
	_, err = s.bk.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// UpdateSessionTracker updates a tracker resource for an active session.
func (s *sessionTracker) UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error {
	for i := 0; i < casRetryLimit; i++ {
		sessionItem, err := s.bk.Get(ctx, backend.Key(sessionPrefix, req.SessionID))
		if err != nil {
			return trace.Wrap(err)
		}

		session, err := unmarshalSession(sessionItem.Value)
		if err != nil {
			return trace.Wrap(err)
		}

		switch session := session.(type) {
		case *types.SessionTrackerV1:
			switch update := req.Update.(type) {
			case *proto.UpdateSessionTrackerRequest_UpdateState:
				session.SetState(update.UpdateState.State)
			case *proto.UpdateSessionTrackerRequest_AddParticipant:
				session.AddParticipant(*update.AddParticipant.Participant)
			case *proto.UpdateSessionTrackerRequest_RemoveParticipant:
				session.RemoveParticipant(update.RemoveParticipant.ParticipantID)
			}
		default:
			return trace.BadParameter("unrecognized session version %T", session)
		}

		sessionJSON, err := marshalSession(session)
		if err != nil {
			return trace.Wrap(err)
		}

		item := backend.Item{Key: backend.Key(sessionPrefix, req.SessionID), Value: sessionJSON}
		_, err = s.bk.CompareAndSwap(ctx, *sessionItem, item)
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

	return trace.CompareFailed(casErrorMessage)
}

// RemoveSessionTracker removes a tracker resource for an active session.
func (s *sessionTracker) RemoveSessionTracker(ctx context.Context, sessionID string) error {
	return trace.Wrap(s.bk.Delete(ctx, backend.Key(sessionPrefix, sessionID)))
}

// unmarshalSession unmarshals the Session resource from JSON.
func unmarshalSession(bytes []byte, opts ...services.MarshalOption) (types.SessionTracker, error) {
	var session types.SessionTrackerV1

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(bytes, &session); err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	if err := session.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		session.SetResourceID(cfg.ID)
	}

	if !cfg.Expires.IsZero() {
		session.SetExpiry(cfg.Expires)
	}

	return &session, nil
}

// marshalSession marshals the Session resource to JSON.
func marshalSession(session types.SessionTracker, opts ...services.MarshalOption) ([]byte, error) {
	if err := session.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch session := session.(type) {
	case *types.SessionTrackerV1:
		if !cfg.PreserveResourceID {
			copy := *session
			copy.SetResourceID(0)
			session = &copy
		}
		return utils.FastMarshal(session)
	default:
		return nil, trace.BadParameter("unrecognized session version %T", session)
	}
}
