/*
Copyright 2020 Gravitational, Inc.

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
	"regexp"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"

	"github.com/gravitational/trace"
)

// Resource represents common properties for all resources.
type Resource interface {
	// GetKind returns resource kind
	GetKind() string
	// GetSubKind returns resource subkind
	GetSubKind() string
	// SetSubKind sets resource subkind
	SetSubKind(string)
	// GetVersion returns resource version
	GetVersion() string
	// GetName returns the name of the resource
	GetName() string
	// SetName sets the name of the resource
	SetName(string)
	// Expiry returns object expiry setting
	Expiry() time.Time
	// SetExpiry sets object expiry
	SetExpiry(time.Time)
	// GetMetadata returns object metadata
	GetMetadata() Metadata
	// GetResourceID returns resource ID
	GetResourceID() int64
	// SetResourceID sets resource ID
	SetResourceID(int64)
	// CheckAndSetDefaults validates the Resource and sets any empty fields to
	// default values.
	CheckAndSetDefaults() error
}

// ResourceWithSecrets includes additional properties which must
// be provided by resources which *may* contain secrets.
type ResourceWithSecrets interface {
	Resource
	// WithoutSecrets returns an instance of the resource which
	// has had all secrets removed.  If the current resource has
	// already had its secrets removed, this may be a no-op.
	WithoutSecrets() Resource
}

// ResourceWithOrigin provides information on the origin of the resource
// (defaults, config-file, dynamic).
type ResourceWithOrigin interface {
	Resource
	// Origin returns the origin value of the resource.
	Origin() string
	// SetOrigin sets the origin value of the resource.
	SetOrigin(string)
}

// ResourceWithLabels is a common interface for resources that have labels.
type ResourceWithLabels interface {
	// ResourceWithOrigin is the base resource interface.
	ResourceWithOrigin
	// GetAllLabels returns all resource's labels.
	GetAllLabels() map[string]string
	// MatchSearch goes through select field values of a resource
	// and tries to match against the list of search values.
	MatchSearch(searchValues []string) bool
}

// ResourcesWithLabels is a list of labeled resources.
type ResourcesWithLabels []ResourceWithLabels

// Find returns resource with the specified name or nil.
func (r ResourcesWithLabels) Find(name string) ResourceWithLabels {
	for _, resource := range r {
		if resource.GetName() == name {
			return resource
		}
	}
	return nil
}

// Len returns the slice length.
func (r ResourcesWithLabels) Len() int { return len(r) }

// Less compares resources by name.
func (r ResourcesWithLabels) Less(i, j int) bool { return r[i].GetName() < r[j].GetName() }

// Swap swaps two resources.
func (r ResourcesWithLabels) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

// GetVersion returns resource version
func (h *ResourceHeader) GetVersion() string {
	return h.Version
}

// GetResourceID returns resource ID
func (h *ResourceHeader) GetResourceID() int64 {
	return h.Metadata.ID
}

// SetResourceID sets resource ID
func (h *ResourceHeader) SetResourceID(id int64) {
	h.Metadata.ID = id
}

// GetName returns the name of the resource
func (h *ResourceHeader) GetName() string {
	return h.Metadata.Name
}

// SetName sets the name of the resource
func (h *ResourceHeader) SetName(v string) {
	h.Metadata.SetName(v)
}

// Expiry returns object expiry setting
func (h *ResourceHeader) Expiry() time.Time {
	return h.Metadata.Expiry()
}

// SetExpiry sets object expiry
func (h *ResourceHeader) SetExpiry(t time.Time) {
	h.Metadata.SetExpiry(t)
}

// GetMetadata returns object metadata
func (h *ResourceHeader) GetMetadata() Metadata {
	return h.Metadata
}

// GetKind returns resource kind
func (h *ResourceHeader) GetKind() string {
	return h.Kind
}

// GetSubKind returns resource subkind
func (h *ResourceHeader) GetSubKind() string {
	return h.SubKind
}

// SetSubKind sets resource subkind
func (h *ResourceHeader) SetSubKind(s string) {
	h.SubKind = s
}

func (h *ResourceHeader) CheckAndSetDefaults() error {
	if h.Kind == "" {
		return trace.BadParameter("resource has an empty Kind field")
	}
	if h.Version == "" {
		return trace.BadParameter("resource has an empty Version field")
	}
	return trace.Wrap(h.Metadata.CheckAndSetDefaults())
}

// GetID returns resource ID
func (m *Metadata) GetID() int64 {
	return m.ID
}

// SetID sets resource ID
func (m *Metadata) SetID(id int64) {
	m.ID = id
}

// GetMetadata returns object metadata
func (m *Metadata) GetMetadata() Metadata {
	return *m
}

// GetName returns the name of the resource
func (m *Metadata) GetName() string {
	return m.Name
}

// SetName sets the name of the resource
func (m *Metadata) SetName(name string) {
	m.Name = name
}

// SetExpiry sets expiry time for the object
func (m *Metadata) SetExpiry(expires time.Time) {
	m.Expires = &expires
}

// Expiry returns object expiry setting.
func (m *Metadata) Expiry() time.Time {
	if m.Expires == nil {
		return time.Time{}
	}
	return *m.Expires
}

// Origin returns the origin value of the resource.
func (m *Metadata) Origin() string {
	if m.Labels == nil {
		return ""
	}
	return m.Labels[OriginLabel]
}

// SetOrigin sets the origin value of the resource.
func (m *Metadata) SetOrigin(origin string) {
	if m.Labels == nil {
		m.Labels = map[string]string{}
	}
	m.Labels[OriginLabel] = origin
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (m *Metadata) CheckAndSetDefaults() error {
	if m.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	if m.Namespace == "" {
		m.Namespace = defaults.Namespace
	}

	// adjust expires time to UTC if it's set
	if m.Expires != nil {
		utils.UTC(m.Expires)
	}

	for key := range m.Labels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("invalid label key: %q", key)
		}
	}

	// Check the origin value.
	if m.Origin() != "" {
		if !utils.SliceContainsStr(OriginValues, m.Origin()) {
			return trace.BadParameter("invalid origin value %q, must be one of %v", m.Origin(), OriginValues)
		}
	}

	return nil
}

// MatchLabels takes a map of labels and returns `true` if the resource has ALL
// of them.
func MatchLabels(resource ResourceWithLabels, labels map[string]string) bool {
	resourceLabels := resource.GetAllLabels()
	for name, value := range labels {
		if resourceLabels[name] != value {
			return false
		}
	}

	return true
}

// LabelPattern is a regexp that describes a valid label key
const LabelPattern = `^[a-zA-Z/.0-9_*-]+$`

var validLabelKey = regexp.MustCompile(LabelPattern)

// IsValidLabelKey checks if the supplied string matches the
// label key regexp.
func IsValidLabelKey(s string) bool {
	return validLabelKey.MatchString(s)
}

// MatchSearch goes through select field values from a resource
// and tries to match against the list of search values, ignoring case and order.
// Returns true if all search vals were matched (or if nil search vals).
// Returns false if no or partial match (or nil field values).
func MatchSearch(fieldVals []string, searchVals []string, customMatch func(val string) bool) bool {
	// Case fold all values to avoid repeated case folding while matching.
	caseFoldedSearchVals := utils.ToLowerStrings(searchVals)
	caseFoldedFieldVals := utils.ToLowerStrings(fieldVals)

Outer:
	for _, searchV := range caseFoldedSearchVals {
		// Iterate through field values to look for a match.
		for _, fieldV := range caseFoldedFieldVals {
			if strings.Contains(fieldV, searchV) {
				continue Outer
			}
		}

		if customMatch != nil && customMatch(searchV) {
			continue
		}

		// When no fields matched a value, prematurely end if we can.
		return false
	}

	return true
}
