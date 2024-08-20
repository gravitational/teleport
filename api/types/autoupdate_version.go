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
	"github.com/gravitational/teleport/api/utils"
	"time"

	"github.com/gravitational/trace"
)

// AutoUpdateVersion defines resource for storing semantic version of auto updates.
type AutoUpdateVersion interface {
	// Resource provides common resource properties.
	Resource
	// SetToolsVersion defines required version for tools autoupdate.
	SetToolsVersion(string)
	// GetToolsVersion gets last known required version for autoupdate.
	GetToolsVersion() string
	// Clone performs a deep copy.
	Clone() AutoUpdateVersion
}

// NewAutoUpdateVersion is a convenience wrapper to create a AutoupdateVersionV1 resource.
func NewAutoUpdateVersion(spec AutoupdateVersionSpecV1) (AutoUpdateVersion, error) {
	resource := &AutoUpdateVersionV1{Spec: spec}
	if err := resource.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return resource, nil
}

// GetVersion returns resource version
func (c *AutoUpdateVersionV1) GetVersion() string {
	return c.Version
}

// GetKind returns resource kind
func (c *AutoUpdateVersionV1) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource sub kind
func (c *AutoUpdateVersionV1) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *AutoUpdateVersionV1) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetRevision returns the revision
func (c *AutoUpdateVersionV1) GetRevision() string {
	return c.Metadata.GetRevision()
}

// SetRevision sets the revision
func (c *AutoUpdateVersionV1) SetRevision(rev string) {
	c.Metadata.SetRevision(rev)
}

// GetName returns the name of the autoupdate version.
func (c *AutoUpdateVersionV1) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the autoupdate version.
func (c *AutoUpdateVersionV1) SetName(e string) {
	c.Metadata.Name = e
}

// Expiry returns object expiry setting
func (c *AutoUpdateVersionV1) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (c *AutoUpdateVersionV1) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// GetMetadata returns object metadata
func (c *AutoUpdateVersionV1) GetMetadata() Metadata {
	return c.Metadata
}

// SetToolsVersion defines required version for tools autoupdate.
func (c *AutoUpdateVersionV1) SetToolsVersion(version string) {
	c.Spec.ToolsVersion = version
}

// GetToolsVersion gets last known required version for autoupdate.
func (c *AutoUpdateVersionV1) GetToolsVersion() string {
	return c.Spec.ToolsVersion
}

// Clone performs a deep copy.
func (c *AutoUpdateVersionV1) Clone() AutoUpdateVersion {
	return utils.CloneProtoMsg(c)
}

// setStaticFields sets static resource header and metadata fields.
func (c *AutoUpdateVersionV1) setStaticFields() {
	c.Kind = KindAutoUpdateVersion
	c.Version = V1
	c.Metadata.Name = MetaNameAutoUpdateVersion
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *AutoUpdateVersionV1) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if c.Spec.ToolsVersion == "" {
		return trace.BadParameter("missing ToolsVersion field")
	}

	return nil
}
