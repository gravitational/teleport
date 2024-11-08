package servicemini

import (
	"crypto/tls"
	"crypto/x509"
	"slices"
	"sync/atomic"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
)

// Connector has all resources process needs to connect to other parts of the
// cluster: client and identity.
type Connector struct {
	clusterName string
	hostID      string
	role        types.SystemRole

	// clientState contains the current connector state for outbound connections
	// to the cluster.
	clientState atomic.Pointer[connectorState]
	// serverState contains the current connector state for inbound connections
	// from the cluster.
	serverState atomic.Pointer[connectorState]

	// Client is an authenticated client intended to use the credentials in
	// clientState (unless it's a client shared from some other connector as
	// signified by ReusedClient).
	Client *authclient.Client

	// ReusedClient, if true, indicates that the client reference is owned by
	// a different connector and should not be closed.
	ReusedClient bool
}

func (c *Connector) ClusterName() string {
	return c.clusterName
}

func (c *Connector) HostID() string {
	return c.hostID
}

func (c *Connector) Role() types.SystemRole {
	return c.role
}

// ClientGetCertificate returns the current credentials for outgoing TLS
// connections to other cluster components.
func (c *Connector) ClientGetCertificate() (*tls.Certificate, error) {
	tlsCert := c.clientState.Load().tlsCert
	if tlsCert == nil {
		return nil, trace.NotFound("no TLS credentials setup for this identity")
	}
	return tlsCert, nil
}

// ClientGetPool returns a pool with the trusted X.509/TLS signers from the host
// CA of the local cluster, as known by the connector.
func (c *Connector) ClientGetPool() (*x509.CertPool, error) {
	roots := c.clientState.Load().pool
	if roots == nil {
		return nil, trace.NotFound("no TLS credentials setup for this identity")
	}
	return roots, nil
}

// ClientAuthMethods returns the [ssh.AuthMethod]s that should be used for
// outgoing SSH connections to other cluster components (the Proxy Service,
// almost surely).
func (c *Connector) ClientAuthMethods() []ssh.AuthMethod {
	return []ssh.AuthMethod{
		ssh.PublicKeysCallback(func() (signers []ssh.Signer, err error) {
			sshCertSigner := c.clientState.Load().sshCertSigner
			if sshCertSigner == nil {
				return nil, nil
			}
			return []ssh.Signer{sshCertSigner}, nil
		}),
	}
}

func (c *Connector) clientIdentityString() string {
	return c.clientState.Load().identity.String()
}

