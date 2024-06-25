/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"fmt"
	"time"

	"github.com/gravitational/trace"
)

// Version defines teleport version information.
type Version interface {
	Resource

	// GetTeleportVersion returns teleport version.
	GetTeleportVersion() string
	// SetTeleportVersion sets teleport version.
	SetTeleportVersion(string)
}

// NewVersion is a convenience method to create VersionV3.
func NewVersion(value string) (Version, error) {
	v := &VersionV3{
		Metadata: Metadata{
			Name: "teleport_version",
		},
		Spec: VersionSpecV3{
			Version: value,
		},
	}
	if err := v.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return v, nil
}

// VersionV3 represents Version resource version V3.
type VersionV3 struct {
	// Kind is a resource kind - always resource.
	Kind string `json:"kind"`

	// SubKind is a resource sub kind
	SubKind string `json:"sub_kind,omitempty"`

	// Version is a resource version.
	Version string `json:"version"`

	// Metadata is metadata about the resource.
	Metadata Metadata `json:"metadata"`

	// Spec is the specification of the resource.
	Spec VersionSpecV3 `json:"spec"`
}

// GetVersion returns resource version.
func (v *VersionV3) GetVersion() string {
	return v.Version
}

// GetSubKind returns resource sub kind.
func (v *VersionV3) GetSubKind() string {
	return v.SubKind
}

// SetSubKind sets resource sub kind.
func (v *VersionV3) SetSubKind(s string) {
	v.SubKind = s
}

// GetKind returns resource kind.
func (v *VersionV3) GetKind() string {
	return v.Kind
}

// GetRevision returns the revision.
func (v *VersionV3) GetRevision() string {
	return v.Metadata.GetRevision()
}

// SetRevision sets the revision.
func (v *VersionV3) SetRevision(rev string) {
	v.Metadata.SetRevision(rev)
}

// GetName returns the name of the resource.
func (v *VersionV3) GetName() string {
	return v.Metadata.Name
}

// SetLabels sets metadata labels.
func (v *VersionV3) SetLabels(labels map[string]string) {
	v.Metadata.Labels = labels
}

// GetLabels returns metadata labels.
func (v *VersionV3) GetLabels() map[string]string {
	return v.Metadata.Labels
}

// SetName sets the name of the resource.
func (v *VersionV3) SetName(name string) {
	v.Metadata.Name = name
}

// Expiry returns object expiry setting
func (v *VersionV3) Expiry() time.Time {
	return v.Metadata.Expiry()
}

// SetExpiry sets object expiry.
func (v *VersionV3) SetExpiry(t time.Time) {
	v.Metadata.SetExpiry(t)
}

// GetMetadata returns object metadata.
func (v *VersionV3) GetMetadata() Metadata {
	return v.Metadata
}

// GetTeleportVersion returns teleport version.
func (v *VersionV3) GetTeleportVersion() string {
	return v.Spec.Version
}

// SetTeleportVersion sets teleport version.
func (v *VersionV3) SetTeleportVersion(version string) {
	v.Spec.Version = version
}

// String represents a human-readable version of version.
func (v *VersionV3) String() string {
	expires := "never"
	if !v.Expiry().IsZero() {
		expires = fmt.Sprintf("at %v", v.Expiry().String())
	}
	return fmt.Sprintf("VersionV3(version=%v, expires %v)", v.GetTeleportVersion(), expires)
}

// CheckAndSetDefaults checks and sets defaults.
func (v *VersionV3) CheckAndSetDefaults() error {
	v.Kind = KindVersion
	v.Version = V1

	if err := v.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// VersionSpecV3 is the actual data we care about for VersionV3.
type VersionSpecV3 struct {
	// Version is the teleport version.
	Version string `json:"version"`
}
