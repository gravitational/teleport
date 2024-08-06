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

	"github.com/gravitational/teleport/api/utils"
)

// ClusterAuditConfig defines cluster-wide audit log configuration. This is
// a configuration resource, never create more than one instance of it.
type ClusterAuditConfig interface {
	Resource

	// Type gets the audit backend type.
	Type() string
	// SetType sets the audit backend type.
	SetType(string)

	// Region gets a cloud provider region.
	Region() string
	// SetRegion sets a cloud provider region.
	SetRegion(string)

	// ShouldUploadSessions returns whether audit config
	// instructs server to upload sessions.
	ShouldUploadSessions() bool

	// AuditSessionsURI gets the audit sessions URI.
	AuditSessionsURI() string
	// SetAuditSessionsURI sets the audit sessions URI.
	SetAuditSessionsURI(string)

	// AuditEventsURIs gets the audit events URIs.
	AuditEventsURIs() []string
	// SetAuditEventsURIs sets the audit events URIs.
	SetAuditEventsURIs([]string)

	// SetUseFIPSEndpoint sets the FIPS endpoint state for S3/Dynamo backends.
	SetUseFIPSEndpoint(state ClusterAuditConfigSpecV2_FIPSEndpointState)
	// GetUseFIPSEndpoint gets the current FIPS endpoint setting
	GetUseFIPSEndpoint() ClusterAuditConfigSpecV2_FIPSEndpointState

	// EnableContinuousBackups is used to enable (or disable) PITR (Point-In-Time Recovery).
	EnableContinuousBackups() bool
	// EnableAutoScaling is used to enable (or disable) auto scaling policy.
	EnableAutoScaling() bool
	// ReadMaxCapacity is the maximum provisioned read capacity.
	ReadMaxCapacity() int64
	// ReadMinCapacity is the minimum provisioned read capacity.
	ReadMinCapacity() int64
	// ReadTargetValue is the ratio of consumed read to provisioned capacity.
	ReadTargetValue() float64
	// WriteMaxCapacity is the maximum provisioned write capacity.
	WriteMaxCapacity() int64
	// WriteMinCapacity is the minimum provisioned write capacity.
	WriteMinCapacity() int64
	// WriteTargetValue is the ratio of consumed write to provisioned capacity.
	WriteTargetValue() float64
	// RetentionPeriod is the retention period for audit events.
	RetentionPeriod() *Duration
	// Clone performs a deep copy.
	Clone() ClusterAuditConfig
}

// NewClusterAuditConfig is a convenience method to to create ClusterAuditConfigV2.
func NewClusterAuditConfig(spec ClusterAuditConfigSpecV2) (ClusterAuditConfig, error) {
	auditConfig := &ClusterAuditConfigV2{Spec: spec}
	if err := auditConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return auditConfig, nil
}

// DefaultClusterAuditConfig returns the default audit log configuration.
func DefaultClusterAuditConfig() ClusterAuditConfig {
	config, _ := NewClusterAuditConfig(ClusterAuditConfigSpecV2{})
	return config
}

// GetVersion returns resource version.
func (c *ClusterAuditConfigV2) GetVersion() string {
	return c.Version
}

// GetName returns the name of the resource.
func (c *ClusterAuditConfigV2) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the resource.
func (c *ClusterAuditConfigV2) SetName(e string) {
	c.Metadata.Name = e
}

