package relaytunnel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/hashicorp/yamux"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	relaytunnelv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaytunnel/v1alpha"
	"github.com/gravitational/teleport/lib/tlsca"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type ClientConfig struct {
	GetCertificate func() (*tls.Certificate, error)
	GetPool        func() (*x509.CertPool, error)
	Ciphersuites   []uint16

	TunnelType types.TunnelType

	RelayLoadBalancerAddr string
	TargetConnectionCount int
}

func NewClient(cfg ClientConfig) (*Client, error) {
	c := &Client{
		getCertificate: cfg.GetCertificate,
		getPool:        cfg.GetPool,
		ciphersuites:   cfg.Ciphersuites,

		targetConnectionCount: cfg.TargetConnectionCount,
		relayLoadBalancerAddr: cfg.RelayLoadBalancerAddr,
	}

	return c, nil
}

type Client struct {
	getCertificate func() (*tls.Certificate, error)
	getPool        func() (*x509.CertPool, error)
	ciphersuites   []uint16

	tunnelType types.TunnelType

	log *slog.Logger
	wg  sync.WaitGroup

	mu          sync.Mutex
	started     bool
	terminating bool

	relayLoadBalancerAddr string
	targetConnectionCount int

	// activeConnections is keyed by relay username (i.e. host ID dot
	// clustername).
	activeConnections map[string]clientConn
}

type clientConn interface {
	Close() error

	ServerIsTerminating() bool
}

func (c *Client) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return trace.AlreadyExists("relay tunnel client already started")
	}
	c.started = true

	if c.terminating {
		return nil
	}

	c.wg.Add(1)
	go c.dialLoopGrouped()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.discoverLoop()
	}()

	return nil
}

func (c *Client) discoverLoop() {
	// TODO(espadolini): discover configuration based on relay api endpoint
}

func (c *Client) dialLoopGrouped() {
	defer c.wg.Done()

	for {
		if !c.shouldDial() {
			c.log.LogAttrs(context.Background(), logutils.TraceLevel, "not attempting new relay tunnel connection")
			time.Sleep(time.Second)
			continue
		}

		c.dialRelay(c.relayLoadBalancerAddr)

	}
}

type relayAlreadyConnectedError struct {
	HostID string
}

func (err *relayAlreadyConnectedError) Error() string {
	return fmt.Sprintf("tunnel to relay %q already established", err.HostID)
}

