package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentAuth,
})

// Client is HTTP Auth API client. It works by connecting to auth servers
// via HTTP.
//
// When Teleport servers connect to auth API, they usually establish an SSH
// tunnel first, and then do HTTP-over-SSH. This client is wrapped by auth.TunClient
// in lib/auth/tun.go
type Client struct {
	sync.Mutex
	ClientConfig
	grpc AuthServiceClient
	conn *grpc.ClientConn
	// closedFlag is set to indicate that the services are closed
	closedFlag int32
}

// NewClient establishes a gRPC connection to an auth server.
func NewClient() (*Client, error) {
	tlsConfig, err := PathCreds("certs/api-admin")
	if err != nil {
		return nil, fmt.Errorf("Failed to setup TLS config: %v", err)
	}

	// replace 127.0.0.1:3025 (default) with your auth server address
	addrs := []utils.NetAddr{{Addr: "127.0.0.1:3025"}}
	clientConfig := ClientConfig{Addrs: addrs, TLS: tlsConfig}

	client, err := NewTLSClient(clientConfig)
	return client, nil
}

// NewTLSClient returns a new TLS client that uses mutual TLS authentication
// and dials the remote server using dialer. Connection is loaded lazily.
func NewTLSClient(cfg ClientConfig, params ...roundtrip.ClientParam) (*Client, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{ClientConfig: cfg}, nil
}

// PathCreds loads mounted creds from path, detects reloads and updates the grpc transport
func PathCreds(path string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(path+".crt", path+".key")
	if err != nil {
		return nil, err
	}
	caFile, err := os.Open(path + ".cas")
	if err != nil {
		return nil, err
	}
	caCerts, err := ioutil.ReadAll(caFile)
	if err != nil {
		return nil, err
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCerts); !ok {
		return nil, fmt.Errorf("invalid CA cert PEM")
	}
	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
	}
	return conf, nil
}

// Close closes the Client connection to the auth server
func (c *Client) Close() error {
	c.Lock()
	defer c.Unlock()
	c.setClosed()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// TLSConfig returns TLS config used by the client, could return nil
// if the client is not using TLS
func (c *Client) TLSConfig() *tls.Config {
	return c.ClientConfig.TLS
}

func (c *Client) isClosed() bool {
	return atomic.LoadInt32(&c.closedFlag) == 1
}

func (c *Client) setClosed() {
	atomic.StoreInt32(&c.closedFlag, 1)
}

// Make sure Client implements all the necessary methods.
var _ ClientI = &Client{}

// ClientI is a client to Auth service
type ClientI interface {
	// GetUsers(withSecrets bool) ([]services.User, error)

	// auth.IdentityService
	// auth.ProvisioningService
	// services.Trust
	// events.IAuditLog
	// events.Streamer
	// events.Emitter
	// services.Presence
	// services.Access
	// services.DynamicAccess
	// auth.WebService
	// session.Service
	// services.ClusterConfiguration
	// services.Events

	// // NewKeepAliver returns a new instance of keep aliver
	// NewKeepAliver(ctx context.Context) (services.KeepAliver, error)

	// // RotateCertAuthority starts or restarts certificate authority rotation process.
	// RotateCertAuthority(req auth.RotateRequest) error

	// // RotateExternalCertAuthority rotates external certificate authority,
	// // this method is used to update only public keys and certificates of the
	// // the certificate authorities of trusted clusters.
	// RotateExternalCertAuthority(ca services.CertAuthority) error

	// // ValidateTrustedCluster validates trusted cluster token with
	// // main cluster, in case if validation is successful, main cluster
	// // adds remote cluster
	// ValidateTrustedCluster(*auth.ValidateTrustedClusterRequest) (*auth.ValidateTrustedClusterResponse, error)

	// // GetDomainName returns auth server cluster name
	// GetDomainName() (string, error)

	// // GetClusterCACert returns the CAs for the local cluster without signing keys.
	// GetClusterCACert() (*auth.LocalCAResponse, error)

	// // GenerateServerKeys generates new host private keys and certificates (signed
	// // by the host certificate authority) for a node
	// GenerateServerKeys(auth.GenerateServerKeysRequest) (*auth.PackedKeys, error)
	// // AuthenticateWebUser authenticates web user, creates and  returns web session
	// // in case if authentication is successful
	// AuthenticateWebUser(req auth.AuthenticateUserRequest) (services.WebSession, error)
	// // AuthenticateSSHUser authenticates SSH console user, creates and  returns a pair of signed TLS and SSH
	// // short lived certificates as a result
	// AuthenticateSSHUser(req auth.AuthenticateSSHRequest) (*auth.SSHLoginResponse, error)

	// // ProcessKubeCSR processes CSR request against Kubernetes CA, returns
	// // signed certificate if successful.
	// ProcessKubeCSR(req auth.KubeCSR) (*auth.KubeCSRResponse, error)

	// Ping gets basic info about the auth server.
	Ping(ctx context.Context) (auth.PingResponse, error)

	// // CreateAppSession creates an application web session. Application web
	// // sessions represent a browser session the client holds.
	// CreateAppSession(context.Context, services.CreateAppSessionRequest) (services.WebSession, error)
}
