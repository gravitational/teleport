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

type Session interface {
	Resource

	GetID() string

	GetNamespace() string

	GetType() SessionType

	GetState() SessionState

	SetState(SessionState) error

	GetCreated() time.Time

	GetExpires() time.Time

	GetReason() string

	GetInvited() []string

	GetLastActive() time.Time

	GetHostname() string

	GetAddress() string

	GetClustername() string

	GetLogin() string

	GetParticipants() []Participant

	AddParticipant(Participant)

	RemoveParticipant(string) error
}

func NewSession(spec SessionSpecV3) (Session, error) {
	session := &SessionV3{Spec: spec}

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
	panic("unimplemented")
}

func (s *SessionV3) GetID() string {
	panic("unimplemented")
}

func (s *SessionV3) GetNamespace() string {
	panic("unimplemented")
}

func (s *SessionV3) GetType() SessionType {
	panic("unimplemented")
}

func (s *SessionV3) GetState() SessionState {
	panic("unimplemented")
}

func (s *SessionV3) SetState(state SessionState) error {
	panic("unimplemented")
}

func (s *SessionV3) GetCreated() time.Time {
	panic("unimplemented")
}

func (s *SessionV3) GetExpires() time.Time {
	panic("unimplemented")
}

func (s *SessionV3) GetReason() string {
	panic("unimplemented")
}

func (s *SessionV3) GetInvited() []string {
	panic("unimplemented")
}

func (s *SessionV3) GetLastActive() time.Time {
	panic("unimplemented")
}

func (s *SessionV3) GetHostname() string {
	panic("unimplemented")
}

func (s *SessionV3) GetAddress() string {
	panic("unimplemented")
}

func (s *SessionV3) GetClustername() string {
	panic("unimplemented")
}

func (s *SessionV3) GetLogin() string {
	panic("unimplemented")
}

func (s *SessionV3) GetParticipants() []Participant {
	panic("unimplemented")
}

func (s *SessionV3) AddParticipant(participant Participant) {
	panic("unimplemented")
}

func (s *SessionV3) RemoveParticipant(id string) error {
	panic("unimplemented")
}
