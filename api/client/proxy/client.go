// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"crypto/tls"
	"encoding/asn1"
	"net"
	"slices"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	authpb "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/proxy/transport/transportv1"
	"github.com/gravitational/teleport/api/defaults"
	transportv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
)

// ClientConfig contains configuration needed for a Client
// to be able to connect to the cluster.
type ClientConfig struct {
	// ProxyAddress is the address of the Proxy server.
	ProxyAddress string
	// TLSRoutingEnabled indicates if the cluster is using TLS Routing.
	TLSRoutingEnabled bool
	// TLSConfigFunc produces the [tls.Config] required for mTLS connections to a specific cluster.
	TLSConfigFunc func(cluster string) (*tls.Config, error)
	// UnaryInterceptors are optional [grpc.UnaryClientInterceptor] to apply
	// to the gRPC client.
	UnaryInterceptors []grpc.UnaryClientInterceptor
	// StreamInterceptors are optional [grpc.StreamClientInterceptor] to apply
	// to the gRPC client.
	StreamInterceptors []grpc.StreamClientInterceptor
	// SSHConfig is the [ssh.ClientConfig] used to connect to the Proxy SSH server.
	SSHConfig *ssh.ClientConfig
	// DialTimeout defines how long to attempt dialing before timing out.
	DialTimeout time.Duration
	// DialOpts define options for dialing the client connection.
	DialOpts []grpc.DialOption
	// ALPNConnUpgradeRequired indicates that ALPN connection upgrades are
	// required for making TLS routing requests.
	ALPNConnUpgradeRequired bool
	// InsecureSkipVerify is an option to skip HTTPS cert check
	InsecureSkipVerify bool
	// ViaJumpHost indicates if the connection to the cluster is direct
	// or via another cluster.
	ViaJumpHost bool
	// PROXYHeaderGetter is used if present to get signed PROXY headers to propagate client's IP.
	// Used by proxy's web server to make calls on behalf of connected clients.
	PROXYHeaderGetter client.PROXYHeaderGetter

	// The below items are intended to be used by tests to connect without mTLS.
	// The gRPC transport credentials to use when establishing the connection to proxy.
	creds func(cluster string) (credentials.TransportCredentials, error)
	// The client credentials to use when establishing the connection to auth.
	clientCreds func(cluster string) (client.Credentials, error)
}

// CheckAndSetDefaults ensures required options are present and
// sets the default value of any that are omitted.
func (c *ClientConfig) CheckAndSetDefaults() error {
	if c.ProxyAddress == "" {
		return trace.BadParameter("missing required parameter ProxyAddress")
	}
	if c.SSHConfig == nil {
		return trace.BadParameter("missing required parameter SSHConfig")
	}
	if c.DialTimeout <= 0 {
		c.DialTimeout = defaults.DefaultIOTimeout
	}
	if c.TLSConfigFunc != nil {
		c.clientCreds = func(cluster string) (client.Credentials, error) {
			cfg, err := c.TLSConfigFunc(cluster)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return client.LoadTLS(cfg), nil
		}
		c.creds = func(cluster string) (credentials.TransportCredentials, error) {
			tlsCfg, err := c.TLSConfigFunc(cluster)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if !slices.Contains(tlsCfg.NextProtos, protocolProxySSHGRPC) {
				tlsCfg.NextProtos = append(tlsCfg.NextProtos, protocolProxySSHGRPC)
			}

			// This logic still appears to be necessary to force client to always send
			// a certificate regardless of the server setting. Otherwise the client may pick
			// not to send the client certificate by looking at certificate request.
			if len(tlsCfg.Certificates) > 0 {
				cert := tlsCfg.Certificates[0]
				tlsCfg.Certificates = nil
				tlsCfg.GetClientCertificate = func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
					return &cert, nil
				}
			}

			return credentials.NewTLS(tlsCfg), nil
		}
	} else {
		c.clientCreds = func(cluster string) (client.Credentials, error) {
			return insecureCredentials{}, nil
		}
		c.creds = func(cluster string) (credentials.TransportCredentials, error) {
			return insecure.NewCredentials(), nil
		}
	}

	return nil
}

// insecureCredentials implements [client.Credentials] and is used by tests
// to connect to the Auth server without mTLS.
type insecureCredentials struct{}

func (mc insecureCredentials) TLSConfig() (*tls.Config, error) {
	return nil, nil
}

func (mc insecureCredentials) SSHClientConfig() (*ssh.ClientConfig, error) {
	return nil, trace.NotImplemented("no ssh config")
}

// Client is a client to the Teleport Proxy SSH server on behalf of a user.
// The Proxy SSH port used to serve only SSH, however portions of the api are
// being migrated to gRPC to reduce latency. The Client is capable of communicating
// to the Proxy via both mechanism; by default it will choose to use gRPC over
// SSH where it is able to.
type Client struct {
	// cfg are the user provided configuration parameters required to
	// connect and interact with the Proxy.
	cfg *ClientConfig
	// grpcConn is the established gRPC connection to the Proxy.
	grpcConn *grpc.ClientConn
	// transport is the transportv1.Client
	transport *transportv1.Client
	// clusterName as determined by inspecting the certificate presented by
	// the Proxy during the connection handshake.
	clusterName *clusterName
}

