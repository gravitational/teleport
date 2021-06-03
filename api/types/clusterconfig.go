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
	"github.com/gravitational/teleport/api/utils"

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

	// GetAuditConfig returns audit settings
	GetAuditConfig() AuditConfig

	// SetAuditConfig sets audit config
	SetAuditConfig(AuditConfig)

	// GetDisconnectExpiredCert returns disconnect expired certificate setting
	GetDisconnectExpiredCert() bool

	// SetDisconnectExpiredCert sets disconnect client with expired certificate setting
	SetDisconnectExpiredCert(bool)

	// GetLocalAuth gets if local authentication is allowed.
	GetLocalAuth() bool

	// SetLocalAuth sets if local authentication is allowed.
	SetLocalAuth(bool)

	// HasNetworkingConfig returns true if embedded networking configuration is set.
	// DELETE IN 8.0.0
	HasNetworkingConfig() bool

	// GetNetworkingConfig returns embedded networking configuration.
	// DELETE IN 8.0.0
	GetNetworkingConfig() (ClusterNetworkingConfig, error)

	// SetNetworkingConfig sets embedded networking configuration.
	// DELETE IN 8.0.0
	SetNetworkingConfig(ClusterNetworkingConfig) error

	// HasSessionRecordingConfig returns true if embedded session recording
	// configuration is set.
	// DELETE IN 8.0.0
	HasSessionRecordingConfig() bool

	// GetSessionRecordingConfig returns embedded session recording configuration.
	// DELETE IN 8.0.0
	GetSessionRecordingConfig() (SessionRecordingConfig, error)

	// SetSessionRecordingConfig sets embedded session recording configuration.
	// DELETE IN 8.0.0
	SetSessionRecordingConfig(SessionRecordingConfig) error

	// Copy creates a copy of the resource and returns it.
	Copy() ClusterConfig
}

// NewClusterConfig is a convenience wrapper to create a ClusterConfig resource.
func NewClusterConfig(spec ClusterConfigSpecV3) (ClusterConfig, error) {
	cc := ClusterConfigV3{
		Kind:    KindClusterConfig,
		Version: V3,
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
func (c *ClusterConfigV3) GetVersion() string {
	return c.Version
}

// GetSubKind returns resource subkind
func (c *ClusterConfigV3) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *ClusterConfigV3) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetKind returns resource kind
func (c *ClusterConfigV3) GetKind() string {
	return c.Kind
}

// GetResourceID returns resource ID
func (c *ClusterConfigV3) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *ClusterConfigV3) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetName returns the name of the cluster.
func (c *ClusterConfigV3) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the cluster.
func (c *ClusterConfigV3) SetName(e string) {
	c.Metadata.Name = e
}

// Expiry returns object expiry setting
func (c *ClusterConfigV3) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (c *ClusterConfigV3) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (c *ClusterConfigV3) SetTTL(clock Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (c *ClusterConfigV3) GetMetadata() Metadata {
	return c.Metadata
}

// GetClusterID returns the unique cluster ID
func (c *ClusterConfigV3) GetClusterID() string {
	return c.Spec.ClusterID
}

// SetClusterID sets the cluster ID
func (c *ClusterConfigV3) SetClusterID(id string) {
	c.Spec.ClusterID = id
}

// GetAuditConfig returns audit settings
func (c *ClusterConfigV3) GetAuditConfig() AuditConfig {
	return c.Spec.Audit
}

// SetAuditConfig sets audit config
func (c *ClusterConfigV3) SetAuditConfig(cfg AuditConfig) {
	c.Spec.Audit = cfg
}

// GetDisconnectExpiredCert returns disconnect expired certificate setting
func (c *ClusterConfigV3) GetDisconnectExpiredCert() bool {
	return c.Spec.DisconnectExpiredCert.Value()
}

// SetDisconnectExpiredCert sets disconnect client with expired certificate setting
func (c *ClusterConfigV3) SetDisconnectExpiredCert(b bool) {
	c.Spec.DisconnectExpiredCert = NewBool(b)
}

// GetLocalAuth gets if local authentication is allowed.
func (c *ClusterConfigV3) GetLocalAuth() bool {
	return c.Spec.LocalAuth.Value()
}

// SetLocalAuth gets if local authentication is allowed.
func (c *ClusterConfigV3) SetLocalAuth(b bool) {
	c.Spec.LocalAuth = NewBool(b)
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *ClusterConfigV3) CheckAndSetDefaults() error {
	// make sure we have defaults for all metadata fields
	err := c.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	if c.Version == "" {
		c.Version = V3
	}
	return nil
}

// HasNetworkingConfig returns true if embedded networking configuration is set.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) HasNetworkingConfig() bool {
	return c.Spec.ClusterNetworkingConfigSpecV2 != nil
}

// GetNetworkingConfig returns embedded networking configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) GetNetworkingConfig() (ClusterNetworkingConfig, error) {
	if c.Spec.ClusterNetworkingConfigSpecV2 == nil {
		return nil, trace.BadParameter("ClusterNetworkingConfigSpec is not set")
	}
	return NewClusterNetworkingConfig(*c.Spec.ClusterNetworkingConfigSpecV2)
}

// SetNetworkingConfig sets embedded networking configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) SetNetworkingConfig(netConfig ClusterNetworkingConfig) error {
	netConfigV2, ok := netConfig.(*ClusterNetworkingConfigV2)
	if !ok {
		return trace.BadParameter("unexpected type %T", netConfig)
	}
	c.Spec.ClusterNetworkingConfigSpecV2 = &netConfigV2.Spec
	return nil
}

// HasSessionRecordingConfig returns true if embedded session recording
// configuration is set.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) HasSessionRecordingConfig() bool {
	return c.Spec.LegacySessionRecordingConfigSpec != nil
}

// GetSessionRecordingConfig returns embedded session recording configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) GetSessionRecordingConfig() (SessionRecordingConfig, error) {
	if c.Spec.LegacySessionRecordingConfigSpec == nil {
		return nil, trace.BadParameter("LegacySessionRecordingConfigSpec is not set")
	}
	recordingSpec := SessionRecordingConfigSpecV2{
		Mode: c.Spec.LegacySessionRecordingConfigSpec.Mode,
	}
	b, err := utils.ParseBool(c.Spec.LegacySessionRecordingConfigSpec.ProxyChecksHostKeys)
	if err != nil {
		recordingSpec.ProxyChecksHostKeys = NewBoolOption(b)
	}
	return NewSessionRecordingConfig(recordingSpec)
}

// SetSessionRecordingConfig sets embedded session recording configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) SetSessionRecordingConfig(recConfig SessionRecordingConfig) error {
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

// Copy creates a copy of the resource and returns it.
func (c *ClusterConfigV3) Copy() ClusterConfig {
	out := *c
	return &out
}

// String represents a human readable version of the cluster name.
func (c *ClusterConfigV3) String() string {
	return fmt.Sprintf("ClusterConfig(ClusterID=%v)", c.Spec.ClusterID)
}
