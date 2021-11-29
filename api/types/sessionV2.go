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

package types

import (
	"time"

	"github.com/gravitational/trace"
)

const (
	SSHSessionKind        SessionKind            = "ssh"
	KubernetesSessionKind SessionKind            = "k8s"
	SessionObserverMode   SessionParticipantMode = "observer"
	SessionModeratorMode  SessionParticipantMode = "moderator"
	SessionPeerMode       SessionParticipantMode = "peer"
)

type SessionKind string
type SessionParticipantMode string

type Session interface {
	Resource

	GetID() string

	GetNamespace() string

	GetSessionKind() SessionKind

	GetState() SessionState

	SetState(SessionState) error

	GetCreated() time.Time

	GetExpires() time.Time

	GetReason() string

	GetInvited() []string

	GetLastActive() time.Time

	SetLastActive(string)

	GetHostname() string

	GetAddress() string

	GetClustername() string

	GetLogin() string

	GetParticipants() []*Participant

	AddParticipant(*Participant)

	RemoveParticipant(string) error

	GetKubeCluster() string

	GetHostUser() string
}

func NewSession(spec SessionSpecV3) (Session, error) {
	meta := Metadata{
		Name: spec.SessionID,
	}

	session := &SessionV3{Metadata: meta, Spec: spec}

	if err := session.Metadata.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// GetVersion returns resource version.
func (c *SessionV3) GetVersion() string {
	return c.Version
}

// GetName returns the name of the resource.
func (c *SessionV3) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the resource.
func (c *SessionV3) SetName(e string) {
	c.Metadata.Name = e
}

// SetExpiry sets expiry time for the object.
func (c *SessionV3) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting.
func (c *SessionV3) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetMetadata returns object metadata.
func (c *SessionV3) GetMetadata() Metadata {
	return c.Metadata
}

// GetResourceID returns resource ID.
func (c *SessionV3) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID.
func (c *SessionV3) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetKind returns resource kind.
func (c *SessionV3) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource subkind.
func (c *SessionV3) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind.
func (c *SessionV3) SetSubKind(sk string) {
	c.SubKind = sk
}

func (s *SessionV3) CheckAndSetDefaults() error {
	s.Kind = KindSessionTracker
	s.Version = V3

	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *SessionV3) GetID() string {
	return s.Spec.SessionID
}

func (s *SessionV3) GetNamespace() string {
	return s.Spec.Namespace
}

func (s *SessionV3) GetSessionKind() SessionKind {
	return SessionKind(s.Spec.Type)
}

func (s *SessionV3) GetState() SessionState {
	return s.Spec.State
}

func (s *SessionV3) SetState(state SessionState) error {
	switch state {
	default:
		return trace.BadParameter("invalid session state: %v", state)
	case SessionState_SessionStateRunning:
		fallthrough
	case SessionState_SessionStatePending:
		fallthrough
	case SessionState_SessionStateTerminated:
		s.Spec.State = state
		return nil
	}
}

func (s *SessionV3) GetCreated() time.Time {
	return s.Spec.Created
}

func (s *SessionV3) GetExpires() time.Time {
	return s.Spec.Expires
}

func (s *SessionV3) GetReason() string {
	return s.Spec.Reason
}

func (s *SessionV3) GetInvited() []string {
	return s.Spec.Invited
}

func (s *SessionV3) GetLastActive() time.Time {
	return s.Spec.LastActive
}

func (s *SessionV3) SetLastActive(participantID string) {
	now := time.Now()
	s.Spec.LastActive = now

	for _, participant := range s.Spec.Participants {
		if participant.ID == participantID {
			participant.LastActive = now
			return
		}
	}
}

func (s *SessionV3) GetHostname() string {
	return s.Spec.Hostname
}

func (s *SessionV3) GetAddress() string {
	return s.Spec.Address
}

func (s *SessionV3) GetClustername() string {
	return s.Spec.ClusterName
}

func (s *SessionV3) GetLogin() string {
	return s.Spec.Login
}

func (s *SessionV3) GetParticipants() []*Participant {
	return s.Spec.Participants
}

func (s *SessionV3) AddParticipant(participant *Participant) {
	s.Spec.Participants = append(s.Spec.Participants, participant)
}

func (s *SessionV3) RemoveParticipant(id string) error {
	for i, participant := range s.Spec.Participants {
		if participant.ID == id {
			s.Spec.Participants = append(s.Spec.Participants[:i], s.Spec.Participants[i+1:]...)
			return nil
		}
	}

	return trace.BadParameter("participant %v not found", id)
}

func (s *SessionV3) GetKubeCluster() string {
	return s.Spec.KubernetesCluster
}

func (s *SessionV3) GetHostUser() string {
	return s.Spec.HostUser
}