func (c *Client) dialRelay(addr string) error {
	log := c.log.With("connection_id", uuid.NewString())
	log.DebugContext(context.Background(), "attempting new relay tunnel connection")

	cert, err := c.getCertificate()
	if err != nil {
		return trace.Wrap(err)
	}

	pool, err := c.getPool()
	if err != nil {
		return trace.Wrap(err)
	}

	helloCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	nc, err := new(net.Dialer).DialContext(helloCtx, "tcp", addr)
	if err != nil {
		return trace.Wrap(err)
	}

	var serverID *tlsca.Identity
	tlsConfig := &tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return cert, nil
		},

		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.NegotiatedProtocol == "" {
				return trace.NotImplemented("relay tunnel protocol not supported")
			}

			opts := x509.VerifyOptions{
				DNSName: "",

				Roots:         pool,
				Intermediates: nil,

				KeyUsages: []x509.ExtKeyUsage{
					x509.ExtKeyUsageServerAuth,
				},
			}
			if len(cs.PeerCertificates) > 1 {
				opts.Intermediates = x509.NewCertPool()
				for _, cert := range cs.PeerCertificates[1:] {
					opts.Intermediates.AddCert(cert)
				}
			}
			if _, err := cs.PeerCertificates[0].Verify(opts); err != nil {
				return trace.Wrap(err)
			}

			id, err := tlsca.FromSubject(cs.PeerCertificates[0].Subject, cs.PeerCertificates[0].NotAfter)
			if err != nil {
				return trace.Wrap(err)
			}
			serverID = id

			if !slices.Contains(id.Groups, string(types.RoleRelay)) &&
				!slices.Contains(id.SystemRoles, string(types.RoleRelay)) {
				return trace.BadParameter("dialed server is not a relay (roles %+q, system roles %+q)", id.Groups, id.SystemRoles)
			}

			c.mu.Lock()
			_, alreadyConnected := c.activeConnections[id.Username]
			c.mu.Unlock()
			if alreadyConnected {
				return trace.AlreadyExists("relay %+q already claimed", id.Username)
			}

			return nil
		},

		NextProtos: []string{yamuxTunnelALPN},
		ServerName: "",

		CipherSuites: c.ciphersuites,
		MinVersion:   tls.VersionTLS12,
	}

	tc := tls.Client(nc, tlsConfig)
	if err := tc.HandshakeContext(helloCtx); err != nil {
		return trace.Wrap(err)
	}

	c.mu.Lock()
	_, alreadyConnected := c.activeConnections[serverID.Username]
	c.mu.Unlock()
	if alreadyConnected {
		_ = tc.Close()
		return trace.AlreadyExists("relay %+q already claimed (after TLS handshake)", serverID.Username)
	}

	yamuxConfig := &yamux.Config{
		AcceptBacklog: 128,

		EnableKeepAlive:        true,
		KeepAliveInterval:      30 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,

		MaxStreamWindowSize: 256 * 1024,

		StreamCloseTimeout: time.Minute,
		StreamOpenTimeout:  30 * time.Second,

		LogOutput: nil,
		Logger:    (*yamuxLogger)(log),
	}

	session, err := yamux.Client(tc, yamuxConfig)
	if err != nil {
		_ = tc.Close()
		return err
	}

	controlStream, err := session.OpenStream()
	if err != nil {
		_ = session.Close()
		return trace.Wrap(err)
	}
	helloDeadline, _ := helloCtx.Deadline()
	controlStream.SetDeadline(helloDeadline)

	if err := writeProto(controlStream, &relaytunnelv1alpha.ClientHello{
		TunnelType: string(c.tunnelType),
	}); err != nil {
		_ = controlStream.Close()
		_ = session.Close()
		return trace.Wrap(err)
	}

	serverHello := new(relaytunnelv1alpha.ServerHello)
	if err := readProto(controlStream, serverHello); err != nil {
		_ = controlStream.Close()
		_ = session.Close()
		return trace.Wrap(err)
	}

	controlStream.SetDeadline(time.Time{})

	if err := trail.FromGRPC(status.FromProto(serverHello.GetStatus()).Err()); err != nil {
		_ = controlStream.Close()
		_ = session.Close()
		return trace.Wrap(err)
	}

	c.mu.Lock()
	_, alreadyConnected = c.activeConnections[serverID.Username]
	if alreadyConnected {
		c.mu.Unlock()
		_ = controlStream.Close()
		_ = session.Close()
		return trace.AlreadyExists("relay %+q already claimed (after tunnel handshake)", serverID.Username)
	}
	cc := &yamuxClientConn{
		session: session,
	}
	c.activeConnections[serverID.Username] = cc
	c.mu.Unlock()

	cc.run(controlStream)

	return nil
}

type yamuxClientConn struct {
	session *yamux.Session

	terminating atomic.Bool
}

func (c *yamuxClientConn) run(controlStream *yamux.Stream) {

}

// Close implements [clientConn].
func (c *yamuxClientConn) Close() error {
	return c.session.Close()
}

// ServerIsTerminating implements [clientConn].
func (c *yamuxClientConn) ServerIsTerminating() bool {
	return c.terminating.Load()
}

func (c *Client) shouldDial() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	var thrivingConnections int
	for _, cc := range c.activeConnections {
		if !cc.ServerIsTerminating() {
			thrivingConnections++
		}
	}

	return thrivingConnections < c.targetConnectionCount
}
