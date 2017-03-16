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

	"github.com/gravitational/trace"
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

	// KindOIDC is oidc connector resource
	KindOIDC = "oidc"

	// KindOIDCReques is oidc auth request resource
	KindOIDCRequest = "oidc_request"

	// KindSession is a recorded session resource
	KindSession = "session"

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

	// KindAuthPreference is the type of authentication for this cluster.
	KindClusterAuthPreference = "cluster_auth_preference"

	// KindAuthPreference is the type of authentication for this cluster.
	MetaNameClusterAuthPreference = "cluster-auth-preference"

	// KindUniversalSecondFactor is a type of second factor authentication.
	KindUniversalSecondFactor = "universal_second_factor"

	// MetaNameUniversalSecondFactor is a type of second factor authentication.
	MetaNameUniversalSecondFactor = "universal-second-factor"

	// KindTrustedCluster is a resource that contains trusted cluster configuration.
	KindTrustedCluster = "trusted_cluster"

	// V2 is our current version
	V2 = "v2"

	// V1 is our first version
	// resources were not explicitly versioned at that point
	V1 = "v1"
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
  }
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
	// Namespace is object namespace
	Namespace string `json:"namespace"`
	// Description is object description
	Description string `json:"description,omitempty"`
	// Labels is a set of labels
	Labels map[string]string `json:"labels,omitempty"`
	// Expires is a global expiry time header
	// can be set on any resource in the system
	Expires time.Time `json:"expires,omitempty"`
}

// Check checks validity of all parameters and sets defaults
func (m *Metadata) Check() error {
	if m.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	return nil
}

// ParseShortcut parses resource shortcut
func ParseShortcut(in string) (string, error) {
	if in == "" {
		return "", trace.BadParameter("missing resource name")
	}
	switch strings.ToLower(in) {
	case "roles":
		return KindRole, nil
	case "namespaces", "ns":
		return KindNamespace, nil
	case "auth_servers", "auth":
		return KindAuthServer, nil
	case "proxies":
		return KindProxy, nil
	case "nodes":
		return KindNode, nil
	case "oidc":
		return KindOIDCConnector, nil
	case "users":
		return KindUser, nil
	case "cert_authorities", "cas":
		return KindCertAuthority, nil
	case "reverse_tunnels", "rts":
		return KindReverseTunnel, nil
	case "trusted_cluster", "tc":
		return KindTrustedCluster, nil
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

// Ref is a resource refernece
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
	return fmt.Sprintf("%v/%v", r.Kind, r.Name)
}
