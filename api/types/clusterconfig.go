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

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"
)

// ClusterConfig defines cluster level configuration. This is a configuration
// resource, never create more than one instance of it.
// DELETE IN 8.0.0
type ClusterConfig interface {
	// Resource provides common resource properties.
	Resource

	// GetLegacyClusterID returns the legacy cluster ID.
	// DELETE IN 8.0.0
	GetLegacyClusterID() string

	// SetLegacyClusterID sets the legacy cluster ID.
	// DELETE IN 8.0.0
	SetLegacyClusterID(string)

	// HasAuditConfig returns true if audit configuration is set.
	// DELETE IN 8.0.0
	HasAuditConfig() bool

	// SetAuditConfig sets audit configuration.
	// DELETE IN 8.0.0
	SetAuditConfig(ClusterAuditConfig) error

	// GetClusterAuditConfig gets embedded cluster audit configuration.
	// DELETE IN 8.0.0
	GetClusterAuditConfig() (ClusterAuditConfig, error)

	// HasNetworkingFields returns true if embedded networking configuration is set.
	// DELETE IN 8.0.0
	HasNetworkingFields() bool

	// SetNetworkingFields sets embedded networking configuration.
	// DELETE IN 8.0.0
	SetNetworkingFields(ClusterNetworkingConfig) error

	// GetClusterNetworkingConfig gets embedded cluster networking configuration.
	// DELETE IN 8.0.0
	GetClusterNetworkingConfig() (ClusterNetworkingConfig, error)

	// HasSessionRecordingFields returns true if embedded session recording
	// configuration is set.
	// DELETE IN 8.0.0
	HasSessionRecordingFields() bool

	// SetSessionRecordingFields sets embedded session recording configuration.
	// DELETE IN 8.0.0
	SetSessionRecordingFields(SessionRecordingConfig) error

	// GetSessionRecordingConfig gets embedded session recording configuration.
	// DELETE IN 8.0.0
	GetSessionRecordingConfig() (SessionRecordingConfig, error)

	// HasAuthFields returns true if legacy auth fields are set.
	// DELETE IN 8.0.0
	HasAuthFields() bool

	// SetAuthFields sets legacy auth fields.
	// DELETE IN 8.0.0
	SetAuthFields(AuthPreference) error

	// Copy creates a copy of the resource and returns it.
	Copy() ClusterConfig
}

// NewClusterConfig is a convenience wrapper to create a ClusterConfig resource.
func NewClusterConfig(spec ClusterConfigSpecV3) (ClusterConfig, error) {
	cc := &ClusterConfigV3{Spec: spec}
	if err := cc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return cc, nil
}

// DefaultClusterConfig is used as the default cluster configuration when
// one is not specified (record at node).
func DefaultClusterConfig() ClusterConfig {
	config, _ := NewClusterConfig(ClusterConfigSpecV3{})
	return config
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

// GetMetadata returns object metadata
func (c *ClusterConfigV3) GetMetadata() Metadata {
	return c.Metadata
}

// setStaticFields sets static resource header and metadata fields.
func (c *ClusterConfigV3) setStaticFields() {
	c.Kind = KindClusterConfig
	c.Version = V3
	c.Metadata.Name = MetaNameClusterConfig
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *ClusterConfigV3) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetLegacyClusterID returns the legacy cluster ID.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) GetLegacyClusterID() string {
	return c.Spec.ClusterID
}

// SetLegacyClusterID sets the legacy cluster ID.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) SetLegacyClusterID(id string) {
	c.Spec.ClusterID = id
}

// HasAuditConfig returns true if audit configuration is set.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) HasAuditConfig() bool {
	return c.Spec.Audit != nil
}

// SetAuditConfig sets audit configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) SetAuditConfig(auditConfig ClusterAuditConfig) error {
	auditConfigV2, ok := auditConfig.(*ClusterAuditConfigV2)
	if !ok {
		return trace.BadParameter("unexpected type %T", auditConfig)
	}
	c.Spec.Audit = &auditConfigV2.Spec
	return nil
}

// GetClusterAuditConfig gets embedded cluster audit configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) GetClusterAuditConfig() (ClusterAuditConfig, error) {
	if !c.HasAuditConfig() {
		return nil, nil
	}
	auditConfig, err := NewClusterAuditConfig(*c.Spec.Audit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return auditConfig, nil
}

// HasNetworkingFields returns true if embedded networking configuration is set.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) HasNetworkingFields() bool {
	return c.Spec.ClusterNetworkingConfigSpecV2 != nil
}

// SetNetworkingFields sets embedded networking configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) SetNetworkingFields(netConfig ClusterNetworkingConfig) error {
	netConfigV2, ok := netConfig.(*ClusterNetworkingConfigV2)
	if !ok {
		return trace.BadParameter("unexpected type %T", netConfig)
	}
	c.Spec.ClusterNetworkingConfigSpecV2 = &netConfigV2.Spec
	return nil
}

// GetClusterNetworkingConfig gets embedded cluster networking configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) GetClusterNetworkingConfig() (ClusterNetworkingConfig, error) {
	if !c.HasNetworkingFields() {
		return nil, nil
	}
	netConfig, err := NewClusterNetworkingConfigFromConfigFile(*c.Spec.ClusterNetworkingConfigSpecV2)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return netConfig, nil
}

// HasSessionRecordingFields returns true if embedded session recording
// configuration is set.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) HasSessionRecordingFields() bool {
	return c.Spec.LegacySessionRecordingConfigSpec != nil
}

// SetSessionRecordingFields sets embedded session recording configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) SetSessionRecordingFields(recConfig SessionRecordingConfig) error {
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

// GetSessionRecordingConfig gets embedded session recording configuration.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) GetSessionRecordingConfig() (SessionRecordingConfig, error) {
	if !c.HasSessionRecordingFields() {
		return nil, nil
	}
	proxyChecksHostKeys, err := utils.ParseBool(c.Spec.LegacySessionRecordingConfigSpec.ProxyChecksHostKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	recConfigSpec := SessionRecordingConfigSpecV2{
		Mode:                c.Spec.LegacySessionRecordingConfigSpec.Mode,
		ProxyChecksHostKeys: NewBoolOption(proxyChecksHostKeys),
	}
	recConfig, err := NewSessionRecordingConfigFromConfigFile(recConfigSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return recConfig, nil
}

// HasAuthFields returns true if legacy auth fields are set.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) HasAuthFields() bool {
	return c.Spec.LegacyClusterConfigAuthFields != nil
}

// SetAuthFields sets legacy auth fields.
// DELETE IN 8.0.0
func (c *ClusterConfigV3) SetAuthFields(authPref AuthPreference) error {
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

// Copy creates a copy of the resource and returns it.
func (c *ClusterConfigV3) Copy() ClusterConfig {
	out := *c
	return &out
}

// String represents a human readable version of the cluster name.
func (c *ClusterConfigV3) String() string {
	return fmt.Sprintf("ClusterConfig(ClusterID=%v)", c.Spec.ClusterID)
}
