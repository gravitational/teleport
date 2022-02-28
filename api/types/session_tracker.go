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

// SessionKind is a type of session.
type SessionKind string

// SessionParticipantMode is the mode that determines what you can do when you join a session.
type SessionParticipantMode string

// SessionTracker is a resource which tracks an active session.
type SessionTracker interface {
	Resource

	// GetSessionID returns the ID of the session.
	GetSessionID() string

	// GetSessionKind returns the kind of the session.
	GetSessionKind() SessionKind

	// GetState returns the state of the session.
	GetState() SessionState

	// SetState sets the state of the session.
	SetState(SessionState) error

	// GetCreated returns the time at which the session was created.
	GetCreated() time.Time

	// GetExpires return the time at which the session expires.
	GetExpires() time.Time

	// GetReason returns the reason for the session.
	GetReason() string

	// GetInvited returns a list of people invited to the session.
	GetInvited() []string

	// GetHostname returns the hostname of the session target.
	GetHostname() string

	// GetAddress returns the address of the session target.
	GetAddress() string

	// GetClustername returns the name of the cluster.
	GetClustername() string

	// GetLogin returns the target machine username used for this session.
	GetLogin() string

	// GetParticipants returns the list of participants in the session.
	GetParticipants() []Participant

	// AddParticipant adds a participant to the session tracker.
	AddParticipant(Participant)

	// RemoveParticipant removes a participant from the session tracker.
	RemoveParticipant(string) error

	// UpdatePresence updates presence timestamp of a participant.
	UpdatePresence(string) error

	// GetKubeCluster returns the name of the kubernetes cluster the session is running in.
	GetKubeCluster() string

	// GetHostUser fetches the user marked as the "host" of the session.
	// Things like RBAC policies are determined from this user.
	GetHostUser() string

	// GetHostPolicySets returns a list of policy sets held by the host user at the time of session creation.
	// This a subset of a role that contains some versioning and naming information in addition to the require policies
	GetHostPolicySets() []*SessionTrackerPolicySet
}