// protocolProxySSHGRPC is TLS ALPN protocol value used to indicate gRPC
// traffic intended for the Teleport Proxy on the SSH port.
const protocolProxySSHGRPC string = "teleport-proxy-ssh-grpc"

// NewClient creates a new Client that attempts to connect to the gRPC
// server being served by the Proxy SSH port by default. If unable to
// connect the Client falls back to connecting to the Proxy SSH port
// via SSH.
//
// If it is known that the gRPC server doesn't serve the required API
// of the caller, then prefer to use NewSSHClient instead which omits
// the gRPC dialing altogether.
func NewClient(ctx context.Context, cfg ClientConfig) (*Client, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := newGRPCClient(ctx, &cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If connecting via a jump host make a call to perform the
	// TLS handshake to ensure that we get the name of the cluster
	// being connected to from its certificate.
	if cfg.ViaJumpHost {
		if _, err := clt.ClusterDetails(ctx); err != nil {
			return nil, trace.NewAggregate(err, clt.Close())
		}
	}
	return clt, trace.Wrap(err)
}

// clusterName stores the name of the cluster
// in a protected manner which allows it to
// be set during handshakes with the server.
type clusterName struct {
	name atomic.Pointer[string]
}

func (c *clusterName) get() string {
	name := c.name.Load()
	if name != nil {
		return *name
	}
	return ""
}

func (c *clusterName) set(name string) {
	c.name.CompareAndSwap(nil, &name)
}

// clusterCredentials is a [credentials.TransportCredentials] implementation
// that obtains the name of the cluster being connected to from the certificate
// presented by the server. This allows the client to determine the cluster name when
// connecting via jump hosts.
type clusterCredentials struct {
	credentials.TransportCredentials
	clusterName *clusterName
}

// teleportClusterASN1ExtensionOID is an extension ID used when encoding/decoding
// origin teleport cluster name into certificates.
var teleportClusterASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 7}

// ClientHandshake performs the handshake with the wrapped [credentials.TransportCredentials] and
// then inspects the provided cert for the [teleportClusterASN1ExtensionOID] to determine
// the cluster that the server belongs to.
func (c *clusterCredentials) ClientHandshake(ctx context.Context, authority string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, info, err := c.TransportCredentials.ClientHandshake(ctx, authority, conn)
	if err != nil {
		return conn, info, trace.Wrap(err)
	}

	tlsInfo, ok := info.(credentials.TLSInfo)
	if !ok {
		return conn, info, nil
	}

	certs := tlsInfo.State.PeerCertificates
	if len(certs) == 0 {
		return conn, info, nil
	}

	clientCert := certs[0]
	for _, attr := range clientCert.Subject.Names {
		if attr.Type.Equal(teleportClusterASN1ExtensionOID) {
			val, ok := attr.Value.(string)
			if ok {
				c.clusterName.set(val)
				break
			}
		}
	}

	return conn, info, nil
}

// newGRPCClient creates a Client that is connected via gRPC.
func newGRPCClient(ctx context.Context, cfg *ClientConfig) (_ *Client, err error) {
	dialCtx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()

	c := &clusterName{}

	creds, err := cfg.creds("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, err := grpc.DialContext(
		dialCtx,
		cfg.ProxyAddress,
		append([]grpc.DialOption{
			grpc.WithContextDialer(newDialerForGRPCClient(ctx, cfg)),
			grpc.WithTransportCredentials(&clusterCredentials{TransportCredentials: creds, clusterName: c}),
			grpc.WithChainUnaryInterceptor(
				append(cfg.UnaryInterceptors,
					//nolint:staticcheck // SA1019. There is a data race in the stats.Handler that is replacing
					// the interceptor. See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/4576.
					otelgrpc.UnaryClientInterceptor(),
					metadata.UnaryClientInterceptor,
					interceptors.GRPCClientUnaryErrorInterceptor,
				)...,
			),
			grpc.WithChainStreamInterceptor(
				append(cfg.StreamInterceptors,
					//nolint:staticcheck // SA1019. There is a data race in the stats.Handler that is replacing
					// the interceptor. See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/4576.
					otelgrpc.StreamClientInterceptor(),
					metadata.StreamClientInterceptor,
					interceptors.GRPCClientStreamErrorInterceptor,
				)...,
			),
		}, cfg.DialOpts...)...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()

	transport, err := transportv1.NewClient(transportv1pb.NewTransportServiceClient(conn))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{
		cfg:         cfg,
		grpcConn:    conn,
		transport:   transport,
		clusterName: c,
	}, nil
}

func newDialerForGRPCClient(ctx context.Context, cfg *ClientConfig) func(context.Context, string) (net.Conn, error) {
	return client.GRPCContextDialer(client.NewDialer(ctx, defaults.DefaultIdleTimeout, cfg.DialTimeout,
		client.WithInsecureSkipVerify(cfg.InsecureSkipVerify),
		client.WithALPNConnUpgrade(cfg.ALPNConnUpgradeRequired),
		client.WithALPNConnUpgradePing(true), // Use Ping protocol for long-lived connections.
		client.WithPROXYHeaderGetter(cfg.PROXYHeaderGetter),
	))
}

// ClusterName returns the name of the cluster that the
// connected Proxy is a member of.
func (c *Client) ClusterName() string {
	return c.clusterName.get()
}

// Close attempts to close both the gRPC and SSH connections.
func (c *Client) Close() error {
	return trace.Wrap(c.grpcConn.Close())
}

// SSHConfig returns the [ssh.ClientConfig] for the provided user which
// should be used when creating a [tracessh.Client] with the returned
// [net.Conn] from [Client.DialHost].
func (c *Client) SSHConfig(user string) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		Config:            c.cfg.SSHConfig.Config,
		User:              user,
		Auth:              c.cfg.SSHConfig.Auth,
		HostKeyCallback:   c.cfg.SSHConfig.HostKeyCallback,
		BannerCallback:    c.cfg.SSHConfig.BannerCallback,
		ClientVersion:     c.cfg.SSHConfig.ClientVersion,
		HostKeyAlgorithms: c.cfg.SSHConfig.HostKeyAlgorithms,
		Timeout:           c.cfg.SSHConfig.Timeout,
	}
}

