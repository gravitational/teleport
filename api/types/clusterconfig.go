/*
Copyright 2017-2021 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api/defaults"

	"github.com/gravitational/trace"
)

// ClusterConfig defines cluster level configuration. This is a configuration
// resource, never create more than one instance of it.
type ClusterConfig interface {
	// Resource provides common resource properties.
	Resource

	// GetClusterID returns the unique cluster ID
	GetClusterID() string

	// SetClusterID sets the cluster ID
	SetClusterID(string)

	// HasAuditConfig returns true if audit configuration is set.
	// DELETE IN 8.0.0
	HasAuditConfig() bool

	// SetAuditConfig sets audit configuration.
	// DELETE IN 8.0.0
	SetAuditConfig(ClusterAuditConfig) error

	// HasNetworkingFields returns true if embedded networking configuration is set.
	// DELETE IN 8.0.0
	HasNetworkingFields() bool

	// SetNetworkingFields sets embedded networking configuration.
	// DELETE IN 8.0.0
	SetNetworkingFields(ClusterNetworkingConfig) error

	// HasSessionRecordingFields returns true if embedded session recording
	// configuration is set.
	// DELETE IN 8.0.0
	HasSessionRecordingFields() bool

	// SetSessionRecordingFields sets embedded session recording configuration.
	// DELETE IN 8.0.0
	SetSessionRecordingFields(SessionRecordingConfig) error

	// HasAuthFields returns true if legacy auth fields are set.
	// DELETE IN 8.0.0
	HasAuthFields() bool

	// SetAuthFields sets legacy auth fields.
	// DELETE IN 8.0.0
	SetAuthFields(AuthPreference) error

	// ClearLegacyFields clears embedded legacy fields.
	// DELETE IN 8.0.0
	ClearLegacyFields()

	// Copy creates a copy of the resource and returns it.
	Copy() ClusterConfig
}

// NewClusterConfig is a convenience wrapper to create a ClusterConfig resource.
func NewClusterConfig(spec ClusterConfigSpecV4) (ClusterConfig, error) {
	cc := ClusterConfigV4{
		Kind:    KindClusterConfig,
		Version: V4,
		Metadata: Metadata{
			Name:      MetaNameClusterConfig,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
	if err := cc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &cc, nil
}

// GetVersion returns resource version
func (c *ClusterConfigV4) GetVersion() string {
	return c.Version
}

// GetSubKind returns resource subkind
func (c *ClusterConfigV4) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *ClusterConfigV4) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetKind returns resource kind
func (c *ClusterConfigV4) GetKind() string {
	return c.Kind
}

// GetResourceID returns resource ID
func (c *ClusterConfigV4) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *ClusterConfigV4) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetName returns the name of the cluster.
func (c *ClusterConfigV4) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the cluster.
func (c *ClusterConfigV4) SetName(e string) {
	c.Metadata.Name = e
}

// Expiry returns object expiry setting
func (c *ClusterConfigV4) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (c *ClusterConfigV4) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (c *ClusterConfigV4) SetTTL(clock Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (c *ClusterConfigV4) GetMetadata() Metadata {
	return c.Metadata
}

// GetClusterID returns the unique cluster ID
func (c *ClusterConfigV4) GetClusterID() string {
	return c.Spec.ClusterID
}

// SetClusterID sets the cluster ID
func (c *ClusterConfigV4) SetClusterID(id string) {
	c.Spec.ClusterID = id
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *ClusterConfigV4) CheckAndSetDefaults() error {
	// make sure we have defaults for all metadata fields
	err := c.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	if c.Version == "" {
		c.Version = V4
	}
	return nil
}

// HasAuditConfig returns true if audit configuration is set.
// DELETE IN 8.0.0
func (c *ClusterConfigV4) HasAuditConfig() bool {
	return c.Spec.Audit != nil
}

// SetAuditConfig sets audit configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV4) SetAuditConfig(auditConfig ClusterAuditConfig) error {
	auditConfigV2, ok := auditConfig.(*ClusterAuditConfigV2)
	if !ok {
		return trace.BadParameter("unexpected type %T", auditConfig)
	}
	c.Spec.Audit = &auditConfigV2.Spec
	return nil
}

// HasNetworkingFields returns true if embedded networking configuration is set.
// DELETE IN 8.0.0
func (c *ClusterConfigV4) HasNetworkingFields() bool {
	return c.Spec.ClusterNetworkingConfigSpecV3 != nil
}

// SetNetworkingFields sets embedded networking configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV4) SetNetworkingFields(netConfig ClusterNetworkingConfig) error {
	netConfigV3, ok := netConfig.(*ClusterNetworkingConfigV3)
	if !ok {
		return trace.BadParameter("unexpected type %T", netConfig)
	}
	c.Spec.ClusterNetworkingConfigSpecV3 = &netConfigV3.Spec
	return nil
}

// HasSessionRecordingFields returns true if embedded session recording
// configuration is set.
// DELETE IN 8.0.0
func (c *ClusterConfigV4) HasSessionRecordingFields() bool {
	return c.Spec.LegacySessionRecordingConfigSpec != nil
}

// SetSessionRecordingFields sets embedded session recording configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV4) SetSessionRecordingFields(recConfig SessionRecordingConfig) error {
	recConfigV2, ok := recConfig.(*SessionRecordingConfigV2)
	if !ok {
		return trace.BadParameter("unexpected type %T", recConfig)
	}
	proxyChecksHostKeys := "no"
	if recConfigV2.Spec.ProxyChecksHostKeys.Value {
		proxyChecksHostKeys = "yes"
	}
	c.Spec.LegacySessionRecordingConfigSpec = &LegacySessionRecordingConfigSpec{
		Mode:                recConfigV2.Spec.Mode,
		ProxyChecksHostKeys: proxyChecksHostKeys,
	}
	return nil
}

// HasAuthFields returns true if legacy auth fields are set.
// DELETE IN 8.0.0
func (c *ClusterConfigV4) HasAuthFields() bool {
	return c.Spec.LegacyClusterConfigAuthFields != nil
}

// SetAuthFields sets legacy auth fields.
// DELETE IN 8.0.0
func (c *ClusterConfigV4) SetAuthFields(authPref AuthPreference) error {
	authPrefV2, ok := authPref.(*AuthPreferenceV2)
	if !ok {
		return trace.BadParameter("unexpected type %T", authPref)
	}
	c.Spec.LegacyClusterConfigAuthFields = &LegacyClusterConfigAuthFields{
		AllowLocalAuth:        NewBool(authPrefV2.Spec.AllowLocalAuth.Value),
		DisconnectExpiredCert: NewBool(authPrefV2.Spec.DisconnectExpiredCert.Value),
	}
	return nil
}

// ClearLegacyFields clears legacy fields.
// DELETE IN 8.0.0
func (c *ClusterConfigV4) ClearLegacyFields() {
	c.Spec.Audit = nil
	c.Spec.ClusterNetworkingConfigSpecV3 = nil
	c.Spec.LegacySessionRecordingConfigSpec = nil
	c.Spec.LegacyClusterConfigAuthFields = nil
}

// Copy creates a copy of the resource and returns it.
func (c *ClusterConfigV4) Copy() ClusterConfig {
	out := *c
	return &out
}

// String represents a human readable version of the cluster name.
func (c *ClusterConfigV4) String() string {
	return fmt.Sprintf("ClusterConfig(ClusterID=%v)", c.Spec.ClusterID)
}
