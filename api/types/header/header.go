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
	"regexp"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/header/v1"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/utils"
)

// FromResourceHeaderV1 converts the resource header protobuf message into an internal resource header object.
// This function does not use the builder due to the generics for the builder object.
func FromResourceHeaderV1(msg *headerv1.ResourceHeader) *ResourceHeader {
	return &ResourceHeader{
		Kind:     msg.Kind,
		SubKind:  msg.SubKind,
		Version:  msg.Version,
		Metadata: FromMetadataV1(msg.Metadata),
	}
}

func ResourceHeaderFromMetadata(metadata *Metadata) *ResourceHeader {
	return &ResourceHeader{
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
	Metadata *Metadata `json:"metadata,omitempty"`
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
func (h *ResourceHeader) GetMetadata() *Metadata {
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
func (h *ResourceHeader) GetLabel(key string) (value string, ok bool) {
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

// FromMetadataV1 converts v1 metadata into an internal metadata object.
func FromMetadataV1(msg *headerv1.Metadata) *Metadata {
	return &Metadata{
		Name:        msg.Name,
		Description: msg.Description,
		Labels:      msg.Labels,
		Expires:     msg.Expires.AsTime(),
	}
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
	ID int64 `json:"id,omitempty" yaml:"id,omitempty"`
}

// GetID returns the resource ID.
func (m *Metadata) GetID() int64 {
	return m.ID
}

// SetID sets the resource ID.
func (m *Metadata) SetID(id int64) {
	m.ID = id
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
		if !IsValidLabelKey(key) {
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
func (m *Metadata) GetLabel(key string) (value string, ok bool) {
	v, ok := m.Labels[key]
	return v, ok
}

// GetAllLabels returns all labels from the resource.
func (m *Metadata) GetAllLabels() map[string]string {
	return m.Labels
}

// LabelPattern is a regexp that describes a valid label key
const LabelPattern = `^[a-zA-Z/.0-9_:*-]+$`

var validLabelKey = regexp.MustCompile(LabelPattern)

// IsValidLabelKey checks if the supplied string matches the
// label key regexp.
func IsValidLabelKey(s string) bool {
	return validLabelKey.MatchString(s)
}
