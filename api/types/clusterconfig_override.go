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
	"fmt"
	"time"

	"github.com/gravitational/trace"
)

// ClusterConfigOverride provides overrides for chosen fields of ClusterConfig.
type ClusterConfigOverride interface {
	// Resource provides common resource properties.
	Resource

	// GetSessionRecording gets the override for session_recording.
	GetSessionRecording() string

	// CheckAndSetDefaults checks and set default values for missing fields.
	CheckAndSetDefaults() error
}

// NewClusterConfigOverride is a convenience wrapper to create a ClusterConfigOverride resource.
func NewClusterConfigOverride(spec ClusterConfigOverrideSpecV3) (ClusterConfigOverride, error) {
	override := ClusterConfigOverrideV3{
		Kind:    KindClusterConfigOverride,
		Version: V3,
		Metadata: Metadata{
			Name: MetaNameClusterConfigOverride,
		},
		Spec: spec,
	}
	if err := override.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &override, nil
}

// GetVersion returns resource version
func (c *ClusterConfigOverrideV3) GetVersion() string {
	return c.Version
}

// GetSubKind returns resource subkind
func (c *ClusterConfigOverrideV3) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *ClusterConfigOverrideV3) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetKind returns resource kind
func (c *ClusterConfigOverrideV3) GetKind() string {
	return c.Kind
}

// GetResourceID returns resource ID
func (c *ClusterConfigOverrideV3) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *ClusterConfigOverrideV3) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetName returns the name of the cluster.
func (c *ClusterConfigOverrideV3) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the cluster.
func (c *ClusterConfigOverrideV3) SetName(e string) {
	c.Metadata.Name = e
}

// Expiry returns object expiry setting
func (c *ClusterConfigOverrideV3) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (c *ClusterConfigOverrideV3) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (c *ClusterConfigOverrideV3) SetTTL(clock Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (c *ClusterConfigOverrideV3) GetMetadata() Metadata {
	return c.Metadata
}

// GetSessionRecording gets the override for session_recording.
func (c *ClusterConfigOverrideV3) GetSessionRecording() string {
	return c.Spec.SessionRecording
}

// String represents a human readable version of the cluster config override.
func (c *ClusterConfigOverrideV3) String() string {
	return fmt.Sprintf("ClusterConfigOverride(SessionRecording=%v)", c.Spec.SessionRecording)
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *ClusterConfigOverrideV3) CheckAndSetDefaults() error {
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
