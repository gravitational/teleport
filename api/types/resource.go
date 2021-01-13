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
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/utils"

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
	// SetTTL sets Expires header using current clock
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

// Clock is used to track TTL of resources
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

// SetTTL sets Expires header using current clock
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

// SetTTL sets Expires header using realtime clock
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

	// adjust expires time to utils.UTC if it's set
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

const labelPattern = `^[a-zA-Z/.0-9_*-]+$`

var validLabelKey = regexp.MustCompile(labelPattern)

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

// marshalerMutex is a mutex for resource marshalers/unmarshalers
var marshalerMutex sync.RWMutex

// ResourceMarshaler handles marshaling of a specific resource type.
type ResourceMarshaler func(Resource, ...MarshalOption) ([]byte, error)

// ResourceUnmarshaler handles unmarshaling of a specific resource type.
type ResourceUnmarshaler func([]byte, ...MarshalOption) (Resource, error)

// resourceMarshalers holds a collection of marshalers organized by kind.
var resourceMarshalers map[string]ResourceMarshaler = make(map[string]ResourceMarshaler)

// resourceUnmarshalers holds a collection of unmarshalers organized by kind.
var resourceUnmarshalers map[string]ResourceUnmarshaler = make(map[string]ResourceUnmarshaler)

// GetResourceMarshalerKinds lists all registered resource marshalers by kind.
func GetResourceMarshalerKinds() []string {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	kinds := make([]string, 0, len(resourceMarshalers))
	for kind := range resourceMarshalers {
		kinds = append(kinds, kind)
	}
	return kinds
}

// RegisterResourceMarshaler registers a marshaler for resources of a specific kind.
func RegisterResourceMarshaler(kind string, marshaler ResourceMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	resourceMarshalers[kind] = marshaler
}

// RegisterResourceUnmarshaler registers an unmarshaler for resources of a specific kind.
func RegisterResourceUnmarshaler(kind string, unmarshaler ResourceUnmarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	resourceUnmarshalers[kind] = unmarshaler
}

func getResourceMarshaler(kind string) (ResourceMarshaler, bool) {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	m, ok := resourceMarshalers[kind]
	if !ok {
		return nil, false
	}
	return m, true
}

func getResourceUnmarshaler(kind string) (ResourceUnmarshaler, bool) {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	u, ok := resourceUnmarshalers[kind]
	if !ok {
		return nil, false
	}
	return u, true
}

func init() {
	RegisterResourceMarshaler(KindUser, func(r Resource, opts ...MarshalOption) ([]byte, error) {
		rsc, ok := r.(User)
		if !ok {
			return nil, trace.BadParameter("expected User, got %T", r)
		}
		raw, err := GetUserMarshaler().MarshalUser(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	RegisterResourceUnmarshaler(KindUser, func(b []byte, opts ...MarshalOption) (Resource, error) {
		rsc, err := GetUserMarshaler().UnmarshalUser(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})

	RegisterResourceMarshaler(KindCertAuthority, func(r Resource, opts ...MarshalOption) ([]byte, error) {
		rsc, ok := r.(CertAuthority)
		if !ok {
			return nil, trace.BadParameter("expected CertAuthority, got %T", r)
		}
		raw, err := GetCertAuthorityMarshaler().MarshalCertAuthority(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	RegisterResourceUnmarshaler(KindCertAuthority, func(b []byte, opts ...MarshalOption) (Resource, error) {
		rsc, err := GetCertAuthorityMarshaler().UnmarshalCertAuthority(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})

	RegisterResourceMarshaler(KindTrustedCluster, func(r Resource, opts ...MarshalOption) ([]byte, error) {
		rsc, ok := r.(TrustedCluster)
		if !ok {
			return nil, trace.BadParameter("expected TrustedCluster, got %T", r)
		}
		raw, err := GetTrustedClusterMarshaler().Marshal(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	RegisterResourceUnmarshaler(KindTrustedCluster, func(b []byte, opts ...MarshalOption) (Resource, error) {
		rsc, err := GetTrustedClusterMarshaler().Unmarshal(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})

	RegisterResourceMarshaler(KindGithubConnector, func(r Resource, opts ...MarshalOption) ([]byte, error) {
		rsc, ok := r.(GithubConnector)
		if !ok {
			return nil, trace.BadParameter("expected GithubConnector, got %T", r)
		}
		raw, err := GetGithubConnectorMarshaler().Marshal(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	RegisterResourceUnmarshaler(KindGithubConnector, func(b []byte, opts ...MarshalOption) (Resource, error) {
		rsc, err := GetGithubConnectorMarshaler().Unmarshal(b) // XXX: Does not support marshal options.
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})
}

// MarshalResource attempts to marshal a resource dynamically, returning NotImplementedError
// if no marshaler has been registered.
//
// NOTE: This function only supports the subset of resources which may be imported/exported
// by users (e.g. via `tctl get`).
func MarshalResource(resource Resource, opts ...MarshalOption) ([]byte, error) {
	marshal, ok := getResourceMarshaler(resource.GetKind())
	if !ok {
		return nil, trace.NotImplemented("cannot dynamically marshal resources of kind %q", resource.GetKind())
	}
	// Handle the case where `resource` was never fully unmarshaled.
	if r, ok := resource.(*UnknownResource); ok {
		u, err := UnmarshalResource(r.GetKind(), r.Raw, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resource = u
	}
	m, err := marshal(resource, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return m, nil
}

// UnmarshalResource attempts to unmarshal a resource dynamically, returning NotImplementedError
// if no unmarshaler has been registered.
//
// NOTE: This function only supports the subset of resources which may be imported/exported
// by users (e.g. via `tctl get`).
func UnmarshalResource(kind string, raw []byte, opts ...MarshalOption) (Resource, error) {
	unmarshal, ok := getResourceUnmarshaler(kind)
	if !ok {
		return nil, trace.NotImplemented("cannot dynamically unmarshal resources of kind %q", kind)
	}
	u, err := unmarshal(raw, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

// UnknownResource is used to detect resources
type UnknownResource struct {
	ResourceHeader
	// Raw is raw representation of the resource
	Raw []byte
}

// UnmarshalJSON unmarshals header and captures raw state
func (u *UnknownResource) UnmarshalJSON(raw []byte) error {
	var h ResourceHeader
	if err := json.Unmarshal(raw, &h); err != nil {
		return trace.Wrap(err)
	}
	u.Raw = make([]byte, len(raw))
	u.ResourceHeader = h
	copy(u.Raw, raw)
	return nil
}

const baseMetadataSchema = `{
  "type": "object",
  "additionalProperties": false,
  "default": {},
  "required": ["name"],
  "properties": {
	"name": {"type": "string"},
	"namespace": {"type": "string", "default": "default"},
	"description": {"type": "string"},
	"expires": {"type": "string"},
	"id": {"type": "integer"},
	"labels": {
	  "type": "object",
	  "additionalProperties": false,
	  "patternProperties": {
  	    "%s":  { "type": "string" }
  	  }
    }
  }
}`

// V2SchemaTemplate is a template JSON Schema for V2 style objects
const V2SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["kind", "spec", "metadata", "version"],
  "properties": {
    "kind": {"type": "string"},
    "sub_kind": {"type": "string"},
    "version": {"type": "string", "default": "v2"},
    "metadata": %v,
    "spec": %v
  }%v
}`

// MetadataSchema is a schema for resource metadata
var MetadataSchema = fmt.Sprintf(baseMetadataSchema, labelPattern)

// DefaultDefinitions the default list of JSON schema definitions which is none.
const DefaultDefinitions = ``
