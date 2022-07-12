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

package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// MarshalConfig specifies marshaling options
type MarshalConfig struct {
	// Version specifies particular version we should marshal resources with
	Version string

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
		return types.V2
	}
	return m.Version
}

// MarshalOption sets marshaling option
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

// AddOptions adds marshal options and returns a new copy
func AddOptions(opts []MarshalOption, add ...MarshalOption) []MarshalOption {
	out := make([]MarshalOption, len(opts), len(opts)+len(add))
	copy(out, opts)
	return append(opts, add...)
}

// WithResourceID assigns ID to the resource
func WithResourceID(id int64) MarshalOption {
	return func(c *MarshalConfig) error {
		c.ID = id
		return nil
	}
}

// WithExpires assigns expiry value
func WithExpires(expires time.Time) MarshalOption {
	return func(c *MarshalConfig) error {
		c.Expires = expires
		return nil
	}
}

// WithVersion sets marshal version
func WithVersion(v string) MarshalOption {
	return func(c *MarshalConfig) error {
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
func PreserveResourceID() MarshalOption {
	return func(c *MarshalConfig) error {
		c.PreserveResourceID = true
		return nil
	}
}

// ParseShortcut parses resource shortcut
func ParseShortcut(in string) (string, error) {
	if in == "" {
		return "", trace.BadParameter("missing resource name")
	}
	switch strings.ToLower(in) {
	case types.KindRole, "roles":
		return types.KindRole, nil
	case types.KindNamespace, "namespaces", "ns":
		return types.KindNamespace, nil
	case types.KindAuthServer, "auth_servers", "auth":
		return types.KindAuthServer, nil
	case types.KindProxy, "proxies":
		return types.KindProxy, nil
	case types.KindNode, "nodes":
		return types.KindNode, nil
	case types.KindOIDCConnector:
		return types.KindOIDCConnector, nil
	case types.KindSAMLConnector:
		return types.KindSAMLConnector, nil
	case types.KindGithubConnector:
		return types.KindGithubConnector, nil
	case types.KindConnectors, "connector":
		return types.KindConnectors, nil
	case types.KindUser, "users":
		return types.KindUser, nil
	case types.KindCertAuthority, "cert_authorities", "cas":
		return types.KindCertAuthority, nil
	case types.KindReverseTunnel, "reverse_tunnels", "rts":
		return types.KindReverseTunnel, nil
	case types.KindTrustedCluster, "tc", "cluster", "clusters":
		return types.KindTrustedCluster, nil
	case types.KindClusterAuthPreference, "cluster_authentication_preferences", "cap":
		return types.KindClusterAuthPreference, nil
	case types.KindClusterNetworkingConfig, "networking_config", "networking", "net_config", "netconfig":
		return types.KindClusterNetworkingConfig, nil
	case types.KindSessionRecordingConfig, "recording_config", "session_recording", "rec_config", "recconfig":
		return types.KindSessionRecordingConfig, nil
	case types.KindRemoteCluster, "remote_clusters", "rc", "rcs":
		return types.KindRemoteCluster, nil
	case types.KindSemaphore, "semaphores", "sem", "sems":
		return types.KindSemaphore, nil
	case types.KindKubeService, "kube_services":
		return types.KindKubeService, nil
	case types.KindLock, "locks":
		return types.KindLock, nil
	case types.KindDatabaseServer:
		return types.KindDatabaseServer, nil
	case types.KindNetworkRestrictions:
		return types.KindNetworkRestrictions, nil
	case types.KindDatabase:
		return types.KindDatabase, nil
	case types.KindApp, "apps":
		return types.KindApp, nil
	case types.KindWindowsDesktopService, "windows_service", "win_desktop_service", "win_service":
		return types.KindWindowsDesktopService, nil
	case types.KindWindowsDesktop, "win_desktop":
		return types.KindWindowsDesktop, nil
	case types.KindToken, "tokens":
		return types.KindToken, nil
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
		if r.Name == "" {
			return r.Kind
		}
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

// ResourceMarshaler handles marshaling of a specific resource type.
type ResourceMarshaler func(types.Resource, ...MarshalOption) ([]byte, error)

// ResourceUnmarshaler handles unmarshaling of a specific resource type.
type ResourceUnmarshaler func([]byte, ...MarshalOption) (types.Resource, error)

// resourceMarshalers holds a collection of marshalers organized by kind.
var resourceMarshalers = make(map[string]ResourceMarshaler)

// resourceUnmarshalers holds a collection of unmarshalers organized by kind.
var resourceUnmarshalers = make(map[string]ResourceUnmarshaler)

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
	RegisterResourceMarshaler(types.KindUser, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		user, ok := resource.(types.User)
		if !ok {
			return nil, trace.BadParameter("expected User, got %T", resource)
		}
		bytes, err := MarshalUser(user, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindUser, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		user, err := UnmarshalUser(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return user, nil
	})

	RegisterResourceMarshaler(types.KindCertAuthority, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		certAuthority, ok := resource.(types.CertAuthority)
		if !ok {
			return nil, trace.BadParameter("expected CertAuthority, got %T", resource)
		}
		bytes, err := MarshalCertAuthority(certAuthority, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindCertAuthority, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		certAuthority, err := UnmarshalCertAuthority(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return certAuthority, nil
	})

	RegisterResourceMarshaler(types.KindTrustedCluster, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		trustedCluster, ok := resource.(types.TrustedCluster)
		if !ok {
			return nil, trace.BadParameter("expected TrustedCluster, got %T", resource)
		}
		bytes, err := MarshalTrustedCluster(trustedCluster, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindTrustedCluster, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		trustedCluster, err := UnmarshalTrustedCluster(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return trustedCluster, nil
	})

	RegisterResourceMarshaler(types.KindGithubConnector, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		githubConnector, ok := resource.(types.GithubConnector)
		if !ok {
			return nil, trace.BadParameter("expected GithubConnector, got %T", resource)
		}
		bytes, err := MarshalGithubConnector(githubConnector, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindGithubConnector, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		githubConnector, err := UnmarshalGithubConnector(bytes) // XXX: Does not support marshal options.
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return githubConnector, nil
	})

	RegisterResourceMarshaler(types.KindRole, func(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
		role, ok := resource.(types.Role)
		if !ok {
			return nil, trace.BadParameter("expected Role, got %T", resource)
		}
		bytes, err := MarshalRole(role, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	})
	RegisterResourceUnmarshaler(types.KindRole, func(bytes []byte, opts ...MarshalOption) (types.Resource, error) {
		role, err := UnmarshalRole(bytes, opts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return role, nil
	})
}

// MarshalResource attempts to marshal a resource dynamically, returning NotImplementedError
// if no marshaler has been registered.
//
// NOTE: This function only supports the subset of resources which may be imported/exported
// by users (e.g. via `tctl get`).
func MarshalResource(resource types.Resource, opts ...MarshalOption) ([]byte, error) {
	if err := resource.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

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
func UnmarshalResource(kind string, raw []byte, opts ...MarshalOption) (types.Resource, error) {
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
	types.ResourceHeader
	// Raw is raw representation of the resource
	Raw []byte
}

// UnmarshalJSON unmarshals header and captures raw state
func (u *UnknownResource) UnmarshalJSON(raw []byte) error {
	var h types.ResourceHeader
	if err := json.Unmarshal(raw, &h); err != nil {
		return trace.Wrap(err)
	}
	u.Raw = make([]byte, len(raw))
	u.ResourceHeader = h
	copy(u.Raw, raw)
	return nil
}
