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

package resource

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"

	"github.com/gravitational/trace"
)

// ParseShortcut parses resource shortcut
func ParseShortcut(in string) (string, error) {
	if in == "" {
		return "", trace.BadParameter("missing resource name")
	}
	switch strings.ToLower(in) {
	case KindRole, "roles":
		return KindRole, nil
	case KindNamespace, "namespaces", "ns":
		return KindNamespace, nil
	case KindAuthServer, "auth_servers", "auth":
		return KindAuthServer, nil
	case KindProxy, "proxies":
		return KindProxy, nil
	case KindNode, "nodes":
		return KindNode, nil
	case KindOIDCConnector:
		return KindOIDCConnector, nil
	case KindSAMLConnector:
		return KindSAMLConnector, nil
	case KindGithubConnector:
		return KindGithubConnector, nil
	case KindConnectors, "connector":
		return KindConnectors, nil
	case KindUser, "users":
		return KindUser, nil
	case KindCertAuthority, "cert_authorities", "cas":
		return KindCertAuthority, nil
	case KindReverseTunnel, "reverse_tunnels", "rts":
		return KindReverseTunnel, nil
	case KindTrustedCluster, "tc", "cluster", "clusters":
		return KindTrustedCluster, nil
	case KindClusterAuthPreference, "cluster_authentication_preferences", "cap":
		return KindClusterAuthPreference, nil
	case KindRemoteCluster, "remote_clusters", "rc", "rcs":
		return KindRemoteCluster, nil
	case KindSemaphore, "semaphores", "sem", "sems":
		return KindSemaphore, nil
	case KindKubeService, "kube_services":
		return KindKubeService, nil
	}
	return "", trace.BadParameter("unsupported resource: %q - resources should be expressed as 'type/name', for example 'connector/github'", in)
}

