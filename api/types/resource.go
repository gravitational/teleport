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
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"

	"github.com/gogo/protobuf/proto"
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
	// SetTTL sets Expires header using the provided clock.
	// Use SetExpiry instead.
	// DELETE IN 7.0.0
	SetTTL(clock Clock, ttl time.Duration)
	// GetMetadata returns object metadata
	GetMetadata() Metadata
	// GetResourceID returns resource ID
	GetResourceID() int64
	// SetResourceID sets resource ID
	SetResourceID(int64)
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

// Clock is used to track TTL of resources.
// This is only used in SetTTL which is deprecated.
// DELETE IN 7.0.0
type Clock interface {
	Now() time.Time
}

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

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (h *ResourceHeader) SetTTL(clock Clock, ttl time.Duration) {
	h.Metadata.SetTTL(clock, ttl)
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

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (m *Metadata) SetTTL(clock Clock, ttl time.Duration) {
	expireTime := clock.Now().UTC().Add(ttl)
	m.Expires = &expireTime
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

	return nil
}

// Merge overwrites r from src and
// is part of support for cloning Server values
// using proto.Clone.
//
// Note: this does not implement the full Merger interface,
// specifically, it assumes that r is zero value.
// See https://github.com/gogo/protobuf/blob/v1.3.1/proto/clone.go#L58-L60
//
// Implements proto.Merger
func (m *Metadata) Merge(src proto.Message) {
	metadata, ok := src.(*Metadata)
	if !ok {
		return
	}
	*m = *metadata
	// Manually clone expiry timestamp as proto.Clone
	// cannot cope with values that contain unexported
	// attributes (as time.Time does)
	if metadata.Expires != nil {
		expires := *metadata.Expires
		m.Expires = &expires
	}
	if len(metadata.Labels) != 0 {
		m.Labels = make(map[string]string)
		for k, v := range metadata.Labels {
			m.Labels[k] = v
		}
	}
}

var validLabelKey = regexp.MustCompile(constants.LabelPattern)

// IsValidLabelKey checks if the supplied string matches the
// label key regexp.
func IsValidLabelKey(s string) bool {
	return validLabelKey.MatchString(s)
}

// MarshalConfig specifies marshalling options
type MarshalConfig struct {
	// Version specifies particular version we should marshal resources with
	Version string

	// SkipValidation is used to skip schema validation.
	SkipValidation bool

	// ID is a record ID to assign
	ID int64

	// PreserveResourceID preserves resource IDs in resource
	// specs when marshaling
	PreserveResourceID bool

	// Expires is an optional expiry time
	Expires time.Time
}

// GetVersion returns explicitly provided version or sets latest as default
func (m *MarshalConfig) GetVersion() string {
	if m.Version == "" {
		return V2
	}
	return m.Version
}

// MarshalOption sets marshalling option
type MarshalOption func(c *MarshalConfig) error

// CollectOptions collects all options from functional arg and returns config
func CollectOptions(opts []MarshalOption) (*MarshalConfig, error) {
	var cfg MarshalConfig
	for _, o := range opts {
		if err := o(&cfg); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &cfg, nil
}