// ClusterDetails provide cluster configuration
// details as known by the connected Proxy.
type ClusterDetails struct {
	// FIPS dictates whether FIPS mode is enabled.
	FIPS bool
}

// ClientConfig returns a [client.Config] that may be used to connect to the
// Auth server in the provided cluster via [client.New] or similar. The [client.Config]
// returned will have the correct credentials and dialer set based on the ClientConfig
// that was provided to create this Client.
func (c *Client) ClientConfig(ctx context.Context, cluster string) (client.Config, error) {
	creds, err := c.cfg.clientCreds(cluster)
	if err != nil {
		return client.Config{}, trace.Wrap(err)
	}

	if c.cfg.TLSRoutingEnabled {
		return client.Config{
			Context:                    ctx,
			Addrs:                      []string{c.cfg.ProxyAddress},
			Credentials:                []client.Credentials{creds},
			ALPNSNIAuthDialClusterName: cluster,
			CircuitBreakerConfig:       breaker.NoopBreakerConfig(),
			ALPNConnUpgradeRequired:    c.cfg.ALPNConnUpgradeRequired,
			DialOpts:                   c.cfg.DialOpts,
			InsecureAddressDiscovery:   c.cfg.InsecureSkipVerify,
			DialInBackground:           true,
		}, nil
	}

	return client.Config{
		Context:                  ctx,
		Credentials:              []client.Credentials{creds},
		CircuitBreakerConfig:     breaker.NoopBreakerConfig(),
		DialInBackground:         true,
		InsecureAddressDiscovery: c.cfg.InsecureSkipVerify,
		Dialer: client.ContextDialerFunc(func(dialCtx context.Context, _ string, _ string) (net.Conn, error) {
			conn, err := c.transport.DialCluster(dialCtx, cluster, nil)
			return conn, trace.Wrap(err)
		}),
		DialOpts: c.cfg.DialOpts,
	}, nil
}

// DialHost establishes a connection to the `target` in cluster named `cluster`. If a keyring
// is provided it will only be forwarded if proxy recording mode is enabled in the cluster.
func (c *Client) DialHost(ctx context.Context, target, cluster string, keyring agent.ExtendedAgent) (net.Conn, ClusterDetails, error) {
	conn, details, err := c.transport.DialHost(ctx, target, cluster, nil, keyring)
	if err != nil {
		return nil, ClusterDetails{}, trace.ConnectionProblem(err, "failed connecting to host %s: %v", target, err)
	}

	return conn, ClusterDetails{FIPS: details.FipsEnabled}, nil
}

// ClusterDetails retrieves cluster information as seen by the Proxy.
func (c *Client) ClusterDetails(ctx context.Context) (ClusterDetails, error) {
	details, err := c.transport.ClusterDetails(ctx)
	if err != nil {
		return ClusterDetails{}, trace.Wrap(err)
	}

	return ClusterDetails{FIPS: details.FipsEnabled}, nil
}

// Ping measures the round trip latency of sending a message to the Proxy.
func (c *Client) Ping(ctx context.Context) error {
	// TODO(tross): Update to call Ping when it is added to the transport service.
	// For now we don't really care what method is used we just want to measure
	// how long it takes to get a reply. This will always fail with a not implemented
	// error since the Proxy gRPC server doesn't serve the auth service proto. However,
	// we use it because it's already imported in the api package.
	clt := authpb.NewAuthServiceClient(c.grpcConn)
	_, _ = clt.Ping(ctx, &authpb.PingRequest{})
	return nil
}
