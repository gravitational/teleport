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
	"strings"
	"time"

	"github.com/gravitational/teleport/api/utils"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
)

// Lock configures locking out of a particular access vector.
type Lock interface {
	Resource

	// Target returns the lock's target.
	Target() LockTarget
	// SetTarget sets the lock's target.
	SetTarget(LockTarget)

	// Message returns the message displayed to locked-out users.
	Message() string
	// SetMessage sets the lock's user message.
	SetMessage(string)

	// LockExpiry returns when the lock ceases to be in force.
	LockExpiry() *time.Time
	// SetLockExpiry sets the lock's expiry.
	SetLockExpiry(*time.Time)

	// IsInForce returns whether the lock is in force at a particular time.
	IsInForce(time.Time) bool
}

// NewLock is a convenience method to create a Lock resource.
func NewLock(name string, spec LockSpecV2) (Lock, error) {
	lock := &LockV2{
		Metadata: Metadata{
			Name: name,
		},
		Spec: spec,
	}
	if err := lock.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return lock, nil
}

// GetVersion returns resource version.
func (c *LockV2) GetVersion() string {
	return c.Version
}

// GetName returns the name of the resource.
func (c *LockV2) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the resource.
func (c *LockV2) SetName(e string) {
	c.Metadata.Name = e
}

// SetExpiry sets expiry time for the object.
func (c *LockV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting.
func (c *LockV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetMetadata returns object metadata.
func (c *LockV2) GetMetadata() Metadata {
	return c.Metadata
}

// GetResourceID returns resource ID.
func (c *LockV2) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID.
func (c *LockV2) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetKind returns resource kind.
func (c *LockV2) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource subkind.
func (c *LockV2) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind.
func (c *LockV2) SetSubKind(sk string) {
	c.SubKind = sk
}

// Target returns the lock's target.
func (c *LockV2) Target() LockTarget {
	return c.Spec.Target
}

// SetTarget sets the lock's target.
func (c *LockV2) SetTarget(target LockTarget) {
	c.Spec.Target = target
}

// Message returns the message displayed to locked-out users.
func (c *LockV2) Message() string {
	return c.Spec.Message
}

// SetMessage sets the lock's user message.
func (c *LockV2) SetMessage(message string) {
	c.Spec.Message = message
}

// LockExpiry returns when the lock ceases to be in force.
func (c *LockV2) LockExpiry() *time.Time {
	return c.Spec.Expires
}

// SetLockExpiry sets the lock's expiry.
func (c *LockV2) SetLockExpiry(expiry *time.Time) {
	c.Spec.Expires = expiry
}

// IsInForce returns whether the lock is in force at a particular time.
func (c *LockV2) IsInForce(t time.Time) bool {
	if c.Spec.Expires == nil {
		return true
	}
	return t.Before(*c.Spec.Expires)
}

// setStaticFields sets static resource header and metadata fields.
func (c *LockV2) setStaticFields() {
	c.Kind = KindLock
	c.Version = V2
}

// CheckAndSetDefaults verifies the constraints for Lock.
func (c *LockV2) CheckAndSetDefaults() error {
	c.setStaticFields()
	err := c.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	if c.Spec.Target.IsEmpty() {
		return trace.BadParameter("at least one target field must be set")
	}
	return nil
}

// IsEmpty returns true if none of the target's fields is set.
func (t LockTarget) IsEmpty() bool {
	return cmp.Equal(t, LockTarget{})
}

// IntoMap returns the target attributes in the form of a map.
func (t LockTarget) IntoMap() (map[string]string, error) {
	m := map[string]string{}
	if err := utils.ObjectToStruct(t, &m); err != nil {
		return nil, trace.Wrap(err)
	}
	return m, nil
}

// FromMap copies values from a map into this LockTarget.
func (t *LockTarget) FromMap(m map[string]string) error {
	return trace.Wrap(utils.ObjectToStruct(m, t))
}

// Match returns true if the lock's target is matched by this target.
func (t LockTarget) Match(lock Lock) bool {
	if t.IsEmpty() {
		return false
	}
	lockTarget := lock.Target()
	if t.User != "" && lockTarget.User != t.User {
		return false
	}
	if t.Role != "" && lockTarget.Role != t.Role {
		return false
	}
	if t.Login != "" && lockTarget.Login != t.Login {
		return false
	}
	if t.Node != "" && lockTarget.Node != t.Node {
		return false
	}
	if t.MFADevice != "" && lockTarget.MFADevice != t.MFADevice {
		return false
	}
	if t.WindowsDesktop != "" && lockTarget.WindowsDesktop != t.WindowsDesktop {
		return false
	}
	if t.AccessRequest != "" && lockTarget.AccessRequest != t.AccessRequest {
		return false
	}
	return true
}

// String returns string representation of the LockTarget.
func (t LockTarget) String() string {
	return strings.TrimSpace(proto.CompactTextString(&t))
}

// Equals returns true when the two lock targets are equal.
func (t LockTarget) Equals(t2 LockTarget) bool {
	return proto.Equal(&t, &t2)
}
