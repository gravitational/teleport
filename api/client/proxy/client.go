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
	"io"
	"net"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proxy/transport/transportv1"
	"github.com/gravitational/teleport/api/defaults"
	transportv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
)

// SSHDialer provides a mechanism to create a ssh client.
type SSHDialer interface {
	// Dial establishes a client connection to an SSH server.
	Dial(ctx context.Context, network string, addr string, config *ssh.ClientConfig) (*tracessh.Client, error)
}

// SSHDialerFunc implements SSHDialer
type SSHDialerFunc func(ctx context.Context, network string, addr string, config *ssh.ClientConfig) (*tracessh.Client, error)

// Dial calls f(ctx, network, addr, config).
func (f SSHDialerFunc) Dial(ctx context.Context, network string, addr string, config *ssh.ClientConfig) (*tracessh.Client, error) {
	return f(ctx, network, addr, config)
}

// ClientConfig contains configuration needed for a Client
// to be able to connect to the cluster.
type ClientConfig struct {
	// ProxyWebAddress is the address of the Proxy Web server.
	ProxyWebAddress string
	// ProxySSHAddress is the address of the Proxy SSH server.
	ProxySSHAddress string
	// TLSRoutingEnabled indicates if the cluster is using TLS Routing.
	TLSRoutingEnabled bool
	// ClusterName is the name of the Teleport cluster that the client
	// will be connected to.
	ClusterName string
	// TLSConfig contains the tls.Config required for mTLS connections.
	TLSConfig *tls.Config
	// UnaryInterceptors are optional [grpc.UnaryClientInterceptor] to apply
	// to the gRPC client.
	UnaryInterceptors []grpc.UnaryClientInterceptor
	// StreamInterceptors are optional [grpc.StreamClientInterceptor] to apply
	// to the gRPC client.
	StreamInterceptors []grpc.StreamClientInterceptor
	// SSHDialer allows callers to control how a [tracessh.Client] is created.
	SSHDialer SSHDialer
	// SSHConfig is the [ssh.ClientConfig] used to connect to the Proxy SSH server.
	SSHConfig *ssh.ClientConfig
	// DialTimeout defines how long to attempt dialing before timing out.
	DialTimeout time.Duration
	// DialOpts define options for dialing the client connection.
	DialOpts []grpc.DialOption

	// The below items are intended to be used by tests to connect without mTLS.
	// The gRPC transport credentials to use when establishing the connection to proxy.
	creds func() credentials.TransportCredentials
	// The client credentials to use when establishing the connection to auth.
	clientCreds func() client.Credentials
}

func (c *ClientConfig) CheckAndSetDefaults() error {
	if c.ProxyWebAddress == "" {
		return trace.BadParameter("missing required parameter ProxyWebAddress")
	}
	if c.ProxySSHAddress == "" {
		return trace.BadParameter("missing required parameter ProxySSHAddress")
	}
	if c.ClusterName == "" {
		return trace.BadParameter("missing required parameter ClusterName")
	}
	if c.SSHDialer == nil {
		return trace.BadParameter("missing required parameter SSHDialer")
	}
	if c.SSHConfig == nil {
		return trace.BadParameter("missing required parameter SSHConfig")
	}
	if c.DialTimeout <= 0 {
		c.DialTimeout = defaults.DefaultIOTimeout
	}

	if c.TLSConfig != nil {
		if !slices.Contains(c.TLSConfig.NextProtos, protocolProxySSHGRPC) {
			tlsCfg := c.TLSConfig.Clone()
			tlsCfg.NextProtos = append(tlsCfg.NextProtos, protocolProxySSHGRPC)
			c.TLSConfig = tlsCfg
		}
		c.clientCreds = func() client.Credentials {
			return client.LoadTLS(c.TLSConfig.Clone())
		}
		c.creds = func() credentials.TransportCredentials {
			return credentials.NewTLS(c.TLSConfig.Clone())
		}
	} else {
		c.clientCreds = func() client.Credentials {
			return insecureCredentials{}
		}
		c.creds = func() credentials.TransportCredentials {
			return insecure.NewCredentials()
		}
	}

	return nil
}

type insecureCredentials struct{}

