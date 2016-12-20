package services

import (
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

	// V1 is our current version
	V1 = "v1"
)

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
