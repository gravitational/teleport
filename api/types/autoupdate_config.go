/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package types

import (
	"time"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"
)

// ClusterAutoUpdateConfig defines configuration of auto updates for tools and agents
type ClusterAutoUpdateConfig interface {
	// Resource provides common resource properties.
	Resource
	// SetToolsAutoUpdate enables/disables tools autoupdate in the cluster.
	SetToolsAutoUpdate(bool)
	// GetToolsAutoUpdate gets feature flag if autoupdate is enabled in the cluster.
	GetToolsAutoUpdate() bool
	// Clone performs a deep copy.
	Clone() ClusterAutoUpdateConfig
}

// NewClusterAutoUpdateConfig is a convenience wrapper to create a ClusterAutoupdateConfigV1 resource.
func NewClusterAutoUpdateConfig(spec ClusterAutoUpdateConfigSpecV1) (ClusterAutoUpdateConfig, error) {
	resource := &ClusterAutoUpdateConfigV1{Spec: spec}
	if err := resource.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return resource, nil
}

// GetVersion returns resource version
func (c *ClusterAutoUpdateConfigV1) GetVersion() string {
	return c.Version
}

// GetKind returns resource kind
func (c *ClusterAutoUpdateConfigV1) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource sub kind
func (c *ClusterAutoUpdateConfigV1) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *ClusterAutoUpdateConfigV1) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetRevision returns the revision
func (c *ClusterAutoUpdateConfigV1) GetRevision() string {
	return c.Metadata.GetRevision()
}

// SetRevision sets the revision
func (c *ClusterAutoUpdateConfigV1) SetRevision(rev string) {
	c.Metadata.SetRevision(rev)
}

// GetName returns the name of the cluster autoupdate config.
func (c *ClusterAutoUpdateConfigV1) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the cluster autoupdate config.
func (c *ClusterAutoUpdateConfigV1) SetName(e string) {
	c.Metadata.Name = e
}

// Expiry returns object expiry setting
func (c *ClusterAutoUpdateConfigV1) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (c *ClusterAutoUpdateConfigV1) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// GetMetadata returns object metadata
func (c *ClusterAutoUpdateConfigV1) GetMetadata() Metadata {
	return c.Metadata
}

// SetToolsAutoUpdate enables/disables tools autoupdate in the cluster.
func (c *ClusterAutoUpdateConfigV1) SetToolsAutoUpdate(flag bool) {
	c.Spec.ToolsAutoUpdate = flag
}

// GetToolsAutoUpdate gets feature flag if autoupdate is enabled in the cluster.
func (c *ClusterAutoUpdateConfigV1) GetToolsAutoUpdate() bool {
	return c.Spec.ToolsAutoUpdate
}

// Clone performs a deep copy.
func (c *ClusterAutoUpdateConfigV1) Clone() ClusterAutoUpdateConfig {
	return utils.CloneProtoMsg(c)
}

// setStaticFields sets static resource header and metadata fields.
func (c *ClusterAutoUpdateConfigV1) setStaticFields() {
	c.Kind = KindClusterAutoUpdateConfig
	c.Version = V1
	c.Metadata.Name = MetaNameClusterAutoUpdateConfig
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *ClusterAutoUpdateConfigV1) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
