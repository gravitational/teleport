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
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
)

// SessionKind is a type of session.
type SessionKind string

// These represent the possible values for the kind field in session trackers.
const (
	// SSHSessionKind is the kind used for session tracking with the
	// session_tracker resource used in Teleport 9+. Note that it is
	// different from the legacy [types.KindSSHSession] value that was
	// used prior to the introduction of moderated sessions.
	SSHSessionKind            SessionKind = "ssh"
	KubernetesSessionKind     SessionKind = "k8s"
	DatabaseSessionKind       SessionKind = "db"
	AppSessionKind            SessionKind = "app"
	WindowsDesktopSessionKind SessionKind = "desktop"
	UnknownSessionKind        SessionKind = ""
)

// SessionParticipantMode is the mode that determines what you can do when you join a session.
type SessionParticipantMode string

const (
	SessionObserverMode  SessionParticipantMode = "observer"
	SessionModeratorMode SessionParticipantMode = "moderator"
	SessionPeerMode      SessionParticipantMode = "peer"
)

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

	// SetCreated sets the time at which the session was created.
	SetCreated(time.Time)

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

	// GetClusterName returns the name of the Teleport cluster.
	GetClusterName() string

	// GetLogin returns the target machine username used for this session.
	GetLogin() string

	// GetParticipants returns the list of participants in the session.
	GetParticipants() []Participant

	// AddParticipant adds a participant to the session tracker.
	AddParticipant(Participant)

	// RemoveParticipant removes a participant from the session tracker.
	RemoveParticipant(string) error

	// UpdatePresence updates presence timestamp of a participant.
	UpdatePresence(string, time.Time) error

	// GetKubeCluster returns the name of the kubernetes cluster the session is running in.
	GetKubeCluster() string

	// GetDesktopName returns the name of the Windows desktop the session is running in.
	GetDesktopName() string

	// GetAppName returns the name of the app being accessed.
	GetAppName() string

	// GetDatabaseName returns the name of the database being accessed.
	GetDatabaseName() string

	// GetHostUser fetches the user marked as the "host" of the session.
	// Things like RBAC policies are determined from this user.
	GetHostUser() string

	// GetHostPolicySets returns a list of policy sets held by the host user at the time of session creation.
	// This a subset of a role that contains some versioning and naming information in addition to the require policies
	GetHostPolicySets() []*SessionTrackerPolicySet

	// GetLastActive returns the time at which the session was last active (i.e used by any participant).
	GetLastActive() time.Time

	// HostID is the target host id that created the session tracker.
	GetHostID() string

	// GetTargetSubKind returns the sub kind of the target server.
	GetTargetSubKind() string

	// GetCommand returns the command that initiated the session.
	GetCommand() []string
}

func NewSessionTracker(spec SessionTrackerSpecV1) (SessionTracker, error) {
	session := &SessionTrackerV1{
		ResourceHeader: ResourceHeader{
			Metadata: Metadata{
				Name: spec.SessionID,
			},
		},
		Spec: spec,
	}

	if err := session.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// setStaticFields sets static resource header and metadata fields.
func (s *SessionTrackerV1) setStaticFields() {
	s.Kind = KindSessionTracker
	s.Version = V1
}

// CheckAndSetDefaults sets defaults for the session resource.
func (s *SessionTrackerV1) CheckAndSetDefaults() error {
	s.setStaticFields()

	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if s.GetCreated().IsZero() {
		s.SetCreated(time.Now())
	}

	if s.Expiry().IsZero() {
		// By default, resource expiration should match session expiration.
		expiry := s.GetExpires()
		if expiry.IsZero() {
			expiry = s.GetCreated().Add(defaults.SessionTrackerTTL)
		}
		s.SetExpiry(expiry)
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

// SetCreated returns the time at which the session was created.
func (s *SessionTrackerV1) SetCreated(created time.Time) {
	s.Spec.Created = created
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
func (s *SessionTrackerV1) GetClusterName() string {
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
			s.Spec.Participants[i], s.Spec.Participants = s.Spec.Participants[len(s.Spec.Participants)-1], s.Spec.Participants[:len(s.Spec.Participants)-1]
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

// HostID is the target host id that created the session tracker.
func (s *SessionTrackerV1) GetHostID() string {
	return s.Spec.HostID
}

// GetDesktopName returns the name of the Windows desktop the session is running in.
//
// This is only valid for Windows desktop sessions.
func (s *SessionTrackerV1) GetDesktopName() string {
	return s.Spec.DesktopName
}

// GetAppName returns the name of the app being accessed in the session.
//
// This is only valid for app sessions.
func (s *SessionTrackerV1) GetAppName() string {
	return s.Spec.AppName
}

// GetDatabaseName returns the name of the database being accessed in the session.
//
// This is only valid for database sessions.
func (s *SessionTrackerV1) GetDatabaseName() string {
	return s.Spec.DatabaseName
}

// GetHostUser fetches the user marked as the "host" of the session.
// Things like RBAC policies are determined from this user.
func (s *SessionTrackerV1) GetHostUser() string {
	return s.Spec.HostUser
}

// UpdatePresence updates presence timestamp of a participant.
func (s *SessionTrackerV1) UpdatePresence(user string, t time.Time) error {
	idx := slices.IndexFunc(s.Spec.Participants, func(participant Participant) bool {
		return participant.User == user
	})

	if idx < 0 {
		return trace.NotFound("participant %v not found", user)
	}

	s.Spec.Participants[idx].LastActive = t
	return nil
}

// GetHostPolicySets returns a list of policy sets held by the host user at the time of session creation.
// This a subset of a role that contains some versioning and naming information in addition to the require policies
func (s *SessionTrackerV1) GetHostPolicySets() []*SessionTrackerPolicySet {
	return s.Spec.HostPolicies
}

// GetLastActive returns the time at which the session was last active (i.e used by any participant).
func (s *SessionTrackerV1) GetLastActive() time.Time {
	var last time.Time

	for _, participant := range s.Spec.Participants {
		if participant.LastActive.After(last) {
			last = participant.LastActive
		}
	}

	return last
}

// GetTargetSubKind returns the sub kind of the target server.
func (s *SessionTrackerV1) GetTargetSubKind() string {
	return s.Spec.TargetSubKind
}

// GetCommand returns command that intiated the session.
func (s *SessionTrackerV1) GetCommand() []string {
	return s.Spec.InitialCommand
}

// Match checks if a given session tracker matches this filter.
func (f *SessionTrackerFilter) Match(s SessionTracker) bool {
	if f.Kind != "" && string(s.GetSessionKind()) != f.Kind {
		return false
	}
	if f.State != nil && s.GetState() != f.State.State {
		return false
	}
	if f.DesktopName != "" && s.GetDesktopName() != f.DesktopName {
		return false
	}
	return true
}
