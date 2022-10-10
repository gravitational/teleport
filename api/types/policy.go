/*
Copyright 2020 Gravitational, Inc.

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

type Policy interface {
	// Resource provides common resource properties
	Resource
	GetAllow() string
	GetDeny() string
	GetOptions() string
}

// GetVersion returns resource version
func (c *PolicyV1) GetVersion() string {
	return c.Version
}

// GetKind returns resource kind
func (c *PolicyV1) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource sub kind
func (c *PolicyV1) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *PolicyV1) SetSubKind(s string) {
	c.SubKind = s
}

// GetResourceID returns resource ID
func (c *PolicyV1) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *PolicyV1) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// setStaticFields sets static resource header and metadata fields.
func (c *PolicyV1) setStaticFields() {
	c.Kind = KindRemoteCluster
	c.Version = V3
}

// CheckAndSetDefaults checks and sets default values
func (c *PolicyV1) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetMetadata returns object metadata
func (c *PolicyV1) GetMetadata() Metadata {
	return c.Metadata
}

// SetMetadata sets remote cluster metatada
func (c *PolicyV1) SetMetadata(meta Metadata) {
	c.Metadata = meta
}

// SetExpiry sets expiry time for the object
func (c *PolicyV1) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (c *PolicyV1) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetName returns the name of the RemoteCluster.
func (c *PolicyV1) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the RemoteCluster.
func (c *PolicyV1) SetName(e string) {
	c.Metadata.Name = e
}

func (c *PolicyV1) GetAllow() string {
	return c.Spec.Allow
}

func (c *PolicyV1) GetDeny() string {
	return c.Spec.Deny
}

func (c *PolicyV1) GetOptions() string {
	return c.Spec.Options
}