// SetExpiry sets expiry time for the object.
func (c *ClusterAuditConfigV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting.
func (c *ClusterAuditConfigV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetMetadata returns object metadata.
func (c *ClusterAuditConfigV2) GetMetadata() Metadata {
	return c.Metadata
}

// GetRevision returns the revision
func (c *ClusterAuditConfigV2) GetRevision() string {
	return c.Metadata.GetRevision()
}

// SetRevision sets the revision
func (c *ClusterAuditConfigV2) SetRevision(rev string) {
	c.Metadata.SetRevision(rev)
}

// GetKind returns resource kind.
func (c *ClusterAuditConfigV2) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource subkind.
func (c *ClusterAuditConfigV2) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind.
func (c *ClusterAuditConfigV2) SetSubKind(sk string) {
	c.SubKind = sk
}

// Type gets the audit backend type.
func (c *ClusterAuditConfigV2) Type() string {
	return c.Spec.Type
}

// SetType sets the audit backend type.
func (c *ClusterAuditConfigV2) SetType(backendType string) {
	c.Spec.Type = backendType
}

// Region gets a cloud provider region.
func (c *ClusterAuditConfigV2) Region() string {
	return c.Spec.Region
}

// SetRegion sets a cloud provider region.
func (c *ClusterAuditConfigV2) SetRegion(region string) {
	c.Spec.Region = region
}

// ShouldUploadSessions returns whether audit config
// instructs server to upload sessions.
func (c *ClusterAuditConfigV2) ShouldUploadSessions() bool {
	return c.Spec.AuditSessionsURI != ""
}

// AuditSessionsURI gets the audit sessions URI.
func (c *ClusterAuditConfigV2) AuditSessionsURI() string {
	return c.Spec.AuditSessionsURI
}

// SetAuditSessionsURI sets the audit sessions URI.
func (c *ClusterAuditConfigV2) SetAuditSessionsURI(uri string) {
	c.Spec.AuditSessionsURI = uri
}

// AuditEventsURIs gets the audit events URIs.
func (c *ClusterAuditConfigV2) AuditEventsURIs() []string {
	return c.Spec.AuditEventsURI
}

// SetAuditEventsURIs sets the audit events URIs.
func (c *ClusterAuditConfigV2) SetAuditEventsURIs(uris []string) {
	c.Spec.AuditEventsURI = uris
}

// SetUseFIPSEndpoint sets the FIPS endpoint state for S3/Dynamo backends.
func (c *ClusterAuditConfigV2) SetUseFIPSEndpoint(state ClusterAuditConfigSpecV2_FIPSEndpointState) {
	c.Spec.UseFIPSEndpoint = state
}

// GetUseFIPSEndpoint gets the current FIPS endpoint setting
func (c *ClusterAuditConfigV2) GetUseFIPSEndpoint() ClusterAuditConfigSpecV2_FIPSEndpointState {
	return c.Spec.UseFIPSEndpoint
}

// EnableContinuousBackups is used to enable (or disable) PITR (Point-In-Time Recovery).
func (c *ClusterAuditConfigV2) EnableContinuousBackups() bool {
	return c.Spec.EnableContinuousBackups
}

// EnableAutoScaling is used to enable (or disable) auto scaling policy.
func (c *ClusterAuditConfigV2) EnableAutoScaling() bool {
	return c.Spec.EnableAutoScaling
}

// ReadMaxCapacity is the maximum provisioned read capacity.
func (c *ClusterAuditConfigV2) ReadMaxCapacity() int64 {
	return c.Spec.ReadMaxCapacity
}

// ReadMinCapacity is the minimum provisioned read capacity.
func (c *ClusterAuditConfigV2) ReadMinCapacity() int64 {
	return c.Spec.ReadMinCapacity
}

// ReadTargetValue is the ratio of consumed read to provisioned capacity.
func (c *ClusterAuditConfigV2) ReadTargetValue() float64 {
	return c.Spec.ReadTargetValue
}

// WriteMaxCapacity is the maximum provisioned write capacity.
func (c *ClusterAuditConfigV2) WriteMaxCapacity() int64 {
	return c.Spec.WriteMaxCapacity
}

// WriteMinCapacity is the minimum provisioned write capacity.
func (c *ClusterAuditConfigV2) WriteMinCapacity() int64 {
	return c.Spec.WriteMinCapacity
}

// WriteTargetValue is the ratio of consumed write to provisioned capacity.
func (c *ClusterAuditConfigV2) WriteTargetValue() float64 {
	return c.Spec.WriteTargetValue
}

// RetentionPeriod is the retention period for audit events.
func (c *ClusterAuditConfigV2) RetentionPeriod() *Duration {
	value := c.Spec.RetentionPeriod
	return &value
}

// Clone performs a deep copy.
func (c *ClusterAuditConfigV2) Clone() ClusterAuditConfig {
	return utils.CloneProtoMsg(c)
}

// setStaticFields sets static resource header and metadata fields.
func (c *ClusterAuditConfigV2) setStaticFields() {
	c.Kind = KindClusterAuditConfig
	c.Version = V2
	c.Metadata.Name = MetaNameClusterAuditConfig
}

// CheckAndSetDefaults verifies the constraints for ClusterAuditConfig.
func (c *ClusterAuditConfigV2) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