func (mc insecureCredentials) Dialer(client.Config) (client.ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

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
	// sshClient is the established SSH connection to the Proxy.
	sshClient *tracessh.Client
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

	clt, grpcErr := newGRPCClient(ctx, &cfg)
	if grpcErr == nil {
		// Attempt an RPC to ensure the proxy is serving gRPC on the
		// SSH Port. This is needed for backward compatibility with
		// Proxies that aren't serving gRPC since dialing happens in
		// the background.
		//
		// DELETE IN 14.0.0
		_, err := clt.transport.ClusterDetails(ctx)
		if err == nil {
			return clt, nil
		}
	}

	clt, sshErr := newSSHClient(ctx, &cfg)
	if sshErr == nil {
		return clt, nil
	}

	return nil, trace.NewAggregate(grpcErr, sshErr)
}

// newGRPCClient creates a Client that is connected via gRPC.
func newGRPCClient(ctx context.Context, cfg *ClientConfig) (_ *Client, err error) {
	dialCtx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		cfg.ProxySSHAddress,
		append(cfg.DialOpts,
			grpc.WithTransportCredentials(cfg.creds()),
			grpc.WithChainUnaryInterceptor(
				append(cfg.UnaryInterceptors,
					otelgrpc.UnaryClientInterceptor(),
					metadata.UnaryClientInterceptor,
				)...,
			),
			grpc.WithChainStreamInterceptor(
				append(cfg.StreamInterceptors,
					otelgrpc.StreamClientInterceptor(),
					metadata.StreamClientInterceptor,
				)...,
			),
		)...,
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
		cfg:       cfg,
		grpcConn:  conn,
		transport: transport,
	}, nil
}