func NewSessionTracker(spec SessionTrackerSpecV1) (SessionTracker, error) {
	meta := Metadata{
		Name: spec.SessionID,
	}

	session := &SessionTrackerV1{
		ResourceHeader: ResourceHeader{
			Kind:     KindSessionTracker,
			Version:  V1,
			Metadata: meta,
		},
		Spec: spec,
	}

	if err := session.Metadata.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// GetVersion returns resource version.
func (s *SessionTrackerV1) GetVersion() string {
	return s.Version
}

// GetName returns the name of the resource.
func (s *SessionTrackerV1) GetName() string {
	return s.Metadata.Name
}

// SetName sets the name of the resource.
func (s *SessionTrackerV1) SetName(e string) {
	s.Metadata.Name = e
}

// SetExpiry sets expiry time for the object.
func (s *SessionTrackerV1) SetExpiry(expires time.Time) {
	s.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting.
func (s *SessionTrackerV1) Expiry() time.Time {
	return s.Metadata.Expiry()
}

// GetMetadata returns object metadata.
func (s *SessionTrackerV1) GetMetadata() Metadata {
	return s.Metadata
}

// GetResourceID returns resource ID.
func (s *SessionTrackerV1) GetResourceID() int64 {
	return s.Metadata.ID
}

// SetResourceID sets resource ID.
func (s *SessionTrackerV1) SetResourceID(id int64) {
	s.Metadata.ID = id
}

// GetKind returns resource kind.
func (s *SessionTrackerV1) GetKind() string {
	return s.Kind
}

// GetSubKind returns resource subkind.
func (s *SessionTrackerV1) GetSubKind() string {
	return s.SubKind
}

// SetSubKind sets resource subkind.
func (s *SessionTrackerV1) SetSubKind(sk string) {
	s.SubKind = sk
}

// CheckAndSetDefaults sets defaults for the session resource.
func (s *SessionTrackerV1) CheckAndSetDefaults() error {
	s.Kind = KindSessionTracker
	s.Version = V1

	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetSessionID returns the ID of the session.
func (s *SessionTrackerV1) GetSessionID() string {
	return s.Spec.SessionID
}

// GetSessionKind returns the kind of the session.
func (s *SessionTrackerV1) GetSessionKind() SessionKind {
	return SessionKind(s.Spec.Kind)
}

// GetState returns the state of the session.
func (s *SessionTrackerV1) GetState() SessionState {
	return s.Spec.State
}

// SetState sets the state of the session.
func (s *SessionTrackerV1) SetState(state SessionState) error {
	switch state {
	case SessionState_SessionStateRunning, SessionState_SessionStatePending, SessionState_SessionStateTerminated:
		s.Spec.State = state
		return nil
	default:
		return trace.BadParameter("invalid session state: %v", state)
	}
}

// GetCreated returns the time at which the session was created.
func (s *SessionTrackerV1) GetCreated() time.Time {
	return s.Spec.Created
}

// GetExpires return the time at which the session expires.
func (s *SessionTrackerV1) GetExpires() time.Time {
	return s.Spec.Expires
}

// GetReason returns the reason for the session.
func (s *SessionTrackerV1) GetReason() string {
	return s.Spec.Reason
}

// GetInvited returns a list of people invited to the session.
func (s *SessionTrackerV1) GetInvited() []string {
	return s.Spec.Invited
}

// GetHostname returns the hostname of the session target.
func (s *SessionTrackerV1) GetHostname() string {
	return s.Spec.Hostname
}

// GetAddress returns the address of the session target.
func (s *SessionTrackerV1) GetAddress() string {
	return s.Spec.Address
}

// GetClustername returns the name of the cluster the session is running in.
func (s *SessionTrackerV1) GetClustername() string {
	return s.Spec.ClusterName
}

// GetLogin returns the target machine username used for this session.
func (s *SessionTrackerV1) GetLogin() string {
	return s.Spec.Login
}

// GetParticipants returns a list of participants in the session.
func (s *SessionTrackerV1) GetParticipants() []Participant {
	return s.Spec.Participants
}

// AddParticipant adds a participant to the session tracker.
func (s *SessionTrackerV1) AddParticipant(participant Participant) {
	s.Spec.Participants = append(s.Spec.Participants, participant)
}

// RemoveParticipant removes a participant from the session tracker.
func (s *SessionTrackerV1) RemoveParticipant(id string) error {
	for i, participant := range s.Spec.Participants {
		if participant.ID == id {
			s.Spec.Participants = append(s.Spec.Participants[:i], s.Spec.Participants[i+1:]...)
			return nil
		}
	}

	return trace.NotFound("participant %v not found", id)
}

// GetKubeCluster returns the name of the kubernetes cluster the session is running in.
//
// This is only valid for kubernetes sessions.
func (s *SessionTrackerV1) GetKubeCluster() string {
	return s.Spec.KubernetesCluster
}

// GetHostUser fetches the user marked as the "host" of the session.
// Things like RBAC policies are determined from this user.
func (s *SessionTrackerV1) GetHostUser() string {
	return s.Spec.HostUser
}

// UpdatePresence updates presence timestamp of a participant.
func (s *SessionTrackerV1) UpdatePresence(user string) error {
	for _, participant := range s.Spec.Participants {
		if participant.User == user {
			participant.LastActive = time.Now().UTC()
			return nil
		}
	}

	return trace.NotFound("participant %v not found", user)
}

// GetHostPolicySets returns a list of policy sets held by the host user at the time of session creation.
// This a subset of a role that contains some versioning and naming information in addition to the require policies
func (s *SessionTrackerV1) GetHostPolicySets() []*SessionTrackerPolicySet {
	return s.Spec.HostPolicies
}
