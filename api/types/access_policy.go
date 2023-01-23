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

package types

import (
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/trace"
)

// AccessPolicyV1 is a predicate policy used for RBAC, similar to rule but uses predicate language.
type AccessPolicy interface {
	// Resource provides common resource properties
	Resource
	// GetAllow returns a list of allow expressions grouped by scope.
	GetAllow() map[string]string
	// GetDeny returns a list of deny expressions grouped by scope.
	GetDeny() map[string]string
}

// AccessPolicyOption retrieves and attempts to deserialize it into the provided type.
func AccessPolicyOption[T FromRawOption[T]](policy AccessPolicy) (T, error) {
	var opt T
	raw, ok := policy.(*AccessPolicyV1).Spec.Options[opt.Name()]
	if !ok {
		return opt, trace.NotFound("option %q not found", opt.Name())
	}

	if err := opt.deserializeInto(raw); err != nil {
		return opt, trace.Wrap(err)
	}

	return opt, nil
}

// NewAccessPolicy creates a new access policy from a specification.
func NewAccessPolicy(name string, spec AccessPolicySpecV1) *AccessPolicyV1 {
	return &AccessPolicyV1{
		Kind:    KindAccessPolicy,
		Version: V1,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// GetVersion returns resource version
func (c *AccessPolicyV1) GetVersion() string {
	return c.Version
}

// GetKind returns resource kind
func (c *AccessPolicyV1) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource sub kind
func (c *AccessPolicyV1) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *AccessPolicyV1) SetSubKind(s string) {
	c.SubKind = s
}

// GetResourceID returns resource ID
func (c *AccessPolicyV1) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *AccessPolicyV1) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// setStaticFields sets static resource header and metadata fields.
func (c *AccessPolicyV1) setStaticFields() {
	c.Kind = KindAccessPolicy
	c.Version = V1
}

// CheckAndSetDefaults checks and sets default values
func (c *AccessPolicyV1) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetMetadata returns object metadata
func (c *AccessPolicyV1) GetMetadata() Metadata {
	return c.Metadata
}

// SetMetadata sets remote cluster metatada
func (c *AccessPolicyV1) SetMetadata(meta Metadata) {
	c.Metadata = meta
}

// SetExpiry sets expiry time for the object
func (c *AccessPolicyV1) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (c *AccessPolicyV1) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetName returns the name of the RemoteCluster.
func (c *AccessPolicyV1) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the RemoteCluster.
func (c *AccessPolicyV1) SetName(e string) {
	c.Metadata.Name = e
}

// GetAllow returns a list of allow expressions grouped by scope.
func (c *AccessPolicyV1) GetAllow() map[string]string {
	return c.Spec.Allow
}

// GetDeny returns a list of deny expressions grouped by scope.
func (c *AccessPolicyV1) GetDeny() map[string]string {
	return c.Spec.Deny
}