// newSSHClient creates a Client that is connected via SSH.
func newSSHClient(ctx context.Context, cfg *ClientConfig) (*Client, error) {
	clt, err := cfg.SSHDialer.Dial(ctx, "tcp", cfg.ProxySSHAddress, cfg.SSHConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{
		cfg:       cfg,
		sshClient: clt,
	}, nil
}

// Close attempts to close both the gRPC and SSH connections.
func (c *Client) Close() error {
	var errs []error

	if c.sshClient != nil {
		errs = append(errs, c.sshClient.Close())
	}

	if c.grpcConn != nil {
		errs = append(errs, c.grpcConn.Close())
	}

	return trace.NewAggregate(errs...)
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
func (c *Client) ClientConfig(ctx context.Context, cluster string) client.Config {
	switch {
	case c.cfg.TLSRoutingEnabled:
		return client.Config{
			Context:                    ctx,
			Addrs:                      []string{c.cfg.ProxyWebAddress},
			Credentials:                []client.Credentials{c.cfg.clientCreds()},
			ALPNSNIAuthDialClusterName: cluster,
			CircuitBreakerConfig:       breaker.NoopBreakerConfig(),
		}
	case c.sshClient != nil:
		return client.Config{
			Context:              ctx,
			Credentials:          []client.Credentials{c.cfg.clientCreds()},
			CircuitBreakerConfig: breaker.NoopBreakerConfig(),
			DialInBackground:     true,
			Dialer: client.ContextDialerFunc(func(dialCtx context.Context, _ string, _ string) (net.Conn, error) {
				// Don't dial if the context has timed out.
				select {
				case <-dialCtx.Done():
					return nil, dialCtx.Err()
				default:
				}

				conn, err := dialSSH(dialCtx, c.sshClient, c.cfg.ProxySSHAddress, "@"+cluster, nil)
				return conn, trace.Wrap(err)
			}),
		}
	default:
		return client.Config{
			Context:              ctx,
			Credentials:          []client.Credentials{c.cfg.clientCreds()},
			CircuitBreakerConfig: breaker.NoopBreakerConfig(),
			DialInBackground:     true,
			Dialer: client.ContextDialerFunc(func(dialCtx context.Context, _ string, _ string) (net.Conn, error) {
				// Don't dial if the context has timed out.
				select {
				case <-dialCtx.Done():
					return nil, dialCtx.Err()
				default:
				}

				// Intentionally not using the dial context because it is only valid
				// for the lifetime of the dial. Using it causes the stream to be terminated
				// immediately after the dial completes.
				connContext := tracing.WithPropagationContext(context.Background(), tracing.PropagationContextFromContext(dialCtx))
				conn, err := c.transport.DialCluster(connContext, cluster, nil)
				return conn, trace.Wrap(err)
			}),
		}
	}
}

// DialHost establishes a connection to the `target` in cluster named `cluster`. If a keyring
// is provided it will only be forwarded if proxy recording mode is enabled in the cluster.
func (c *Client) DialHost(ctx context.Context, target, cluster string, keyring agent.ExtendedAgent) (net.Conn, ClusterDetails, error) {
	if c.sshClient != nil {
		conn, details, err := c.dialHostSSH(ctx, target, cluster, keyring)
		return conn, details, trace.Wrap(err)
	}

	conn, details, err := c.transport.DialHost(ctx, target, cluster, nil, keyring)
	if err != nil {
		return nil, ClusterDetails{}, trace.Wrap(err)
	}

	return conn, ClusterDetails{FIPS: details.FipsEnabled}, nil
}

// dialHostSSH connects to the target via SSH. To match backwards compatibility the
// cluster details are retrieved from the Proxy SSH server via a clusterDetailsRequest
// request to determine if the keyring should be forwarded.
func (c *Client) dialHostSSH(ctx context.Context, target, cluster string, keyring agent.ExtendedAgent) (net.Conn, ClusterDetails, error) {
	details, err := c.clusterDetailsSSH(ctx)
	if err != nil {
		return nil, ClusterDetails{FIPS: details.FIPSEnabled}, trace.Wrap(err)
	}

	// Prevent forwarding the keychain if the proxy is
	// not doing the recording.
	if !details.RecordingProxy {
		keyring = nil
	}

	conn, err := dialSSH(ctx, c.sshClient, c.cfg.ProxySSHAddress, target+"@"+cluster, keyring)
	return conn, ClusterDetails{FIPS: details.FIPSEnabled}, trace.Wrap(err)
}

// ClusterDetails retrieves cluster information as seen by the Proxy.
func (c *Client) ClusterDetails(ctx context.Context) (ClusterDetails, error) {
	if c.sshClient != nil {
		details, err := c.clusterDetailsSSH(ctx)
		return ClusterDetails{FIPS: details.FIPSEnabled}, trace.Wrap(err)
	}

	details, err := c.transport.ClusterDetails(ctx)
	if err != nil {
		return ClusterDetails{}, trace.Wrap(err)
	}

	return ClusterDetails{FIPS: details.FipsEnabled}, nil
}

// sshDetails is the response from a clusterDetailsRequest.
type sshDetails struct {
	RecordingProxy bool
	FIPSEnabled    bool
}

const clusterDetailsRequest = "cluster-details@goteleport.com"

// clusterDetailsSSH retrieves the cluster details via a clusterDetailsRequest.
func (c *Client) clusterDetailsSSH(ctx context.Context) (sshDetails, error) {
	ok, resp, err := c.sshClient.SendRequest(ctx, clusterDetailsRequest, true, nil)
	if err != nil {
		return sshDetails{}, trace.Wrap(err)
	}

	if !ok {
		return sshDetails{}, trace.ConnectionProblem(nil, "failed to get cluster details")
	}

	var details sshDetails
	if err := ssh.Unmarshal(resp, &details); err != nil {
		return sshDetails{}, trace.Wrap(err)
	}

	return details, trace.Wrap(err)
}

// dialSSH creates a SSH session to the target address and proxies a [net.Conn]
// over the standard input and output of the session.
func dialSSH(ctx context.Context, clt *tracessh.Client, proxyAddress, targetAddress string, keyring agent.ExtendedAgent) (_ net.Conn, err error) {
	session, err := clt.NewSession(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		if err != nil {
			_ = session.Close()
		}
	}()

	conn, err := newSessionConn(session, proxyAddress, targetAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()

	sessionError, err := session.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If a keyring was provided then set up agent forwarding.
	if keyring != nil {
		// Add a handler to receive requests on the auth-agent@openssh.com channel. If there is
		// already a handler it's safe to ignore the error because we only need one active handler
		// to process requests.
		err = agent.ForwardToAgent(clt.Client, keyring)
		if err != nil && !strings.Contains(err.Error(), "agent: already have handler for") {
			return nil, trace.Wrap(err)
		}

		err = agent.RequestAgentForwarding(session.Session)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := session.RequestSubsystem(ctx, "proxy:"+targetAddress); err != nil {
		// read the stderr output from the failed SSH session and append
		// it to the end of our own message:
		serverErrorMsg, _ := io.ReadAll(sessionError)
		return nil, trace.ConnectionProblem(err, "failed connecting to host %s: %v. %v", targetAddress, serverErrorMsg, err)
	}

	return conn, nil
}