// ParseRef parses resource reference eg daemonsets/ds1
func ParseRef(ref string) (*Ref, error) {
	if ref == "" {
		return nil, trace.BadParameter("missing value")
	}
	parts := strings.FieldsFunc(ref, isDelimiter)
	switch len(parts) {
	case 1:
		shortcut, err := ParseShortcut(parts[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Ref{Kind: shortcut}, nil
	case 2:
		shortcut, err := ParseShortcut(parts[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Ref{Kind: shortcut, Name: parts[1]}, nil
	case 3:
		shortcut, err := ParseShortcut(parts[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Ref{Kind: shortcut, SubKind: parts[1], Name: parts[2]}, nil
	}
	return nil, trace.BadParameter("failed to parse '%v'", ref)
}

// isDelimiter returns true if rune is space or /
func isDelimiter(r rune) bool {
	switch r {
	case '\t', ' ', '/':
		return true
	}
	return false
}

// Ref is a resource reference.  Typically of the form kind/name,
// but sometimes of the form kind/subkind/name.
type Ref struct {
	Kind    string
	SubKind string
	Name    string
}

// IsEmpty checks whether the provided resource name is empty
func (r *Ref) IsEmpty() bool {
	return r.Name == ""
}

// Set sets the name of the resource
func (r *Ref) Set(v string) error {
	out, err := ParseRef(v)
	if err != nil {
		return err
	}
	*r = *out
	return nil
}

func (r *Ref) String() string {
	if r.SubKind == "" {
		return fmt.Sprintf("%s/%s", r.Kind, r.Name)
	}
	return fmt.Sprintf("%s/%s/%s", r.Kind, r.SubKind, r.Name)
}

// Refs is a set of resource references
type Refs []Ref

// ParseRefs parses a comma-separated string of resource references (eg "users/alice,users/bob")
func ParseRefs(refs string) (Refs, error) {
	if refs == "all" {
		return []Ref{{Kind: "all"}}, nil
	}
	var escaped bool
	isBreak := func(r rune) bool {
		brk := false
		switch r {
		case ',':
			brk = true && !escaped
			escaped = false
		case '\\':
			escaped = true && !escaped
		default:
			escaped = false
		}
		return brk
	}
	var parsed []Ref
	split := fieldsFunc(strings.TrimSpace(refs), isBreak)
	for _, s := range split {
		ref, err := ParseRef(strings.ReplaceAll(s, `\,`, `,`))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		parsed = append(parsed, *ref)
	}
	return parsed, nil
}

// Set sets the value of `r` from a comma-separated string of resource
// references (in-place equivalent of `ParseRefs`).
func (r *Refs) Set(v string) error {
	refs, err := ParseRefs(v)
	if err != nil {
		return trace.Wrap(err)
	}
	*r = refs
	return nil
}

// IsAll checks if refs is special wildcard case `all`.
func (r *Refs) IsAll() bool {
	refs := *r
	if len(refs) != 1 {
		return false
	}
	return refs[0].Kind == "all"
}

func (r *Refs) String() string {
	var builder strings.Builder
	for i, ref := range *r {
		if i > 0 {
			builder.WriteRune(',')
		}
		builder.WriteString(ref.String())
	}
	return builder.String()
}

// fieldsFunc is an exact copy of the current implementation of `strings.FieldsFunc`.
// The docs of `strings.FieldsFunc` indicate that future implementations may not call
// `f` on every rune, may not preserve ordering, or may panic if `f` does not return the
// same output for every instance of a given rune.  All of these changes would break
// our implementation of backslash-escaping, so we're using a local copy.
func fieldsFunc(s string, f func(rune) bool) []string {
	// A span is used to record a slice of s of the form s[start:end].
	// The start index is inclusive and the end index is exclusive.
	type span struct {
		start int
		end   int
	}
	spans := make([]span, 0, 32)

	// Find the field start and end indices.
	wasField := false
	fromIndex := 0
	for i, rune := range s {
		if f(rune) {
			if wasField {
				spans = append(spans, span{start: fromIndex, end: i})
				wasField = false
			}
		} else {
			if !wasField {
				fromIndex = i
				wasField = true
			}
		}
	}

	// Last field might end at EOF.
	if wasField {
		spans = append(spans, span{fromIndex, len(s)})
	}

	// Create strings from recorded field indices.
	a := make([]string, len(spans))
	for i, span := range spans {
		a[i] = s[span.start:span.end]
	}

	return a
}

// marshalerMutex is a mutex for resource marshalers/unmarshalers
var marshalerMutex sync.RWMutex

// Marshaler handles marshaling of a specific resource type.
type Marshaler func(Resource, ...auth.MarshalOption) ([]byte, error)

// Unmarshaler handles unmarshaling of a specific resource type.
type Unmarshaler func([]byte, ...auth.MarshalOption) (Resource, error)

// resourceMarshalers holds a collection of marshalers organized by kind.
var resourceMarshalers map[string]Marshaler = make(map[string]Marshaler)

// resourceUnmarshalers holds a collection of unmarshalers organized by kind.
var resourceUnmarshalers map[string]Unmarshaler = make(map[string]Unmarshaler)

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
func RegisterResourceMarshaler(kind string, marshaler Marshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	resourceMarshalers[kind] = marshaler
}

// RegisterResourceUnmarshaler registers an unmarshaler for resources of a specific kind.
func RegisterResourceUnmarshaler(kind string, unmarshaler Unmarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	resourceUnmarshalers[kind] = unmarshaler
}

func getResourceMarshaler(kind string) (Marshaler, bool) {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	m, ok := resourceMarshalers[kind]
	if !ok {
		return nil, false
	}
	return m, true
}

func getResourceUnmarshaler(kind string) (Unmarshaler, bool) {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	u, ok := resourceUnmarshalers[kind]
	if !ok {
		return nil, false
	}
	return u, true
}

func init() {
	RegisterResourceMarshaler(KindUser, func(r Resource, opts ...auth.MarshalOption) ([]byte, error) {
		rsc, ok := r.(User)
		if !ok {
			return nil, trace.BadParameter("expected User, got %T", r)
		}
		raw, err := MarshalUser(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	RegisterResourceUnmarshaler(KindUser, func(b []byte, opts ...auth.MarshalOption) (Resource, error) {
		rsc, err := UnmarshalUser(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})

	RegisterResourceMarshaler(KindCertAuthority, func(r Resource, opts ...auth.MarshalOption) ([]byte, error) {
		rsc, ok := r.(CertAuthority)
		if !ok {
			return nil, trace.BadParameter("expected CertAuthority, got %T", r)
		}
		raw, err := MarshalCertAuthority(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	RegisterResourceUnmarshaler(KindCertAuthority, func(b []byte, opts ...auth.MarshalOption) (Resource, error) {
		rsc, err := UnmarshalCertAuthority(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})

	RegisterResourceMarshaler(KindTrustedCluster, func(r Resource, opts ...auth.MarshalOption) ([]byte, error) {
		rsc, ok := r.(TrustedCluster)
		if !ok {
			return nil, trace.BadParameter("expected TrustedCluster, got %T", r)
		}
		raw, err := MarshalTrustedCluster(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	RegisterResourceUnmarshaler(KindTrustedCluster, func(b []byte, opts ...auth.MarshalOption) (Resource, error) {
		rsc, err := UnmarshalTrustedCluster(b, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return rsc, nil
	})

	RegisterResourceMarshaler(KindGithubConnector, func(r Resource, opts ...auth.MarshalOption) ([]byte, error) {
		rsc, ok := r.(GithubConnector)
		if !ok {
			return nil, trace.BadParameter("expected GithubConnector, got %T", r)
		}
		raw, err := MarshalGithubConnector(rsc, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return raw, nil
	})
	RegisterResourceUnmarshaler(KindGithubConnector, func(b []byte, opts ...auth.MarshalOption) (Resource, error) {
		rsc, err := UnmarshalGithubConnector(b) // XXX: Does not support marshal options.
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
func MarshalResource(resource Resource, opts ...auth.MarshalOption) ([]byte, error) {
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
func UnmarshalResource(kind string, raw []byte, opts ...auth.MarshalOption) (Resource, error) {
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
var MetadataSchema = fmt.Sprintf(baseMetadataSchema, types.LabelPattern)

// DefaultDefinitions the default list of JSON schema definitions which is none.
const DefaultDefinitions = ``

// CollectOptions collects all options from functional arg and returns config
func CollectOptions(opts []auth.MarshalOption) (*auth.MarshalConfig, error) {
	var cfg auth.MarshalConfig
	for _, o := range opts {
		if err := o(&cfg); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &cfg, nil
}

// AddOptions adds marshal options and returns a new copy
func AddOptions(opts []auth.MarshalOption, add ...auth.MarshalOption) []auth.MarshalOption {
	out := make([]auth.MarshalOption, len(opts), len(opts)+len(add))
	copy(out, opts)
	return append(opts, add...)
}

// WithResourceID assigns ID to the resource
func WithResourceID(id int64) auth.MarshalOption {
	return func(c *auth.MarshalConfig) error {
		c.ID = id
		return nil
	}
}

// WithExpires assigns expiry value
func WithExpires(expires time.Time) auth.MarshalOption {
	return func(c *auth.MarshalConfig) error {
		c.Expires = expires
		return nil
	}
}

// WithVersion sets marshal version
func WithVersion(v string) auth.MarshalOption {
	return func(c *auth.MarshalConfig) error {
		switch v {
		case types.V1, types.V2, types.V3:
			c.Version = v
			return nil
		default:
			return trace.BadParameter("version '%v' is not supported", v)
		}
	}
}

// PreserveResourceID preserves resource ID when
// marshaling value
func PreserveResourceID() auth.MarshalOption {
	return func(c *auth.MarshalConfig) error {
		c.PreserveResourceID = true
		return nil
	}
}

// SkipValidation is used to disable schema validation.
func SkipValidation() auth.MarshalOption {
	return func(c *auth.MarshalConfig) error {
		c.SkipValidation = true
		return nil
	}
}