func (c *Connector) clientSSHClientConfig(fips bool) (*ssh.ClientConfig, error) {
	hostKeyCallback, err := apisshutils.NewHostKeyCallback(
		apisshutils.HostKeyCallbackConfig{
			GetHostCheckers: func() ([]ssh.PublicKey, error) {
				return c.clientState.Load().hostCheckers, nil
			},
			FIPS: fips,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ssh.ClientConfig{
		User: c.hostID,
		Auth: []ssh.AuthMethod{ssh.PublicKeysCallback(func() (signers []ssh.Signer, err error) {
			return []ssh.Signer{c.clientState.Load().sshCertSigner}, nil
		})},
		HostKeyCallback: hostKeyCallback,
		Timeout:         apidefaults.DefaultIOTimeout,
	}, nil
}

// ServerTLSConfig returns a new server-side [*tls.Config] that presents the
// connector's credentials as its certificate. The returned tls.Config doesn't
// request or trust any client certificates, so the caller is responsible for
// configuring it.
func (c *Connector) ServerTLSConfig(cipherSuites []uint16) (*tls.Config, error) {
	conf := utils.TLSConfig(cipherSuites)
	conf.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return c.serverGetCertificate()
	}
	return conf, nil
}

// ServerGetHostSigners returns the [ssh.Signer]s that should be used as host
// keys for incoming SSH connections.
func (c *Connector) ServerGetHostSigners() []ssh.Signer {
	sshCertSigner := c.serverState.Load().sshCertSigner
	if sshCertSigner == nil {
		return nil
	}
	return []ssh.Signer{sshCertSigner}
}

func (c *Connector) ServerGetValidPrincipals() []string {
	// TODO(espadolini): get rid of this function after refactoring the two
	// integration tests that use it
	sshCert := c.serverState.Load().sshCert
	if sshCert == nil {
		return nil
	}
	return slices.Clone(sshCert.ValidPrincipals)
}

func (c *Connector) serverGetCertificate() (*tls.Certificate, error) {
	tlsCert := c.serverState.Load().tlsCert
	if tlsCert == nil {
		return nil, trace.NotFound("no TLS credentials setup for this identity")
	}
	return tlsCert, nil
}

func (c *Connector) getPROXYSigner(clock clockwork.Clock) (multiplexer.PROXYHeaderSigner, error) {
	proxySigner, err := multiplexer.NewPROXYSigner(c.clusterName, c.serverGetCertificate, clock)
	if err != nil {
		return nil, trace.Wrap(err, "could not create PROXY signer")
	}
	return proxySigner, nil
}

// TunnelProxyResolver if non-nil, indicates that the client is connected to the Auth Server
// through the reverse SSH tunnel proxy
func (c *Connector) TunnelProxyResolver() reversetunnelclient.Resolver {
	if c.Client == nil || c.Client.Dialer() == nil {
		return nil
	}

	switch dialer := c.Client.Dialer().(type) {
	case *reversetunnelclient.TunnelAuthDialer:
		return dialer.Resolver
	default:
		return nil
	}
}

// UseTunnel indicates if the client is connected directly to the Auth Server
// (false) or through the proxy (true).
func (c *Connector) UseTunnel() bool {
	return c.TunnelProxyResolver() != nil
}

// Close closes resources associated with connector
func (c *Connector) Close() error {
	if c.Client != nil && !c.ReusedClient {
		return c.Client.Close()
	}
	return nil
}

func newConnector(clientIdentity, serverIdentity *state.Identity) (*Connector, error) {
	clientState, err := newConnectorState(clientIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverState := clientState
	if serverIdentity != clientIdentity {
		s, err := newConnectorState(serverIdentity)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		serverState = s
	}
	c := &Connector{
		clusterName: clientIdentity.ClusterName,
		hostID:      clientIdentity.ID.HostUUID,
		role:        clientIdentity.ID.Role,
	}
	c.clientState.Store(clientState)
	c.serverState.Store(serverState)
	return c, nil
}

func newConnectorState(identity *state.Identity) (*connectorState, error) {
	state := &connectorState{
		identity: identity,
	}
	if identity.Cert != nil {
		hostCheckers, err := apisshutils.ParseAuthorizedKeys(identity.SSHCACertBytes)
		if err != nil {
			return nil, trace.Wrap(err, "parsing SSH host CAs")
		}
		state.hostCheckers = hostCheckers

		state.sshCert = identity.Cert
		state.sshCertSigner = identity.KeySigner
	}
	if identity.HasTLSConfig() {
		tlsCert, err := keys.X509KeyPair(identity.TLSCertBytes, identity.KeyBytes)
		if err != nil {
			return nil, trace.Wrap(err, "parsing X.509 certificate")
		}
		tlsCert.Leaf = identity.XCert
		certPool := x509.NewCertPool()
		for j := range identity.TLSCACertsBytes {
			parsedCert, err := tlsca.ParseCertificatePEM(identity.TLSCACertsBytes[j])
			if err != nil {
				return nil, trace.Wrap(err, "parsing X.509 host CA")
			}
			certPool.AddCert(parsedCert)
		}
		state.tlsCert = &tlsCert
		state.pool = certPool
	}
	return state, nil
}

// connectorState contains immutable state (generally derived from a
// [*state.Identity]) suitable for sharing behind an atomic pointer.
type connectorState struct {
	identity *state.Identity

	// tlsCert is the TLS client certificate for the identity, with Signer and
	// Leaf filled.
	tlsCert *tls.Certificate
	// pool contains the host CA certificates trusted by the identity.
	pool *x509.CertPool

	// sshCert is the SSH certificate associated with the identity.
	sshCert *ssh.Certificate
	// sshCertSigner is a [ssh.Signer] presenting the sshCert certificate as its
	// public key.
	sshCertSigner ssh.Signer
	// hostCheckers contains the (non-certificate) public keys that make up the
	// host CA trusted by the identity.
	hostCheckers []ssh.PublicKey
}
