package cloud

import (
	"crypto/tls"
	"io"
	"sync/atomic"

	v1 "github.com/gravitational/teleport/e/api/cloud/v1"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ClientConfig is the Teleport Pro (Enterprise) config
type ClientConfig struct {
	// Hostname is the hostname of the API Server
	Hostname string
	// TLSConfig is the client TLS configuration
	TLSConfig *tls.Config
}

// CheckAndSetDefaults checks and sets default config values
func (c *ClientConfig) CheckAndSetDefaults() (err error) {
	if c.Hostname == "" {
		return trace.BadParameter("missing API server hostname")
	}

	if c.TLSConfig == nil {
		return trace.BadParameter("missing TLS configuration")
	}

	return nil
}

// NewClient creates a client that talks to Cloud API Server
func NewClient(cfg ClientConfig) (Client, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, err := grpc.Dial(cfg.Hostname, grpc.WithTransportCredentials(credentials.NewTLS(cfg.TLSConfig)))
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return NewClientFromConnection(conn)
}

// NewClientFromConnection creates a client using existing connection
func NewClientFromConnection(conn *grpc.ClientConn) (Client, error) {
	if conn == nil {
		return nil, trace.BadParameter("missing connection")
	}

	client := v1.NewTenantsServiceClient(conn)
	return &cloudClient{
		TenantsServiceClient: client,
		conn:                 conn,
	}, nil
}

// Client is a client of the Cloud Server API
type cloudClient struct {
	// closedFlag is set to indicate that the services are closed.
	closedFlag int32
	// grpc is the gRPC client specification
	v1.TenantsServiceClient
	// conn is a grpc connection
	conn *grpc.ClientConn
}

// Client is a client of the Cloud Server API
type Client interface {
	v1.TenantsServiceClient
	// Close closes the Client connection to the auth server
	io.Closer
}

// Close closes the Client connection to the auth server
func (c *cloudClient) Close() error {
	if !c.setClosed() {
		return nil
	}
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return trace.Wrap(err)
	}
	return nil
}

// setClosed marks the client to closed and returns true
// if the client was successfully marked as closed.
func (c *cloudClient) setClosed() bool {
	return atomic.CompareAndSwapInt32(&c.closedFlag, 0, 1)
}
