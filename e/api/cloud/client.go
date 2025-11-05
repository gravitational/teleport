package cloud

import (
	"crypto/tls"
	"io"
	"os"
	"sync/atomic"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/trail"
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/utils"
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

const (
	// DefaultAPIServerAddr is default cloud API server address
	DefaultAPIServerAddr = "api.teleport.sh"
	// DefaultAPIServerPort is the default SalesCenter API port
	DefaultAPIServerPort = 443
	// EnvVarHostPort is used to override the default cloud api server address
	EnvVarHostPort = "TELEPORT_CLOUD_HOSTPORT"
)

// NewClientFromTLSConfig creates a client using the provided [tls.Config]
// as a base. The Cloud server address defaults to api.teleport.sh but can
// be overridden via the TELEPORT_CLOUD_HOSTPORT envvar
func NewClientFromTLSConfig(cfg *tls.Config) (Client, error) {
	cloudAPIServerAddr := DefaultAPIServerAddr
	if addr := os.Getenv(EnvVarHostPort); addr != "" {
		cloudAPIServerAddr = addr
	}

	apiServerAddr, err := utils.ParseHostPortAddr(cloudAPIServerAddr, DefaultAPIServerPort)
	if err != nil {
		return nil, trace.BadParameter("invalid cloud API server address")
	}

	tlsCfg := cfg.Clone()
	tlsCfg.ServerName = apiServerAddr.Host()
	tlsCfg.InsecureSkipVerify = lib.IsInsecureDevMode()

	return NewClient(ClientConfig{
		Hostname:  apiServerAddr.Addr,
		TLSConfig: tlsCfg,
	})
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

	// Hostname returns the hostname of the server the client connects to.
	Hostname() string
}

func (c *cloudClient) Hostname() string {
	return c.conn.Target()
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

// IsCloudEnv returns true if the EnvVarHostPort is set within the environment. This is set in the cloud environment.
func IsCloudEnv() bool {
	return os.Getenv(EnvVarHostPort) != ""
}
