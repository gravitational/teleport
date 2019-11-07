/*
Copyright 2015-2019 Gravitational, Inc.

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

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

const (
	// DefaultAPIGroup is a default group of permissions API,
	// lets us to add different permission types
	DefaultAPIGroup = "gravitational.io/teleport"

	// ActionRead grants read access (get, list)
	ActionRead = "read"

	// ActionWrite allows to write (create, update, delete)
	ActionWrite = "write"

	// Wildcard is a special wildcard character matching everything
	Wildcard = "*"

	// KindNamespace is a namespace
	KindNamespace = "namespace"

	// KindUser is a user resource
	KindUser = "user"

	// KindKeyPair is a public/private key pair
	KindKeyPair = "key_pair"

	// KindHostCert is a host certificate
	KindHostCert = "host_cert"

	// KindLicense is a license resource
	KindLicense = "license"

	// KindRole is a role resource
	KindRole = "role"

	// KindAccessRequest is an AccessReqeust resource
	KindAccessRequest = "access_request"

	// KindOIDC is OIDC connector resource
	KindOIDC = "oidc"

	// KindSAML is SAML connector resource
	KindSAML = "saml"

	// KindGithub is Github connector resource
	KindGithub = "github"

	// KindOIDCRequest is OIDC auth request resource
	KindOIDCRequest = "oidc_request"

	// KindSAMLRequest is SAML auth request resource
	KindSAMLRequest = "saml_request"

	// KindGithubRequest is Github auth request resource
	KindGithubRequest = "github_request"

	// KindSession is a recorded SSH session.
	KindSession = "session"

	// KindSSHSession is an active SSH session.
	KindSSHSession = "ssh_session"

	// KindWebSession is a web session resource
	KindWebSession = "web_session"

	// KindEvent is structured audit logging event
	KindEvent = "event"

	// KindAuthServer is auth server resource
	KindAuthServer = "auth_server"

	// KindProxy is proxy resource
	KindProxy = "proxy"

	// KindNode is node resource
	KindNode = "node"

	// KindToken is a provisioning token resource
	KindToken = "token"

	// KindCertAuthority is a certificate authority resource
	KindCertAuthority = "cert_authority"

	// KindReverseTunnel is a reverse tunnel connection
	KindReverseTunnel = "tunnel"

	// KindOIDCConnector is a OIDC connector resource
	KindOIDCConnector = "oidc"

	// KindSAMLConnector is a SAML connector resource
	KindSAMLConnector = "saml"

	// KindGithubConnector is Github OAuth2 connector resource
	KindGithubConnector = "github"

	// KindConnectors is a shortcut for all authentication connector types.
	KindConnectors = "connectors"

	// KindClusterAuthPreference is the type of authentication for this cluster.
	KindClusterAuthPreference = "cluster_auth_preference"

	// MetaNameClusterAuthPreference is the type of authentication for this cluster.
	MetaNameClusterAuthPreference = "cluster-auth-preference"

	// KindClusterConfig is the resource that holds cluster level configuration.
	KindClusterConfig = "cluster_config"

	// MetaNameClusterConfig is the exact name of the cluster config singleton resource.
	MetaNameClusterConfig = "cluster-config"

	// KindClusterName is a type of configuration resource that contains the cluster name.
	KindClusterName = "cluster_name"

	// MetaNameClusterName is the name of a configuration resource for cluster name.
	MetaNameClusterName = "cluster-name"

	// KindStaticTokens is a type of configuration resource that contains static tokens.
	KindStaticTokens = "static_tokens"

	// MetaNameStaticTokens is the name of a configuration resource for static tokens.
	MetaNameStaticTokens = "static-tokens"

	// KindTrustedCluster is a resource that contains trusted cluster configuration.
	KindTrustedCluster = "trusted_cluster"

	// KindAuthConnector allows access to OIDC and SAML connectors.
	KindAuthConnector = "auth_connector"

	// KindTunnelConnection specifies connection of a reverse tunnel to proxy
	KindTunnelConnection = "tunnel_connection"

	// KindRemoteCluster represents remote cluster connected via reverse tunnel
	// to proxy
	KindRemoteCluster = "remote_cluster"

	// KindInviteToken is a local user invite token
	KindInviteToken = "invite_token"

	// KindIdentity is local on disk identity resource
	KindIdentity = "identity"

	// KindState is local on disk process state
	KindState = "state"

	// V3 is the third version of resources.
	V3 = "v3"

	// V2 is the second version of resources.
	V2 = "v2"

	// V1 is the first version of resources. Note: The first version was
	// not explicitly versioned.
	V1 = "v1"
)

const (
	// VerbList is used to list all objects. Does not imply the ability to read a single object.
	VerbList = "list"

	// VerbCreate is used to create an object.
	VerbCreate = "create"

	// VerbRead is used to read a single object.
	VerbRead = "read"

	// VerbReadNoSecrets is used to read a single object without secrets.
	VerbReadNoSecrets = "readnosecrets"

	// VerbUpdate is used to update an object.
	VerbUpdate = "update"

	// VerbDelete is used to remove an object.
	VerbDelete = "delete"

	// VerbRotate is used to rotate certificate authorities
	// used only internally
	VerbRotate = "rotate"
)

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

func collectOptions(opts []MarshalOption) (*MarshalConfig, error) {
	var cfg MarshalConfig
	for _, o := range opts {
		if err := o(&cfg); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &cfg, nil
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

// MarshalOption sets marshalling option
type MarshalOption func(c *MarshalConfig) error

// WithVersion sets marshal version
func WithVersion(v string) MarshalOption {
	return func(c *MarshalConfig) error {
		switch v {
		case V1, V2:
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

// SkipValidation is used to disable schema validation.
func SkipValidation() MarshalOption {
	return func(c *MarshalConfig) error {
		c.SkipValidation = true
		return nil
	}
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
	for kind, _ := range resourceMarshalers {
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
// if not unmarshaler has been registered.
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
const MetadataSchema = `{
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
         "^[a-zA-Z/.0-9_*-]+$":  { "type": "string" }
      }
    }
  }
}`

// DefaultDefinitions the default list of JSON schema definitions which is none.
const DefaultDefinitions = ``

// UnknownResource is used to detect resources
type UnknownResource struct {
	ResourceHeader
	// Raw is raw representation of the resource
	Raw []byte
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
func (h *ResourceHeader) SetTTL(clock clockwork.Clock, ttl time.Duration) {
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

// Resource represents common properties for resources
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
	SetTTL(clock clockwork.Clock, ttl time.Duration)
	// GetMetadata returns object metadata
	GetMetadata() Metadata
	// GetResourceID returns resource ID
	GetResourceID() int64
	// SetResourceID sets resource ID
	SetResourceID(int64)
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
func (m *Metadata) SetTTL(clock clockwork.Clock, ttl time.Duration) {
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

	// adjust expires time to utc if it's set
	if m.Expires != nil {
		utils.UTC(m.Expires)
	}

	return nil
}

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

// Ref is a resource reference
type Ref struct {
	Kind string
	Name string
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
	return fmt.Sprintf("%s/%s", r.Kind, r.Name)
}

// Refs is a set of resource references
type Refs []Ref

// ParseRefs parses a comma-separated string of resource references (eg "users/alice,users/bob")
func ParseRefs(refs string) (Refs, error) {
	if refs == "all" {
		return []Ref{Ref{Kind: "all"}}, nil
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

// Check if refs is special wildcard case `all`.
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
