package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

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

	// V1 is our current version
	V1 = "v1"
)

// marshalerMutex is a mutex for resource marshalers/unmarshalers
var marshalerMutex sync.RWMutex

// V1SchemaTemplate is a template JSON Schema for V1 style objects
const V1SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["kind", "spec", "metadata", "version"],
  "properties": {
    "kind": {"type": "string"},
    "version": {"type": "string", "default": "v1"},
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
	Description string `json:"description"`
	// Labels is a set of labels
	Labels map[string]string `json:"labels,omitempty"`
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
