/*
Copyright 2015 Gravitational, Inc.

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

	// KindRole is a role resource
	KindRole = "role"

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

	// KindAuthPreference is the type of authentication for this cluster.
	KindClusterAuthPreference = "cluster_auth_preference"

	// KindAuthPreference is the type of authentication for this cluster.
	MetaNameClusterAuthPreference = "cluster-auth-preference"

	// KindClusterConfig is the resource that holds cluster level configuration.
	KindClusterConfig = "cluster_config"

	// MetaNameClusterName is the exact name of the singleton resource.
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

	// KindTunnelConection specifies connection of a reverse tunnel to proxy
	KindTunnelConnection = "tunnel_connection"

	// KindRemoteCluster represents remote cluster connected via reverse tunnel
	// to proxy
	KindRemoteCluster = "remote_cluster"

	// KindIdenity is local on disk identity resource
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

func collectOptions(opts []MarshalOption) (*MarshalConfig, error) {
	var cfg MarshalConfig
	for _, o := range opts {
		if err := o(&cfg); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &cfg, nil
}

// MarshalConfig specify marshalling options
type MarshalConfig struct {
	// Version specifies particular version we should marshal resources with
	Version string
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

// marshalerMutex is a mutex for resource marshalers/unmarshalers
var marshalerMutex sync.RWMutex

// V2SchemaTemplate is a template JSON Schema for V2 style objects
const V2SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["kind", "spec", "metadata", "version"],
  "properties": {
    "kind": {"type": "string"},
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
    "labels": {
      "type": "object",
      "patternProperties": {
         "^[a-zA-Z/.0-9_]$":  { "type": "string" }
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

// ResorceHeader is a shared resource header
type ResourceHeader struct {
	// Kind is a resource kind - always resource
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is Role metadata
	Metadata Metadata `json:"metadata"`
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

// Metadata is resource metadata
type Metadata struct {
	// Name is an object name
	Name string `json:"name"`
	// Namespace is object namespace. The field should be called "namespace"
	// when it returns in Teleport 2.4.
	Namespace string `json:"-"`
	// Description is object description
	Description string `json:"description,omitempty"`
	// Labels is a set of labels
	Labels map[string]string `json:"labels,omitempty"`
	// Expires is a global expiry time header can be set on any resource in the system.
	Expires *time.Time `json:"expires,omitempty"`
}

// Resource represents common properties for resources
type Resource interface {
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

// Expires returns object expiry setting.
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
	case "role", "roles":
		return KindRole, nil
	case "namespaces", "ns":
		return KindNamespace, nil
	case "auth_servers", "auth":
		return KindAuthServer, nil
	case "proxies":
		return KindProxy, nil
	case "nodes", "node":
		return KindNode, nil
	case "oidc":
		return KindOIDCConnector, nil
	case "saml":
		return KindSAMLConnector, nil
	case "github":
		return KindGithubConnector, nil
	case "user", "users":
		return KindUser, nil
	case "cert_authorities", "cas":
		return KindCertAuthority, nil
	case "reverse_tunnels", "rts":
		return KindReverseTunnel, nil
	case "trusted_cluster", "tc", "cluster", "clusters":
		return KindTrustedCluster, nil
	case "cluster_authentication_preferences", "cap":
		return KindClusterAuthPreference, nil
	case "remote_cluster", "remote_clusters", "rc", "rcs":
		return KindRemoteCluster, nil
	}
	return "", trace.BadParameter("unsupported resource: %v", in)
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

func (r *Ref) IsEmtpy() bool {
	return r.Name == ""
}

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
