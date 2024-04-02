/*
Copyright 2023 Gravitational, Inc.

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

package header

import (
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/utils"
)

func ResourceHeaderFromMetadata(metadata Metadata) ResourceHeader {
	return ResourceHeader{
		Metadata: metadata,
	}
}

// ResourceHeader is a common header for Teleport resources.
type ResourceHeader struct {
	// Kind is a resource kind.
	Kind string `json:"kind,omitempty"`
	// SubKind is an optional resource sub kind, used in some resources.
	SubKind string `json:"sub_kind,omitempty"`
	// Version is the resource version.
	Version string `json:"version,omitempty"`
	// Metadata is metadata for the resource.
	Metadata Metadata `json:"metadata,omitempty"`
}

// GetVersion returns the resource version.
func (h *ResourceHeader) GetVersion() string {
	return h.Version
}

// SetVersion sets the resource version.
func (h *ResourceHeader) SetVersion(version string) {
	h.Version = version
}

// GetResourceID returns the resource ID.
func (h *ResourceHeader) GetResourceID() int64 {
	return h.Metadata.GetID()
}

// SetResourceID sets the resource ID.
func (h *ResourceHeader) SetResourceID(id int64) {
	h.Metadata.SetID(id)
}

// GetRevision returns the revision.
func (h *ResourceHeader) GetRevision() string {
	return h.Metadata.GetRevision()
}

// SetRevision sets the revision.
func (h *ResourceHeader) SetRevision(rev string) {
	h.Metadata.SetRevision(rev)
}

// GetName returns the name of the resource.
func (h *ResourceHeader) GetName() string {
	return h.Metadata.GetName()
}

// SetName sets the name of the resource.
func (h *ResourceHeader) SetName(v string) {
	h.Metadata.SetName(v)
}

// Expiry returns the resource expiry setting.
func (h *ResourceHeader) Expiry() time.Time {
	return h.Metadata.Expiry()
}

// SetExpiry sets the resource expiry.
func (h *ResourceHeader) SetExpiry(t time.Time) {
	h.Metadata.SetExpiry(t)
}

// GetMetadata returns object metadata.
func (h *ResourceHeader) GetMetadata() Metadata {
	return h.Metadata
}

// GetKind returns the resource kind.
func (h *ResourceHeader) GetKind() string {
	return h.Kind
}

// SetKind sets the resource kind.
func (h *ResourceHeader) SetKind(kind string) {
	h.Kind = kind
}

// GetSubKind returns the resource subkind.
func (h *ResourceHeader) GetSubKind() string {
	return h.SubKind
}

// SetSubKind sets the resource subkind.
func (h *ResourceHeader) SetSubKind(s string) {
	h.SubKind = s
}

// Origin returns the origin value of the resource.
func (h *ResourceHeader) Origin() string {
	return h.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (h *ResourceHeader) SetOrigin(origin string) {
	h.Metadata.SetOrigin(origin)
}

// GetStaticLabels returns the static labels for the resource.
func (h *ResourceHeader) GetStaticLabels() map[string]string {
	return h.Metadata.GetStaticLabels()
}

// SetStaticLabels sets the static labels for the resource.
func (h *ResourceHeader) SetStaticLabels(sl map[string]string) {
	h.Metadata.SetStaticLabels(sl)
}

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (h *ResourceHeader) GetLabel(key string) (string, bool) {
	return h.Metadata.GetLabel(key)
}

// GetAllLabels returns all labels from the resource.
func (h *ResourceHeader) GetAllLabels() map[string]string {
	return h.Metadata.GetAllLabels()
}

// CheckAndSetDefaults will verify that the resource header is valid. This will additionally
// verify that the metadata is valid.
func (h *ResourceHeader) CheckAndSetDefaults() error {
	if h.Kind == "" {
		return trace.BadParameter("resource has an empty Kind field")
	}
	if h.Version == "" {
		return trace.BadParameter("resource has an empty Version field")
	}
	return trace.Wrap(h.Metadata.CheckAndSetDefaults())
}

// IsEqual determines if two resource headers are equivalent to one another.
func (h *ResourceHeader) IsEqual(i *ResourceHeader) bool {
	return deriveTeleportEqualResourceHeader(h, i)
}

// Metadata is resource metadata
type Metadata struct {
	// Name is an object name
	Name string `json:"name" yaml:"name"`
	// Description is object description
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Labels is a set of labels
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	// Expires is a global expiry time header can be set on any resource in the system.
	Expires time.Time `json:"expires" yaml:"expires"`
	// ID is a record ID
	// Deprecated: Use revision instead.
	ID int64 `json:"id,omitempty" yaml:"id,omitempty"`
	// Revision is an opaque identifier which tracks the versions of a resource
	// over time. Clients should ignore and not alter its value but must return
	// the revision in any updates of a resource.
	Revision string `json:"revision,omitempty" yaml:"revision,omitempty"`
}

// GetID returns the resource ID.
// Deprecated: Use GetRevision instead
func (m *Metadata) GetID() int64 {
	return m.ID
}

// SetID sets the resource ID.
// Deprecated: Use SetRevision instead
func (m *Metadata) SetID(id int64) {
	m.ID = id
}

// GetRevision returns the revision
func (m *Metadata) GetRevision() string {
	return m.Revision
}

// SetRevision sets the revision
func (m *Metadata) SetRevision(rev string) {
	m.Revision = rev
}

// GetName returns the name of the resource.
func (m *Metadata) GetName() string {
	return m.Name
}

// SetName sets the name of the resource.
func (m *Metadata) SetName(name string) {
	m.Name = name
}

// SetExpiry sets the expiry time for the object.
func (m *Metadata) SetExpiry(expires time.Time) {
	m.Expires = expires
}

// Expiry returns the object expiry setting.
func (m *Metadata) Expiry() time.Time {
	return m.Expires
}

// Origin returns the origin value of the resource.
func (m *Metadata) Origin() string {
	if m.Labels == nil {
		return ""
	}
	return m.Labels[common.OriginLabel]
}

// SetOrigin sets the origin value of the resource.
func (m *Metadata) SetOrigin(origin string) {
	if m.Labels == nil {
		m.Labels = map[string]string{}
	}
	m.Labels[common.OriginLabel] = origin
}

// CheckAndSetDefaults verifies that the metadata object is valid.
func (m *Metadata) CheckAndSetDefaults() error {
	if m.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}

	// adjust expires time to UTC if it's set
	if !m.Expires.IsZero() {
		utils.UTC(&m.Expires)
	}

	for key := range m.Labels {
		if !common.IsValidLabelKey(key) {
			return trace.BadParameter("invalid label key: %q", key)
		}
	}

	// Check the origin value.
	if m.Origin() != "" {
		if !slices.Contains(common.OriginValues, m.Origin()) {
			return trace.BadParameter("invalid origin value %q, must be one of %v", m.Origin(), common.OriginValues)
		}
	}

	return nil
}

// GetStaticLabels returns the static labels for the metadata.
func (m *Metadata) GetStaticLabels() map[string]string {
	return m.Labels
}

// SetStaticLabels sets the static labels for the metadata.
func (m *Metadata) SetStaticLabels(sl map[string]string) {
	m.Labels = sl
}

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (m *Metadata) GetLabel(key string) (string, bool) {
	v, ok := m.Labels[key]
	return v, ok
}

// GetAllLabels returns all labels from the resource.
func (m *Metadata) GetAllLabels() map[string]string {
	return m.Labels
}

// IsEqual determines if two metadata resources are equivalent to one another.
func (m *Metadata) IsEqual(i *Metadata) bool {
	return deriveTeleportEqualMetadata(m, i)
}
