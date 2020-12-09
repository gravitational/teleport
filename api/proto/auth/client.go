package auth

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentAPI,
})

// Client is a grpc Client that connects to a teleport auth server through TLS.
type Client struct {
	Cfg     Config
	grpc    AuthServiceClient
	connMux sync.Mutex
	conn    *grpc.ClientConn
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
	clientConfig := Config{Addrs: addrs, TLS: tlsConfig}

	return NewTLSClient(clientConfig)
}

// NewTLSClient returns a new TLS client that uses mutual TLS authentication
// and dials the remote server using dialer. Connection is loaded lazily.
func NewTLSClient(cfg Config, params ...roundtrip.ClientParam) (*Client, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{Cfg: cfg}, nil
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

// NewFromAuthServiceClient is used to make mock clients for testing
func NewFromAuthServiceClient(asc AuthServiceClient) *Client {
	return &Client{
		grpc: asc,
	}
}

// Close closes the Client connection to the auth server
func (c *Client) Close() error {
	c.connMux.Lock()
	defer c.connMux.Unlock()
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
	return c.Cfg.TLS
}

func (c *Client) isClosed() bool {
	return atomic.LoadInt32(&c.closedFlag) == 1
}

func (c *Client) setClosed() {
	atomic.StoreInt32(&c.closedFlag, 1)
}
