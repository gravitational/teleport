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
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

const (
	sessionV2Prefix = "session_v2"
	sessionV2List   = "list"
)

// SessionV2 is a realtime session service that has information about
// sessions that are in-flight in the cluster at the moment.
type SessionV2 interface {
	GetActiveSessionTrackers(ctx context.Context) ([]types.Session, error)
	CreateSessionTracker(ctx context.Context, req *proto.CreateSessionRequest) (types.Session, error)
	UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionRequest) error
	RemoveSessionTracker(ctx context.Context, sessionID string) error
}

type sessionV2 struct {
	bk backend.Backend
}

func NewSessionV2Service(bk backend.Backend) (SessionV2, error) {
	_, err := bk.Get(context.TODO(), backend.Key(sessionV2Prefix, sessionV2List))
	if trace.IsNotFound(err) {
		err := createList(bk)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	return &sessionV2{bk}, nil
}

func createList(bk backend.Backend) error {
	data, err := utils.FastMarshal([]string{})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = bk.Create(context.TODO(), backend.Item{Key: backend.Key(sessionV2Prefix, sessionV2List), Value: data})
	if err != nil {
		return err
	}

	return nil
}

func (s *sessionV2) GetActiveSessionTrackers(ctx context.Context) ([]types.Session, error) {
	sessionList, err := s.getSessionList(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessions := make([]types.Session, len(sessionList))
	for i, sessionID := range sessionList {
		sessionJSON, err := s.bk.Get(ctx, backend.Key(sessionV2Prefix, sessionID))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		session, err := unmarshalSession(sessionJSON.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sessions[i] = session
	}

	return sessions, nil
}

func (s *sessionV2) CreateSessionTracker(ctx context.Context, req *proto.CreateSessionRequest) (types.Session, error) {
	now := time.Now().UTC()

	spec := types.SessionSpecV3{
		SessionID:         req.ID,
		Namespace:         req.Namespace,
		Type:              req.Type,
		State:             types.SessionState_SessionStatePending,
		Created:           now,
		LastActive:        now,
		Reason:            req.Reason,
		Invited:           req.Invited,
		Hostname:          req.Hostname,
		Address:           req.Address,
		ClusterName:       req.ClusterName,
		Login:             req.Login,
		Participants:      []*types.Participant{req.Initiator},
		Expires:           req.Expires,
		KubernetesCluster: req.KubernetesCluster,
		HostUser:          req.HostUser,
	}

	session, err := types.NewSession(spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	json, err := marshalSession(session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.addSessionToList(ctx, session.GetID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{Key: backend.Key(sessionV2Prefix, session.GetID()), Value: json}
	_, err = s.bk.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

func (s *sessionV2) UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionRequest) error {
	sessionItem, err := s.bk.Get(ctx, backend.Key(sessionV2Prefix, req.SessionID))
	if err != nil {
		return trace.Wrap(err)
	}

	session, err := unmarshalSession(sessionItem.Value)
	if err != nil {
		return trace.Wrap(err)
	}

	switch session := session.(type) {
	case *types.SessionV3:
		switch update := req.Update.(type) {
		case *proto.UpdateSessionRequest_UpdateState:
			session.SetState(update.UpdateState.State)
		case *proto.UpdateSessionRequest_UpdateActivity:
			session.SetLastActive(update.UpdateActivity.ParticipantID)
		case *proto.UpdateSessionRequest_AddParticipant:
			session.AddParticipant(update.AddParticipant.Participant)
		case *proto.UpdateSessionRequest_RemoveParticipant:
			session.RemoveParticipant(update.RemoveParticipant.ParticipantID)
		}
	default:
		return trace.BadParameter("unrecognized session version %T", session)
	}

	sessionJSON, err := marshalSession(session)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{Key: backend.Key(sessionV2Prefix, req.SessionID), Value: sessionJSON}
	_, err = s.bk.CompareAndSwap(ctx, *sessionItem, item)
	if trace.IsCompareFailed(err) {
		return s.UpdateSessionTracker(ctx, req)
	} else {
		return trace.Wrap(err)
	}
}

func (s *sessionV2) RemoveSessionTracker(ctx context.Context, sessionID string) error {
	err := s.removeSessionFromList(ctx, sessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.bk.Delete(ctx, backend.Key(sessionV2Prefix, sessionID)))
}

func (s *sessionV2) addSessionToList(ctx context.Context, sessionID string) error {
	listItem, err := s.bk.Get(ctx, backend.Key(sessionV2Prefix, sessionV2List))
	if err != nil {
		return trace.Wrap(err)
	}

	var list []string
	err = utils.FastUnmarshal(listItem.Value, &list)
	if err != nil {
		return trace.Wrap(err)
	}

	list = append(list, sessionID)
	listJSON, err := utils.FastMarshal(list)
	if err != nil {
		return trace.Wrap(err)
	}

	newListItem := backend.Item{Key: backend.Key(sessionV2Prefix, sessionV2List), Value: listJSON}
	_, err = s.bk.CompareAndSwap(ctx, *listItem, newListItem)
	return trace.Wrap(err)
}

func (s *sessionV2) removeSessionFromList(ctx context.Context, sessionID string) error {
	listItem, err := s.bk.Get(ctx, backend.Key(sessionV2Prefix, sessionV2List))
	if err != nil {
		return trace.Wrap(err)
	}

	var list []string
	err = utils.FastUnmarshal(listItem.Value, &list)
	if err != nil {
		return trace.Wrap(err)
	}

	found := false
	for i, id := range list {
		if id == sessionID {
			list = append(list[:i], list[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return trace.BadParameter("session %v not found in list", sessionID)
	}

	listJSON, err := utils.FastMarshal(list)
	if err != nil {
		return trace.Wrap(err)
	}

	newListItem := backend.Item{Key: backend.Key(sessionV2Prefix, sessionV2List), Value: listJSON}
	_, err = s.bk.Update(ctx, newListItem)
	return trace.Wrap(err)
}

func (s *sessionV2) getSessionList(ctx context.Context) ([]string, error) {
	listItem, err := s.bk.Get(ctx, backend.Key(sessionV2Prefix, sessionV2List))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var list []string
	err = utils.FastUnmarshal(listItem.Value, &list)
	return list, trace.Wrap(err)
}

// unmarshalSession unmarshals the Session resource from JSON.
func unmarshalSession(bytes []byte, opts ...MarshalOption) (types.Session, error) {
	var session types.SessionV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
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
func marshalSession(session types.Session, opts ...MarshalOption) ([]byte, error) {
	if err := session.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch session := session.(type) {
	case *types.SessionV3:
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
