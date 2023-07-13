/*
Copyright 2020-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	ggzip "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client/okta"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	userpreferencespb "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils"
)

func init() {
	// gzip is used for gRPC auditStream compression. SetLevel changes the
	// compression level, must be called in initialization, and is not thread safe.
	if err := ggzip.SetLevel(gzip.BestSpeed); err != nil {
		panic(err)
	}
}

// AuthServiceClient keeps the interfaces implemented by the auth service.
type AuthServiceClient struct {
	proto.AuthServiceClient
	assist.AssistServiceClient
	auditlogpb.AuditLogServiceClient
	userpreferencespb.UserPreferencesServiceClient
}

// Client is a gRPC Client that connects to a Teleport Auth server either
// locally or over ssh through a Teleport web proxy or tunnel proxy.
//
// This client can be used to cover a variety of Teleport use cases,
// such as programmatically handling access requests, integrating
// with external tools, or dynamically configuring Teleport.
type Client struct {
	// c contains configuration values for the client.
	c Config
	// tlsConfig is the *tls.Config for a successfully connected client.
	tlsConfig *tls.Config
	// dialer is the ContextDialer for a successfully connected client.
	dialer ContextDialer
	// conn is a grpc connection to the auth server.
	conn *grpc.ClientConn
	// grpc is the gRPC client specification for the auth server.
	grpc AuthServiceClient
	// JoinServiceClient is a client for the JoinService, which runs on both the
	// auth and proxy.
	*JoinServiceClient
	// closedFlag is set to indicate that the connection is closed.
	// It's a pointer to allow the Client struct to be copied.
	closedFlag *int32
}

// New creates a new Client with an open connection to a Teleport server.
//
// New will try to open a connection with all combinations of addresses and credentials.
// The first successful connection to a server will be used, or an aggregated error will
// be returned if all combinations fail.
//
// cfg.Credentials must be non-empty. One of cfg.Addrs and cfg.Dialer must be non-empty,
// unless LoadProfile is used to fetch Credentials and load a web proxy dialer.
//
// See the example below for usage.
func New(ctx context.Context, cfg Config) (clt *Client, err error) {
	if err = cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// If cfg.DialInBackground is true, only a single connection is attempted.
	// This option is primarily meant for internal use where the client has
	// direct access to server values that guarantee a successful connection.
	if cfg.DialInBackground {
		return connectInBackground(ctx, cfg)
	}
	return connect(ctx, cfg)
}

// NewTracingClient creates a new tracing.Client that will forward spans to the
// connected Teleport server. See New for details on how the connection it
// established.
func NewTracingClient(ctx context.Context, cfg Config) (*tracing.Client, error) {
	clt, err := New(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return tracing.NewClient(clt.GetConnection()), nil
}

// NewOktaClient creates a new Okta client for managing Okta resources.
func NewOktaClient(ctx context.Context, cfg Config) (*okta.Client, error) {
	clt, err := New(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return okta.NewClient(oktapb.NewOktaServiceClient(clt.GetConnection())), nil
}

// newClient constructs a new client.
func newClient(cfg Config, dialer ContextDialer, tlsConfig *tls.Config) *Client {
	return &Client{
		c:          cfg,
		dialer:     dialer,
		tlsConfig:  ConfigureALPN(tlsConfig, cfg.ALPNSNIAuthDialClusterName),
		closedFlag: new(int32),
	}
}

// connectInBackground connects the client to the server in the background.
//
// The client will use the first credentials and the given dialer.
//
// If no dialer is given, the first address will be used. If no
// ALPNSNIAuthDialClusterName is given, this address must be an auth server
// address.
//
// If ALPNSNIAuthDialClusterName is given, the address is expected to be a web
// proxy address and the client will connect auth through the web proxy server
// using TLS Routing.
func connectInBackground(ctx context.Context, cfg Config) (*Client, error) {
	tlsConfig, err := cfg.Credentials[0].TLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Dialer != nil {
		return dialerConnect(ctx, connectParams{
			cfg:       cfg,
			tlsConfig: tlsConfig,
			dialer:    cfg.Dialer,
		})
	} else if len(cfg.Addrs) != 0 {
		return authConnect(ctx, connectParams{
			cfg:       cfg,
			tlsConfig: tlsConfig,
			addr:      cfg.Addrs[0],
		})
	}
	return nil, trace.BadParameter("must provide Dialer or Addrs in config")
}

// connect connects the client to the server using the Credentials and
// Dialer/Addresses provided in the client's config. Multiple goroutines are started
// to make dial attempts with different combinations of dialers and credentials. The
// first client to successfully connect is used to populate the client's connection
// attributes. If none successfully connect, an aggregated error is returned.
func connect(ctx context.Context, cfg Config) (*Client, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup

	// sendError is used to send errors to errChan with context.
	errChan := make(chan error)
	sendError := func(err error) {
		errChan <- trace.Wrap(err)
	}

	// syncConnect is used to concurrently create multiple clients
	// with the different combination of connection parameters.
	// The first successful client to be sent to cltChan will be returned.
	cltChan := make(chan *Client)
	syncConnect := func(ctx context.Context, connect connectFunc, params connectParams) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			clt, err := connect(ctx, params)
			if err != nil {
				sendError(trace.Wrap(err))
				return
			}
			select {
			case cltChan <- clt:
			case <-ctx.Done():
				clt.Close()
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		// Connect with provided credentials.
		for _, creds := range cfg.Credentials {
			tlsConfig, err := creds.TLSConfig()
			if err != nil {
				sendError(trace.Wrap(err))
				continue
			}

			sshConfig, err := creds.SSHClientConfig()
			if err != nil && !trace.IsNotImplemented(err) {
				sendError(trace.Wrap(err))
				continue
			}

			// Connect with dialer provided in config.
			if cfg.Dialer != nil {
				syncConnect(ctx, dialerConnect, connectParams{
					cfg:       cfg,
					tlsConfig: tlsConfig,
				})
			}

			// Connect with dialer provided in creds.
			if dialer, err := creds.Dialer(cfg); err != nil {
				if !trace.IsNotImplemented(err) {
					sendError(trace.Wrap(err, "failed to retrieve dialer from creds of type %T", creds))
				}
			} else {
				syncConnect(ctx, dialerConnect, connectParams{
					cfg:       cfg,
					tlsConfig: tlsConfig,
					dialer:    dialer,
				})
			}

			// Attempt to connect to each address as Auth, Proxy, Tunnel and TLS Routing.
			for _, addr := range cfg.Addrs {
				syncConnect(ctx, authConnect, connectParams{
					cfg:       cfg,
					tlsConfig: tlsConfig,
					addr:      addr,
				})
				if sshConfig != nil {
					for _, cf := range []connectFunc{proxyConnect, tunnelConnect, tlsRoutingConnect, tlsRoutingWithConnUpgradeConnect} {
						syncConnect(ctx, cf, connectParams{
							cfg:       cfg,
							tlsConfig: tlsConfig,
							sshConfig: sshConfig,
							addr:      addr,
						})
					}
				}
			}
		}
	}()

	// Start goroutine to wait for wait group.
	go func() {
		wg.Wait()
		// Close errChan to return errors.
		close(errChan)
	}()

	var errs []error
Outer:
	for {
		select {
		// Use the first client to successfully connect in syncConnect.
		case clt := <-cltChan:
			go func() {
				for range errChan {
				}
			}()
			return clt, nil
		case err, ok := <-errChan:
			if !ok {
				break Outer
			}
			// Add a new line to make errs human readable.
			errs = append(errs, trace.Wrap(err, ""))
		}
	}

	// errChan is closed, return errors.
	if len(errs) == 0 {
		if len(cfg.Addrs) == 0 && cfg.Dialer == nil {
			// Some credentials don't require these fields. If no errors propagate, then they need to provide these fields.
			return nil, trace.BadParameter("no connection methods found, try providing Dialer or Addrs in config")
		}
		// This case should never be reached with config validation and above case.
		return nil, trace.Errorf("no connection methods found")
	}
	if ctx.Err() != nil {
		errs = append(errs, trace.Wrap(ctx.Err()))
	}
	return nil, trace.Wrap(trace.NewAggregate(errs...), "all connection methods failed")
}

type (
	connectFunc   func(ctx context.Context, params connectParams) (*Client, error)
	connectParams struct {
		cfg       Config
		addr      string
		tlsConfig *tls.Config
		dialer    ContextDialer
		sshConfig *ssh.ClientConfig
	}
)

// authConnect connects to the Teleport Auth Server directly or through Proxy.
func authConnect(ctx context.Context, params connectParams) (*Client, error) {
	dialer := NewDialer(ctx, params.cfg.KeepAlivePeriod, params.cfg.DialTimeout,
		WithInsecureSkipVerify(params.cfg.InsecureAddressDiscovery),
		WithALPNConnUpgrade(params.cfg.ALPNConnUpgradeRequired),
		WithALPNConnUpgradePing(true), // Use Ping protocol for long-lived connections.
		WithPROXYHeaderGetter(params.cfg.PROXYHeaderGetter),
	)

	clt := newClient(params.cfg, dialer, params.tlsConfig)
	if err := clt.dialGRPC(ctx, params.addr); err != nil {
		return nil, trace.Wrap(err, "failed to connect to addr %v as an auth server", params.addr)
	}
	return clt, nil
}

// tunnelConnect connects to the Teleport Auth Server through the proxy's reverse tunnel.
func tunnelConnect(ctx context.Context, params connectParams) (*Client, error) {
	if params.sshConfig == nil {
		return nil, trace.BadParameter("must provide ssh client config")
	}
	dialer := newTunnelDialer(*params.sshConfig, params.cfg.KeepAlivePeriod, params.cfg.DialTimeout, WithInsecureSkipVerify(params.cfg.InsecureAddressDiscovery))
	clt := newClient(params.cfg, dialer, params.tlsConfig)
	if err := clt.dialGRPC(ctx, params.addr); err != nil {
		return nil, trace.Wrap(err, "failed to connect to addr %v as a reverse tunnel proxy", params.addr)
	}
	return clt, nil
}

// proxyConnect connects to the Teleport Auth Server through the proxy.
func proxyConnect(ctx context.Context, params connectParams) (*Client, error) {
	if params.sshConfig == nil {
		return nil, trace.BadParameter("must provide ssh client config")
	}
	dialer := NewProxyDialer(*params.sshConfig, params.cfg.KeepAlivePeriod, params.cfg.DialTimeout, params.addr, params.cfg.InsecureAddressDiscovery, WithInsecureSkipVerify(params.cfg.InsecureAddressDiscovery))
	clt := newClient(params.cfg, dialer, params.tlsConfig)
	if err := clt.dialGRPC(ctx, params.addr); err != nil {
		return nil, trace.Wrap(err, "failed to connect to addr %v as a web proxy", params.addr)
	}
	return clt, nil
}

// tlsRoutingConnect connects to the Teleport Auth Server through the proxy using TLS Routing.
func tlsRoutingConnect(ctx context.Context, params connectParams) (*Client, error) {
	if params.sshConfig == nil {
		return nil, trace.BadParameter("must provide ssh client config")
	}
	dialer := newTLSRoutingTunnelDialer(*params.sshConfig, params.cfg.KeepAlivePeriod, params.cfg.DialTimeout, params.addr, params.cfg.InsecureAddressDiscovery)
	clt := newClient(params.cfg, dialer, params.tlsConfig)
	if err := clt.dialGRPC(ctx, params.addr); err != nil {
		return nil, trace.Wrap(err, "failed to connect to addr %v with TLS Routing dialer", params.addr)
	}
	return clt, nil
}

// tlsRoutingWithConnUpgradeConnect connects to the Teleport Auth Server
// through the proxy using TLS Routing with ALPN connection upgrade.
func tlsRoutingWithConnUpgradeConnect(ctx context.Context, params connectParams) (*Client, error) {
	if params.sshConfig == nil {
		return nil, trace.BadParameter("must provide ssh client config")
	}
	dialer := newTLSRoutingWithConnUpgradeDialer(*params.sshConfig, params)
	clt := newClient(params.cfg, dialer, params.tlsConfig)
	if err := clt.dialGRPC(ctx, params.addr); err != nil {
		return nil, trace.Wrap(err, "failed to connect to addr %v with TLS Routing with ALPN connection upgrade dialer", params.addr)
	}
	return clt, nil
}

// dialerConnect connects to the Teleport Auth Server through a custom dialer.
// The dialer must provide the address in a custom ContextDialerFunc function.
func dialerConnect(ctx context.Context, params connectParams) (*Client, error) {
	if params.dialer == nil {
		if params.cfg.Dialer == nil {
			return nil, trace.BadParameter("must provide dialer to connectParams.dialer or params.cfg.Dialer")
		}
		params.dialer = params.cfg.Dialer
	}
	clt := newClient(params.cfg, params.dialer, params.tlsConfig)
	// Since the client uses a custom dialer to connect to the server and SNI
	// is used for the TLS handshake, the address dialed here is arbitrary.
	if err := clt.dialGRPC(ctx, constants.APIDomain); err != nil {
		return nil, trace.Wrap(err, "failed to connect using pre-defined dialer")
	}
	return clt, nil
}

// dialGRPC dials a connection between server and client.
func (c *Client) dialGRPC(ctx context.Context, addr string) error {
	dialContext, cancel := context.WithTimeout(ctx, c.c.DialTimeout)
	defer cancel()

	cb, err := breaker.New(c.c.CircuitBreakerConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	var dialOpts []grpc.DialOption
	dialOpts = append(dialOpts, grpc.WithContextDialer(c.grpcDialer()))
	dialOpts = append(dialOpts,
		grpc.WithChainUnaryInterceptor(
			otelgrpc.UnaryClientInterceptor(),
			metadata.UnaryClientInterceptor,
			breaker.UnaryClientInterceptor(cb),
		),
		grpc.WithChainStreamInterceptor(
			otelgrpc.StreamClientInterceptor(),
			metadata.StreamClientInterceptor,
			breaker.StreamClientInterceptor(cb),
		),
	)
	// Only set transportCredentials if tlsConfig is set. This makes it possible
	// to explicitly provide grpc.WithTransportCredentials(insecure.NewCredentials())
	// in the client's dial options.
	if c.tlsConfig != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(c.tlsConfig)))
	}
	// must come last, otherwise provided opts may get clobbered by defaults above
	dialOpts = append(dialOpts, c.c.DialOpts...)

	conn, err := grpc.DialContext(dialContext, addr, dialOpts...)
	if err != nil {
		return trace.Wrap(err)
	}

	c.conn = conn
	c.grpc = AuthServiceClient{
		AuthServiceClient:            proto.NewAuthServiceClient(c.conn),
		AssistServiceClient:          assist.NewAssistServiceClient(c.conn),
		AuditLogServiceClient:        auditlogpb.NewAuditLogServiceClient(c.conn),
		UserPreferencesServiceClient: userpreferencespb.NewUserPreferencesServiceClient(c.conn),
	}
	c.JoinServiceClient = NewJoinServiceClient(proto.NewJoinServiceClient(c.conn))

	return nil
}

// ConfigureALPN configures ALPN SNI cluster routing information in TLS settings allowing for
// allowing to dial auth service through Teleport Proxy directly without using SSH Tunnels.
func ConfigureALPN(tlsConfig *tls.Config, clusterName string) *tls.Config {
	if tlsConfig == nil {
		return nil
	}
	if clusterName == "" {
		return tlsConfig
	}
	out := tlsConfig.Clone()
	routeInfo := fmt.Sprintf("%s%s", constants.ALPNSNIAuthProtocol, utils.EncodeClusterName(clusterName))
	out.NextProtos = append([]string{routeInfo}, out.NextProtos...)
	return out
}

// grpcDialer wraps the client's dialer with a grpcDialer.
func (c *Client) grpcDialer() func(ctx context.Context, addr string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		if c.isClosed() {
			return nil, trace.ConnectionProblem(nil, "client is closed")
		}
		conn, err := c.dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, trace.ConnectionProblem(err, "failed to dial: %v", err)
		}
		return conn, nil
	}
}

// waitForConnectionReady waits for the client's grpc connection finish dialing, returning an error
// if the ctx is canceled or the client's gRPC connection enters an unexpected state. This can be used
// alongside the DialInBackground client config option to wait until background dialing has completed.
func (c *Client) waitForConnectionReady(ctx context.Context) error {
	for {
		if c.conn == nil {
			return errors.New("conn was closed")
		}
		switch state := c.conn.GetState(); state {
		case connectivity.Ready:
			return nil
		case connectivity.TransientFailure, connectivity.Connecting, connectivity.Idle:
			// Wait for expected state transitions. For details about grpc.ClientConn state changes
			// see https://github.com/grpc/grpc/blob/master/doc/connectivity-semantics-and-api.md
			if !c.conn.WaitForStateChange(ctx, state) {
				// ctx canceled
				return trace.Wrap(ctx.Err())
			}
		case connectivity.Shutdown:
			return trace.Errorf("client gRPC connection entered an unexpected state: %v", state)
		}
	}
}

// Config contains configuration of the client
type Config struct {
	// Addrs is a list of teleport auth/proxy server addresses to dial.
	Addrs []string
	// Credentials are a list of credentials to use when attempting
	// to connect to the server.
	Credentials []Credentials
	// Dialer is a custom dialer used to dial a server. The Dialer should
	// have custom logic to provide an address to the dialer. If set, Dialer
	// takes precedence over all other connection options.
	Dialer ContextDialer
	// DialOpts define options for dialing the client connection.
	DialOpts []grpc.DialOption
	// DialInBackground specifies to dial the connection in the background
	// rather than blocking until the connection is up. A predefined Dialer
	// or an auth server address must be provided.
	DialInBackground bool
	// DialTimeout defines how long to attempt dialing before timing out.
	DialTimeout time.Duration
	// KeepAlivePeriod defines period between keep alives.
	KeepAlivePeriod time.Duration
	// KeepAliveCount specifies the amount of missed keep alives
	// to wait for before declaring the connection as broken.
	KeepAliveCount int
	// The web proxy uses a self-signed TLS certificate by default, which
	// requires this field to be set. If the web proxy was provided with
	// signed TLS certificates, this field should not be set.
	InsecureAddressDiscovery bool
	// ALPNSNIAuthDialClusterName if present the client will include ALPN SNI routing information in TLS Hello message
	// allowing to dial auth service through Teleport Proxy directly without using SSH Tunnels.
	ALPNSNIAuthDialClusterName string
	// CircuitBreakerConfig defines how the circuit breaker should behave.
	CircuitBreakerConfig breaker.Config
	// Context is the base context to use for dialing. If not provided context.Background is used
	Context context.Context
	// ALPNConnUpgradeRequired indicates that ALPN connection upgrades are
	// required for making TLS Routing requests.
	//
	// In DialInBackground mode without a Dialer, a valid value must be
	// provided as it's assumed that the caller knows the context if connection
	// upgrades are required for TLS Routing.
	//
	// In default mode, this value is optional as some of the connect methods
	// will perform necessary tests to decide if connection upgrade is
	// required.
	ALPNConnUpgradeRequired bool
	// PROXYHeaderGetter returns signed PROXY header that is sent to allow Proxy to propagate client's real IP to the
	// auth server from the Proxy's web server, when we create user's client for the web session.
	PROXYHeaderGetter PROXYHeaderGetter
}

// CheckAndSetDefaults checks and sets default config values.
func (c *Config) CheckAndSetDefaults() error {
	if len(c.Credentials) == 0 {
		return trace.BadParameter("missing connection credentials")
	}

	if c.KeepAlivePeriod == 0 {
		c.KeepAlivePeriod = defaults.ServerKeepAliveTTL()
	}
	if c.KeepAliveCount == 0 {
		c.KeepAliveCount = defaults.KeepAliveCountMax
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = defaults.DefaultIOTimeout
	}
	if c.CircuitBreakerConfig.Trip == nil || c.CircuitBreakerConfig.IsSuccessful == nil {
		c.CircuitBreakerConfig = breaker.DefaultBreakerConfig(clockwork.NewRealClock())
	}

	if c.Context == nil {
		c.Context = context.Background()
	}

	c.DialOpts = append(c.DialOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                c.KeepAlivePeriod,
		Timeout:             c.KeepAlivePeriod * time.Duration(c.KeepAliveCount),
		PermitWithoutStream: true,
	}))
	if !c.DialInBackground {
		c.DialOpts = append(
			c.DialOpts,
			// Provides additional feedback on connection failure, otherwise,
			// users will only receive a `context deadline exceeded` error when
			// c.DialInBackground == false.
			//
			// grpc.WithReturnConnectionError implies grpc.WithBlock which is
			// necessary for connection route selection to work properly.
			grpc.WithReturnConnectionError(),
		)
	}
	return nil
}

// Config returns the tls.Config the client connected with.
func (c *Client) Config() *tls.Config {
	return c.tlsConfig
}

// Dialer returns the ContextDialer the client connected with.
func (c *Client) Dialer() ContextDialer {
	return c.dialer
}

// GetConnection returns GRPC connection.
func (c *Client) GetConnection() *grpc.ClientConn {
	return c.conn
}

// Close closes the Client connection to the auth server.
func (c *Client) Close() error {
	if c.setClosed() && c.conn != nil {
		return trace.Wrap(c.conn.Close())
	}
	return nil
}

// isClosed returns whether the client is marked as closed.
func (c *Client) isClosed() bool {
	return atomic.LoadInt32(c.closedFlag) == 1
}

// setClosed marks the client as closed and returns true if it was open.
func (c *Client) setClosed() bool {
	return atomic.CompareAndSwapInt32(c.closedFlag, 0, 1)
}

// DevicesClient returns an unadorned Device Trust client, using the underlying
// Auth gRPC connection.
// Clients connecting to non-Enterprise clusters, or older Teleport versions,
// still get a devices client when calling this method, but all RPCs will return
// "not implemented" errors (as per the default gRPC behavior).
func (c *Client) DevicesClient() devicepb.DeviceTrustServiceClient {
	return devicepb.NewDeviceTrustServiceClient(c.conn)
}

// CreateDeviceResource creates a device using its resource representation.
// Prefer using [DevicesClient] directly if you can.
func (c *Client) CreateDeviceResource(ctx context.Context, res *types.DeviceV1) (*types.DeviceV1, error) {
	dev, err := types.DeviceFromResource(res)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := c.DevicesClient().CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device:           dev,
		CreateAsResource: true,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return types.DeviceToResource(created), nil
}

// DeleteDeviceResource deletes a device using its ID (either devicepb.Device.Id
// or its Metadata.Name).
// Prefer using [DevicesClient] directly if you can.
func (c *Client) DeleteDeviceResource(ctx context.Context, id string) error {
	_, err := c.DevicesClient().DeleteDevice(ctx, &devicepb.DeleteDeviceRequest{
		DeviceId: id,
	})
	return trail.FromGRPC(err)
}

// GetDeviceResource reads a device using its ID (either devicepb.Device.Id
// or its Metadata.Name).
// Prefer using [DevicesClient] directly if you can.
func (c *Client) GetDeviceResource(ctx context.Context, id string) (*types.DeviceV1, error) {
	dev, err := c.DevicesClient().GetDevice(ctx, &devicepb.GetDeviceRequest{
		DeviceId: id,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return types.DeviceToResource(dev), nil
}

// UpsertDeviceResource creates or updates a device using its resource
// representation.
// Prefer using [DevicesClient] directly if you can.
func (c *Client) UpsertDeviceResource(ctx context.Context, res *types.DeviceV1) (*types.DeviceV1, error) {
	dev, err := types.DeviceFromResource(res)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := c.DevicesClient().UpsertDevice(ctx, &devicepb.UpsertDeviceRequest{
		Device:           dev,
		CreateAsResource: true,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return types.DeviceToResource(upserted), nil
}

// LoginRuleClient returns an unadorned Login Rule client, using the underlying
// Auth gRPC connection.
// Clients connecting to non-Enterprise clusters, or older Teleport versions,
// still get a login rule client when calling this method, but all RPCs will
// return "not implemented" errors (as per the default gRPC behavior).
func (c *Client) LoginRuleClient() loginrulepb.LoginRuleServiceClient {
	return loginrulepb.NewLoginRuleServiceClient(c.conn)
}

// SAMLIdPClient returns an unadorned SAML IdP client, using the underlying
// Auth gRPC connection.
// Clients connecting to non-Enterprise clusters, or older Teleport versions,
// still get a SAML IdP client when calling this method, but all RPCs will
// return "not implemented" errors (as per the default gRPC behavior).
func (c *Client) SAMLIdPClient() samlidppb.SAMLIdPServiceClient {
	return samlidppb.NewSAMLIdPServiceClient(c.conn)
}

// TrustClient returns an unadorned Trust client, using the underlying
// Auth gRPC connection.
func (c *Client) TrustClient() trustpb.TrustServiceClient {
	return trustpb.NewTrustServiceClient(c.conn)
}

// EmbeddingClient returns an unadorned Embedding client, using the underlying
// Auth gRPC connection.
func (c *Client) EmbeddingClient() assist.AssistEmbeddingServiceClient {
	return assist.NewAssistEmbeddingServiceClient(c.conn)
}

// Ping gets basic info about the auth server.
func (c *Client) Ping(ctx context.Context) (proto.PingResponse, error) {
	rsp, err := c.grpc.Ping(ctx, &proto.PingRequest{})
	if err != nil {
		return proto.PingResponse{}, trail.FromGRPC(err)
	}
	return *rsp, nil
}

// UpdateRemoteCluster updates remote cluster from the specified value.
func (c *Client) UpdateRemoteCluster(ctx context.Context, rc types.RemoteCluster) error {
	rcV3, ok := rc.(*types.RemoteClusterV3)
	if !ok {
		return trace.BadParameter("unsupported remote cluster type %T", rcV3)
	}

	_, err := c.grpc.UpdateRemoteCluster(ctx, rcV3)
	return trail.FromGRPC(err)
}

// CreateUser creates a new user from the specified descriptor.
func (c *Client) CreateUser(ctx context.Context, user types.User) error {
	userV2, ok := user.(*types.UserV2)
	if !ok {
		return trace.BadParameter("unsupported user type %T", user)
	}

	_, err := c.grpc.CreateUser(ctx, userV2)
	return trail.FromGRPC(err)
}

// UpdateUser updates an existing user in a backend.
func (c *Client) UpdateUser(ctx context.Context, user types.User) error {
	userV2, ok := user.(*types.UserV2)
	if !ok {
		return trace.BadParameter("unsupported user type %T", user)
	}

	_, err := c.grpc.UpdateUser(ctx, userV2)
	return trail.FromGRPC(err)
}

// GetUser returns a list of usernames registered in the system.
// withSecrets controls whether authentication details are returned.
func (c *Client) GetUser(name string, withSecrets bool) (types.User, error) {
	if name == "" {
		return nil, trace.BadParameter("missing username")
	}
	user, err := c.grpc.GetUser(context.TODO(), &proto.GetUserRequest{
		Name:        name,
		WithSecrets: withSecrets,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return user, nil
}

// GetCurrentUser returns current user as seen by the server.
// Useful especially in the context of remote clusters which perform role and trait mapping.
func (c *Client) GetCurrentUser(ctx context.Context) (types.User, error) {
	currentUser, err := c.grpc.GetCurrentUser(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return currentUser, nil
}

// GetCurrentUserRoles returns current user's roles.
func (c *Client) GetCurrentUserRoles(ctx context.Context) ([]types.Role, error) {
	stream, err := c.grpc.GetCurrentUserRoles(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	var roles []types.Role
	for role, err := stream.Recv(); err != io.EOF; role, err = stream.Recv() {
		if err != nil {
			return nil, trail.FromGRPC(err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// GetUsers returns a list of users.
// withSecrets controls whether authentication details are returned.
func (c *Client) GetUsers(withSecrets bool) ([]types.User, error) {
	stream, err := c.grpc.GetUsers(context.TODO(), &proto.GetUsersRequest{
		WithSecrets: withSecrets,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	var users []types.User
	for user, err := stream.Recv(); err != io.EOF; user, err = stream.Recv() {
		if err != nil {
			return nil, trail.FromGRPC(err)
		}
		users = append(users, user)
	}
	return users, nil
}

// DeleteUser deletes a user by name.
func (c *Client) DeleteUser(ctx context.Context, user string) error {
	req := &proto.DeleteUserRequest{Name: user}
	_, err := c.grpc.DeleteUser(ctx, req)
	return trail.FromGRPC(err)
}

// GenerateUserCerts takes the public key in the OpenSSH `authorized_keys` plain
// text format, signs it using User Certificate Authority signing key and
// returns the resulting certificates.
func (c *Client) GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
	certs, err := c.grpc.GenerateUserCerts(ctx, &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return certs, nil
}

// GenerateHostCerts generates host certificates.
func (c *Client) GenerateHostCerts(ctx context.Context, req *proto.HostCertsRequest) (*proto.Certs, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := c.grpc.GenerateHostCerts(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return certs, nil
}

// GenerateOpenSSHCert signs a SSH certificate that can be used
// to connect to Agentless nodes.
func (c *Client) GenerateOpenSSHCert(ctx context.Context, req *proto.OpenSSHCertRequest) (*proto.OpenSSHCert, error) {
	cert, err := c.grpc.GenerateOpenSSHCert(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return cert, nil
}

// UnstableAssertSystemRole is not a stable part of the public API.  Used by older
// instances to prove that they hold a given system role.
//
// DELETE IN: 11.0 (server side method should continue to exist until 12.0 for back-compat reasons,
// but v11 clients should no longer need this method)
func (c *Client) UnstableAssertSystemRole(ctx context.Context, req proto.UnstableSystemRoleAssertion) error {
	_, err := c.grpc.UnstableAssertSystemRole(ctx, &req)
	return trail.FromGRPC(err)
}

// EmitAuditEvent sends an auditable event to the auth server.
func (c *Client) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	grpcEvent, err := events.ToOneOf(event)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.grpc.EmitAuditEvent(ctx, grpcEvent)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// GetResetPasswordToken returns a reset password token for the specified tokenID.
func (c *Client) GetResetPasswordToken(ctx context.Context, tokenID string) (types.UserToken, error) {
	token, err := c.grpc.GetResetPasswordToken(ctx, &proto.GetResetPasswordTokenRequest{
		TokenID: tokenID,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return token, nil
}

// CreateResetPasswordToken creates reset password token.
func (c *Client) CreateResetPasswordToken(ctx context.Context, req *proto.CreateResetPasswordTokenRequest) (types.UserToken, error) {
	token, err := c.grpc.CreateResetPasswordToken(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return token, nil
}

// CreateBot creates a new bot from the specified descriptor.
func (c *Client) CreateBot(ctx context.Context, req *proto.CreateBotRequest) (*proto.CreateBotResponse, error) {
	response, err := c.grpc.CreateBot(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return response, nil
}

// DeleteBot deletes a bot and associated resources.
func (c *Client) DeleteBot(ctx context.Context, botName string) error {
	_, err := c.grpc.DeleteBot(ctx, &proto.DeleteBotRequest{
		Name: botName,
	})
	return trail.FromGRPC(err)
}

// GetBotUsers fetches all bot users.
func (c *Client) GetBotUsers(ctx context.Context) ([]types.User, error) {
	stream, err := c.grpc.GetBotUsers(ctx, &proto.GetBotUsersRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	var users []types.User
	for user, err := stream.Recv(); err != io.EOF; user, err = stream.Recv() {
		if err != nil {
			return nil, trail.FromGRPC(err)
		}
		users = append(users, user)
	}
	return users, nil
}

// GetAccessRequests retrieves a list of all access requests matching the provided filter.
func (c *Client) GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
	stream, err := c.grpc.GetAccessRequestsV2(ctx, &filter)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	var reqs []types.AccessRequest
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, trail.FromGRPC(err)
		}
		reqs = append(reqs, req)
	}

	return reqs, nil
}

// CreateAccessRequest registers a new access request with the auth server.
func (c *Client) CreateAccessRequest(ctx context.Context, req types.AccessRequest) error {
	r, ok := req.(*types.AccessRequestV3)
	if !ok {
		return trace.BadParameter("unexpected access request type %T", req)
	}
	_, err := c.grpc.CreateAccessRequest(ctx, r)
	return trail.FromGRPC(err)
}

// DeleteAccessRequest deletes an access request.
func (c *Client) DeleteAccessRequest(ctx context.Context, reqID string) error {
	_, err := c.grpc.DeleteAccessRequest(ctx, &proto.RequestID{ID: reqID})
	return trail.FromGRPC(err)
}

// SetAccessRequestState updates the state of an existing access request.
func (c *Client) SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) error {
	setter := proto.RequestStateSetter{
		ID:          params.RequestID,
		State:       params.State,
		Reason:      params.Reason,
		Annotations: params.Annotations,
		Roles:       params.Roles,
	}
	if d := utils.GetDelegator(ctx); d != "" {
		setter.Delegator = d
	}
	_, err := c.grpc.SetAccessRequestState(ctx, &setter)
	return trail.FromGRPC(err)
}

// SubmitAccessReview applies a review to a request and returns the post-application state.
func (c *Client) SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error) {
	req, err := c.grpc.SubmitAccessReview(ctx, &params)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return req, nil
}

// GetAccessCapabilities requests the access capabilities of a user.
func (c *Client) GetAccessCapabilities(ctx context.Context, req types.AccessCapabilitiesRequest) (*types.AccessCapabilities, error) {
	caps, err := c.grpc.GetAccessCapabilities(ctx, &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return caps, nil
}

// GetPluginData loads all plugin data matching the supplied filter.
func (c *Client) GetPluginData(ctx context.Context, filter types.PluginDataFilter) ([]types.PluginData, error) {
	seq, err := c.grpc.GetPluginData(ctx, &filter)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	data := make([]types.PluginData, 0, len(seq.PluginData))
	for _, d := range seq.PluginData {
		data = append(data, d)
	}
	return data, nil
}

// UpdatePluginData updates a per-resource PluginData entry.
func (c *Client) UpdatePluginData(ctx context.Context, params types.PluginDataUpdateParams) error {
	_, err := c.grpc.UpdatePluginData(ctx, &params)
	return trail.FromGRPC(err)
}

// AcquireSemaphore acquires lease with requested resources from semaphore.
func (c *Client) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	lease, err := c.grpc.AcquireSemaphore(ctx, &params)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return lease, nil
}

// KeepAliveSemaphoreLease updates semaphore lease.
func (c *Client) KeepAliveSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	_, err := c.grpc.KeepAliveSemaphoreLease(ctx, &lease)
	return trail.FromGRPC(err)
}

// CancelSemaphoreLease cancels semaphore lease early.
func (c *Client) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	_, err := c.grpc.CancelSemaphoreLease(ctx, &lease)
	return trail.FromGRPC(err)
}

// GetSemaphores returns a list of all semaphores matching the supplied filter.
func (c *Client) GetSemaphores(ctx context.Context, filter types.SemaphoreFilter) ([]types.Semaphore, error) {
	rsp, err := c.grpc.GetSemaphores(ctx, &filter)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	sems := make([]types.Semaphore, 0, len(rsp.Semaphores))
	for _, s := range rsp.Semaphores {
		sems = append(sems, s)
	}
	return sems, nil
}

// DeleteSemaphore deletes a semaphore matching the supplied filter.
func (c *Client) DeleteSemaphore(ctx context.Context, filter types.SemaphoreFilter) error {
	_, err := c.grpc.DeleteSemaphore(ctx, &filter)
	return trail.FromGRPC(err)
}

// GetKubernetesServers returns the list of kubernetes servers registered in the
// cluster.
func (c *Client) GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	servers, err := GetAllResources[types.KubeServer](ctx, c, &proto.ListResourcesRequest{
		Namespace:    defaults.Namespace,
		ResourceType: types.KindKubeServer,
	})
	return servers, trace.Wrap(err)
}

// DeleteKubernetesServer deletes a named kubernetes server.
func (c *Client) DeleteKubernetesServer(ctx context.Context, hostID, name string) error {
	_, err := c.grpc.DeleteKubernetesServer(ctx, &proto.DeleteKubernetesServerRequest{
		HostID: hostID,
		Name:   name,
	})
	return trail.FromGRPC(err)
}

// DeleteAllKubernetesServers deletes all registered kubernetes servers.
func (c *Client) DeleteAllKubernetesServers(ctx context.Context) error {
	_, err := c.grpc.DeleteAllKubernetesServers(ctx, &proto.DeleteAllKubernetesServersRequest{})
	return trail.FromGRPC(err)
}

// UpsertKubernetesServer is used by kubernetes services to report their presence
// to other auth servers in form of heartbeat expiring after ttl period.
func (c *Client) UpsertKubernetesServer(ctx context.Context, s types.KubeServer) (*types.KeepAlive, error) {
	server, ok := s.(*types.KubernetesServerV3)
	if !ok {
		return nil, trace.BadParameter("invalid type %T, expected *types.KubernetesServerV3", server)
	}
	keepAlive, err := c.grpc.UpsertKubernetesServer(ctx, &proto.UpsertKubernetesServerRequest{Server: server})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return keepAlive, nil
}

// GetApplicationServers returns all registered application servers.
func (c *Client) GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error) {
	servers, err := GetAllResources[types.AppServer](ctx, c, &proto.ListResourcesRequest{
		Namespace:    namespace,
		ResourceType: types.KindAppServer,
	})
	return servers, trail.FromGRPC(err)
}

// UpsertApplicationServer registers an application server.
func (c *Client) UpsertApplicationServer(ctx context.Context, server types.AppServer) (*types.KeepAlive, error) {
	s, ok := server.(*types.AppServerV3)
	if !ok {
		return nil, trace.BadParameter("invalid type %T", server)
	}
	keepAlive, err := c.grpc.UpsertApplicationServer(ctx, &proto.UpsertApplicationServerRequest{
		Server: s,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return keepAlive, nil
}

// DeleteApplicationServer removes specified application server.
func (c *Client) DeleteApplicationServer(ctx context.Context, namespace, hostID, name string) error {
	_, err := c.grpc.DeleteApplicationServer(ctx, &proto.DeleteApplicationServerRequest{
		Namespace: namespace,
		HostID:    hostID,
		Name:      name,
	})
	return trail.FromGRPC(err)
}

// DeleteAllApplicationServers removes all registered application servers.
func (c *Client) DeleteAllApplicationServers(ctx context.Context, namespace string) error {
	_, err := c.grpc.DeleteAllApplicationServers(ctx, &proto.DeleteAllApplicationServersRequest{
		Namespace: namespace,
	})
	return trail.FromGRPC(err)
}

// GetAppSession gets an application web session.
func (c *Client) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	resp, err := c.grpc.GetAppSession(ctx, &proto.GetAppSessionRequest{
		SessionID: req.SessionID,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return resp.GetSession(), nil
}

// GetAppSessions gets all application web sessions.
func (c *Client) GetAppSessions(ctx context.Context) ([]types.WebSession, error) {
	var (
		nextToken string
		sessions  []types.WebSession
	)

	// Leverages ListAppSessions instead of GetAppSessions to prevent
	// the server from having to send all sessions in a single message.
	// If there are enough sessions it can cause the max message size to be
	// exceeded.
	for {
		webSessions, token, err := c.ListAppSessions(ctx, defaults.DefaultChunkSize, nextToken, "")
		if err != nil {
			return nil, trail.FromGRPC(err)
		}

		sessions = append(sessions, webSessions...)
		if token == "" {
			break
		}

		nextToken = token
	}

	return sessions, nil
}

// ListAppSessions gets a paginated list of application web sessions.
func (c *Client) ListAppSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error) {
	resp, err := c.grpc.ListAppSessions(
		ctx,
		&proto.ListAppSessionsRequest{
			PageSize:  int32(pageSize),
			PageToken: pageToken,
			User:      user,
		})
	if err != nil {
		return nil, "", trail.FromGRPC(err)
	}

	out := make([]types.WebSession, 0, len(resp.GetSessions()))
	for _, v := range resp.GetSessions() {
		out = append(out, v)
	}
	return out, resp.NextPageToken, nil
}

// GetSnowflakeSessions gets all Snowflake web sessions.
func (c *Client) GetSnowflakeSessions(ctx context.Context) ([]types.WebSession, error) {
	resp, err := c.grpc.GetSnowflakeSessions(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	out := make([]types.WebSession, 0, len(resp.GetSessions()))
	for _, v := range resp.GetSessions() {
		out = append(out, v)
	}
	return out, nil
}

// ListSAMLIdPSessions gets a paginated list of SAML IdP sessions.
func (c *Client) ListSAMLIdPSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error) {
	resp, err := c.grpc.ListSAMLIdPSessions(
		ctx,
		&proto.ListSAMLIdPSessionsRequest{
			PageSize:  int32(pageSize),
			PageToken: pageToken,
			User:      user,
		})
	if err != nil {
		return nil, "", trail.FromGRPC(err)
	}

	out := make([]types.WebSession, 0, len(resp.GetSessions()))
	for _, v := range resp.GetSessions() {
		out = append(out, v)
	}
	return out, resp.NextPageToken, nil
}

// CreateAppSession creates an application web session. Application web
// sessions represent a browser session the client holds.
func (c *Client) CreateAppSession(ctx context.Context, req types.CreateAppSessionRequest) (types.WebSession, error) {
	resp, err := c.grpc.CreateAppSession(ctx, &proto.CreateAppSessionRequest{
		Username:          req.Username,
		PublicAddr:        req.PublicAddr,
		ClusterName:       req.ClusterName,
		AWSRoleARN:        req.AWSRoleARN,
		AzureIdentity:     req.AzureIdentity,
		GCPServiceAccount: req.GCPServiceAccount,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return resp.GetSession(), nil
}

// CreateSnowflakeSession creates a Snowflake web session.
func (c *Client) CreateSnowflakeSession(ctx context.Context, req types.CreateSnowflakeSessionRequest) (types.WebSession, error) {
	resp, err := c.grpc.CreateSnowflakeSession(ctx, &proto.CreateSnowflakeSessionRequest{
		Username:     req.Username,
		SessionToken: req.SessionToken,
		TokenTTL:     proto.Duration(req.TokenTTL),
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return resp.GetSession(), nil
}

// CreateSAMLIdPSession creates a SAML IdP session.
func (c *Client) CreateSAMLIdPSession(ctx context.Context, req types.CreateSAMLIdPSessionRequest) (types.WebSession, error) {
	resp, err := c.grpc.CreateSAMLIdPSession(ctx, &proto.CreateSAMLIdPSessionRequest{
		SessionID:   req.SessionID,
		Username:    req.Username,
		SAMLSession: req.SAMLSession,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return resp.GetSession(), nil
}

// GetSnowflakeSession gets a Snowflake web session.
func (c *Client) GetSnowflakeSession(ctx context.Context, req types.GetSnowflakeSessionRequest) (types.WebSession, error) {
	resp, err := c.grpc.GetSnowflakeSession(ctx, &proto.GetSnowflakeSessionRequest{
		SessionID: req.SessionID,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return resp.GetSession(), nil
}

// GetSAMLIdPSession gets a SAML IdP session.
func (c *Client) GetSAMLIdPSession(ctx context.Context, req types.GetSAMLIdPSessionRequest) (types.WebSession, error) {
	resp, err := c.grpc.GetSAMLIdPSession(ctx, &proto.GetSAMLIdPSessionRequest{
		SessionID: req.SessionID,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return resp.GetSession(), nil
}

// DeleteAppSession removes an application web session.
func (c *Client) DeleteAppSession(ctx context.Context, req types.DeleteAppSessionRequest) error {
	_, err := c.grpc.DeleteAppSession(ctx, &proto.DeleteAppSessionRequest{
		SessionID: req.SessionID,
	})
	return trail.FromGRPC(err)
}

// DeleteSnowflakeSession removes a Snowflake web session.
func (c *Client) DeleteSnowflakeSession(ctx context.Context, req types.DeleteSnowflakeSessionRequest) error {
	_, err := c.grpc.DeleteSnowflakeSession(ctx, &proto.DeleteSnowflakeSessionRequest{
		SessionID: req.SessionID,
	})
	return trail.FromGRPC(err)
}

// DeleteSAMLIdPSession removes a SAML IdP session.
func (c *Client) DeleteSAMLIdPSession(ctx context.Context, req types.DeleteSAMLIdPSessionRequest) error {
	_, err := c.grpc.DeleteSAMLIdPSession(ctx, &proto.DeleteSAMLIdPSessionRequest{
		SessionID: req.SessionID,
	})
	return trail.FromGRPC(err)
}

// DeleteAllAppSessions removes all application web sessions.
func (c *Client) DeleteAllAppSessions(ctx context.Context) error {
	_, err := c.grpc.DeleteAllAppSessions(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// DeleteAllSnowflakeSessions removes all Snowflake web sessions.
func (c *Client) DeleteAllSnowflakeSessions(ctx context.Context) error {
	_, err := c.grpc.DeleteAllSnowflakeSessions(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// DeleteAllSAMLIdPSessions removes all SAML IdP sessions.
func (c *Client) DeleteAllSAMLIdPSessions(ctx context.Context) error {
	_, err := c.grpc.DeleteAllSAMLIdPSessions(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// DeleteUserAppSessions deletes all user’s application sessions.
func (c *Client) DeleteUserAppSessions(ctx context.Context, req *proto.DeleteUserAppSessionsRequest) error {
	_, err := c.grpc.DeleteUserAppSessions(ctx, req)
	return trail.FromGRPC(err)
}

// DeleteUserSAMLIdPSessions deletes all user’s SAML IdP sessions.
func (c *Client) DeleteUserSAMLIdPSessions(ctx context.Context, username string) error {
	req := &proto.DeleteUserSAMLIdPSessionsRequest{
		Username: username,
	}
	_, err := c.grpc.DeleteUserSAMLIdPSessions(ctx, req)
	return trail.FromGRPC(err)
}

// GenerateAppToken creates a JWT token with application access.
func (c *Client) GenerateAppToken(ctx context.Context, req types.GenerateAppTokenRequest) (string, error) {
	traits := map[string]*wrappers.StringValues{}
	for traitName, traitValues := range req.Traits {
		traits[traitName] = &wrappers.StringValues{
			Values: traitValues,
		}
	}
	resp, err := c.grpc.GenerateAppToken(ctx, &proto.GenerateAppTokenRequest{
		Username: req.Username,
		Roles:    req.Roles,
		Traits:   traits,
		URI:      req.URI,
		Expires:  req.Expires,
	})
	if err != nil {
		return "", trail.FromGRPC(err)
	}

	return resp.GetToken(), nil
}

// GenerateSnowflakeJWT generates JWT in the Snowflake required format.
func (c *Client) GenerateSnowflakeJWT(ctx context.Context, req types.GenerateSnowflakeJWT) (string, error) {
	resp, err := c.grpc.GenerateSnowflakeJWT(ctx, &proto.SnowflakeJWTRequest{
		UserName:    req.Username,
		AccountName: req.Account,
	})
	if err != nil {
		return "", trail.FromGRPC(err)
	}

	return resp.GetToken(), nil
}

// GetDatabaseServers returns all registered database proxy servers.
//
// Note that in HA setups, a registered database may have multiple
// DatabaseServer entries. Web UI and `tsh db ls` extract databases from this
// list and remove duplicates by name.
func (c *Client) GetDatabaseServers(ctx context.Context, namespace string) ([]types.DatabaseServer, error) {
	servers, err := GetAllResources[types.DatabaseServer](ctx, c, &proto.ListResourcesRequest{
		Namespace:    namespace,
		ResourceType: types.KindDatabaseServer,
	})
	return servers, trail.FromGRPC(err)
}

// UpsertDatabaseServer registers a new database proxy server.
func (c *Client) UpsertDatabaseServer(ctx context.Context, server types.DatabaseServer) (*types.KeepAlive, error) {
	s, ok := server.(*types.DatabaseServerV3)
	if !ok {
		return nil, trace.BadParameter("invalid type %T", server)
	}
	keepAlive, err := c.grpc.UpsertDatabaseServer(ctx, &proto.UpsertDatabaseServerRequest{
		Server: s,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return keepAlive, nil
}

// DeleteDatabaseServer removes the specified database proxy server.
func (c *Client) DeleteDatabaseServer(ctx context.Context, namespace, hostID, name string) error {
	_, err := c.grpc.DeleteDatabaseServer(ctx, &proto.DeleteDatabaseServerRequest{
		Namespace: namespace,
		HostID:    hostID,
		Name:      name,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// DeleteAllDatabaseServers removes all registered database proxy servers.
func (c *Client) DeleteAllDatabaseServers(ctx context.Context, namespace string) error {
	_, err := c.grpc.DeleteAllDatabaseServers(ctx, &proto.DeleteAllDatabaseServersRequest{
		Namespace: namespace,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// SignDatabaseCSR generates a client certificate used by proxy when talking
// to a remote database service.
func (c *Client) SignDatabaseCSR(ctx context.Context, req *proto.DatabaseCSRRequest) (*proto.DatabaseCSRResponse, error) {
	resp, err := c.grpc.SignDatabaseCSR(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GenerateDatabaseCert generates client certificate used by a database
// service to authenticate with the database instance.
func (c *Client) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	resp, err := c.grpc.GenerateDatabaseCert(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetRole returns role by name
func (c *Client) GetRole(ctx context.Context, name string) (types.Role, error) {
	if name == "" {
		return nil, trace.BadParameter("missing name")
	}
	role, err := c.grpc.GetRole(ctx, &proto.GetRoleRequest{Name: name})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return role, nil
}

// GetRoles returns a list of roles
func (c *Client) GetRoles(ctx context.Context) ([]types.Role, error) {
	resp, err := c.grpc.GetRoles(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	roles := make([]types.Role, 0, len(resp.GetRoles()))
	for _, role := range resp.GetRoles() {
		roles = append(roles, role)
	}
	return roles, nil
}

// UpsertRole creates or updates role
func (c *Client) UpsertRole(ctx context.Context, role types.Role) error {
	r, ok := role.(*types.RoleV6)
	if !ok {
		return trace.BadParameter("invalid type %T", role)
	}

	_, err := c.grpc.UpsertRole(ctx, r)
	return trail.FromGRPC(err)
}

// DeleteRole deletes role by name
func (c *Client) DeleteRole(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing name")
	}
	_, err := c.grpc.DeleteRole(ctx, &proto.DeleteRoleRequest{Name: name})
	return trail.FromGRPC(err)
}

func (c *Client) AddMFADevice(ctx context.Context) (proto.AuthService_AddMFADeviceClient, error) {
	stream, err := c.grpc.AddMFADevice(ctx)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return stream, nil
}

func (c *Client) DeleteMFADevice(ctx context.Context) (proto.AuthService_DeleteMFADeviceClient, error) {
	stream, err := c.grpc.DeleteMFADevice(ctx)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return stream, nil
}

// AddMFADeviceSync adds a new MFA device (nonstream).
func (c *Client) AddMFADeviceSync(ctx context.Context, in *proto.AddMFADeviceSyncRequest) (*proto.AddMFADeviceSyncResponse, error) {
	res, err := c.grpc.AddMFADeviceSync(ctx, in)
	return res, trail.FromGRPC(err)
}

// DeleteMFADeviceSync deletes a users MFA device (nonstream).
func (c *Client) DeleteMFADeviceSync(ctx context.Context, in *proto.DeleteMFADeviceSyncRequest) error {
	_, err := c.grpc.DeleteMFADeviceSync(ctx, in)
	return trail.FromGRPC(err)
}

func (c *Client) GetMFADevices(ctx context.Context, in *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error) {
	resp, err := c.grpc.GetMFADevices(ctx, in)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

func (c *Client) GenerateUserSingleUseCerts(ctx context.Context) (proto.AuthService_GenerateUserSingleUseCertsClient, error) {
	stream, err := c.grpc.GenerateUserSingleUseCerts(ctx)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return stream, nil
}

func (c *Client) IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
	resp, err := c.grpc.IsMFARequired(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetOIDCConnector returns an OIDC connector by name.
func (c *Client) GetOIDCConnector(ctx context.Context, name string, withSecrets bool) (types.OIDCConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("cannot get OIDC Connector, missing name")
	}
	req := &types.ResourceWithSecretsRequest{Name: name, WithSecrets: withSecrets}
	resp, err := c.grpc.GetOIDCConnector(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetOIDCConnectors returns a list of OIDC connectors.
func (c *Client) GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error) {
	req := &types.ResourcesWithSecretsRequest{WithSecrets: withSecrets}
	resp, err := c.grpc.GetOIDCConnectors(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	oidcConnectors := make([]types.OIDCConnector, len(resp.OIDCConnectors))
	for i, oidcConnector := range resp.OIDCConnectors {
		oidcConnectors[i] = oidcConnector
	}
	return oidcConnectors, nil
}

// UpsertOIDCConnector creates or updates an OIDC connector.
func (c *Client) UpsertOIDCConnector(ctx context.Context, oidcConnector types.OIDCConnector) error {
	connector, ok := oidcConnector.(*types.OIDCConnectorV3)
	if !ok {
		return trace.BadParameter("invalid type %T", oidcConnector)
	}
	_, err := c.grpc.UpsertOIDCConnector(ctx, connector)
	return trail.FromGRPC(err)
}

// DeleteOIDCConnector deletes an OIDC connector by name.
func (c *Client) DeleteOIDCConnector(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("cannot delete OIDC Connector, missing name")
	}
	_, err := c.grpc.DeleteOIDCConnector(ctx, &types.ResourceRequest{Name: name})
	return trail.FromGRPC(err)
}

// CreateOIDCAuthRequest creates OIDCAuthRequest.
func (c *Client) CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	resp, err := c.grpc.CreateOIDCAuthRequest(ctx, &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetOIDCAuthRequest gets an OIDCAuthRequest by state token.
func (c *Client) GetOIDCAuthRequest(ctx context.Context, stateToken string) (*types.OIDCAuthRequest, error) {
	req := &proto.GetOIDCAuthRequestRequest{StateToken: stateToken}
	resp, err := c.grpc.GetOIDCAuthRequest(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetSAMLConnector returns a SAML connector by name.
func (c *Client) GetSAMLConnector(ctx context.Context, name string, withSecrets bool) (types.SAMLConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("cannot get SAML Connector, missing name")
	}
	req := &types.ResourceWithSecretsRequest{Name: name, WithSecrets: withSecrets}
	resp, err := c.grpc.GetSAMLConnector(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetSAMLConnectors returns a list of SAML connectors.
func (c *Client) GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error) {
	req := &types.ResourcesWithSecretsRequest{WithSecrets: withSecrets}
	resp, err := c.grpc.GetSAMLConnectors(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	samlConnectors := make([]types.SAMLConnector, len(resp.SAMLConnectors))
	for i, samlConnector := range resp.SAMLConnectors {
		samlConnectors[i] = samlConnector
	}
	return samlConnectors, nil
}

// UpsertSAMLConnector creates or updates a SAML connector.
func (c *Client) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error {
	samlConnectorV2, ok := connector.(*types.SAMLConnectorV2)
	if !ok {
		return trace.BadParameter("invalid type %T", connector)
	}
	_, err := c.grpc.UpsertSAMLConnector(ctx, samlConnectorV2)
	return trail.FromGRPC(err)
}

// DeleteSAMLConnector deletes a SAML connector by name.
func (c *Client) DeleteSAMLConnector(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("cannot delete SAML Connector, missing name")
	}
	_, err := c.grpc.DeleteSAMLConnector(ctx, &types.ResourceRequest{Name: name})
	return trail.FromGRPC(err)
}

// CreateSAMLAuthRequest creates SAMLAuthRequest.
func (c *Client) CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error) {
	resp, err := c.grpc.CreateSAMLAuthRequest(ctx, &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetSAMLAuthRequest gets a SAMLAuthRequest by id.
func (c *Client) GetSAMLAuthRequest(ctx context.Context, id string) (*types.SAMLAuthRequest, error) {
	req := &proto.GetSAMLAuthRequestRequest{ID: id}
	resp, err := c.grpc.GetSAMLAuthRequest(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetGithubConnector returns a Github connector by name.
func (c *Client) GetGithubConnector(ctx context.Context, name string, withSecrets bool) (types.GithubConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("cannot get GitHub Connector, missing name")
	}
	req := &types.ResourceWithSecretsRequest{Name: name, WithSecrets: withSecrets}
	resp, err := c.grpc.GetGithubConnector(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetGithubConnectors returns a list of Github connectors.
func (c *Client) GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error) {
	req := &types.ResourcesWithSecretsRequest{WithSecrets: withSecrets}
	resp, err := c.grpc.GetGithubConnectors(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	githubConnectors := make([]types.GithubConnector, len(resp.GithubConnectors))
	for i, githubConnector := range resp.GithubConnectors {
		githubConnectors[i] = githubConnector
	}
	return githubConnectors, nil
}

// UpsertGithubConnector creates or updates a Github connector.
func (c *Client) UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) error {
	githubConnector, ok := connector.(*types.GithubConnectorV3)
	if !ok {
		return trace.BadParameter("invalid type %T", connector)
	}
	_, err := c.grpc.UpsertGithubConnector(ctx, githubConnector)
	return trail.FromGRPC(err)
}

// DeleteGithubConnector deletes a Github connector by name.
func (c *Client) DeleteGithubConnector(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("cannot delete GitHub Connector, missing name")
	}
	_, err := c.grpc.DeleteGithubConnector(ctx, &types.ResourceRequest{Name: name})
	return trail.FromGRPC(err)
}

// CreateGithubAuthRequest creates GithubAuthRequest.
func (c *Client) CreateGithubAuthRequest(ctx context.Context, req types.GithubAuthRequest) (*types.GithubAuthRequest, error) {
	resp, err := c.grpc.CreateGithubAuthRequest(ctx, &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetGithubAuthRequest gets a GithubAuthRequest by state token.
func (c *Client) GetGithubAuthRequest(ctx context.Context, stateToken string) (*types.GithubAuthRequest, error) {
	req := &proto.GetGithubAuthRequestRequest{StateToken: stateToken}
	resp, err := c.grpc.GetGithubAuthRequest(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetSSODiagnosticInfo returns SSO diagnostic info records for a specific SSO Auth request.
func (c *Client) GetSSODiagnosticInfo(ctx context.Context, authRequestKind string, authRequestID string) (*types.SSODiagnosticInfo, error) {
	req := &proto.GetSSODiagnosticInfoRequest{AuthRequestKind: authRequestKind, AuthRequestID: authRequestID}
	resp, err := c.grpc.GetSSODiagnosticInfo(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetServerInfos returns a stream of ServerInfos.
func (c *Client) GetServerInfos(ctx context.Context) stream.Stream[types.ServerInfo] {
	// set up cancelable context so that Stream.Done can close the stream if the caller
	// halts early.
	ctx, cancel := context.WithCancel(ctx)

	serverInfos, err := c.grpc.GetServerInfos(ctx, &emptypb.Empty{})
	if err != nil {
		cancel()
		return stream.Fail[types.ServerInfo](trail.FromGRPC(err))
	}
	return stream.Func(func() (types.ServerInfo, error) {
		si, err := serverInfos.Recv()
		if err != nil {
			if trace.IsEOF(err) {
				// io.EOF signals that stream has completed successfully
				return nil, io.EOF
			}
			return nil, trail.FromGRPC(err)
		}
		return si, nil
	}, cancel)
}

// GetServerInfo returns a ServerInfo by name.
func (c *Client) GetServerInfo(ctx context.Context, name string) (types.ServerInfo, error) {
	if name == "" {
		return nil, trace.BadParameter("cannot get server info, missing name")
	}
	req := &types.ResourceRequest{Name: name}
	resp, err := c.grpc.GetServerInfo(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// UpsertServerInfo upserts a ServerInfo.
func (c *Client) UpsertServerInfo(ctx context.Context, serverInfo types.ServerInfo) error {
	si, ok := serverInfo.(*types.ServerInfoV1)
	if !ok {
		return trace.BadParameter("invalid type %T", serverInfo)
	}
	_, err := c.grpc.UpsertServerInfo(ctx, si)
	return trail.FromGRPC(err)
}

// DeleteServerInfo deletes a ServerInfo by name.
func (c *Client) DeleteServerInfo(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("cannot delete server info, missing name")
	}
	req := &types.ResourceRequest{Name: name}
	_, err := c.grpc.DeleteServerInfo(ctx, req)
	return trail.FromGRPC(err)
}

// DeleteAllServerInfos deletes all ServerInfos.
func (c *Client) DeleteAllServerInfos(ctx context.Context) error {
	_, err := c.grpc.DeleteAllServerInfos(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// GetTrustedCluster returns a Trusted Cluster by name.
func (c *Client) GetTrustedCluster(ctx context.Context, name string) (types.TrustedCluster, error) {
	if name == "" {
		return nil, trace.BadParameter("cannot get trusted cluster, missing name")
	}
	req := &types.ResourceRequest{Name: name}
	resp, err := c.grpc.GetTrustedCluster(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetTrustedClusters returns a list of Trusted Clusters.
func (c *Client) GetTrustedClusters(ctx context.Context) ([]types.TrustedCluster, error) {
	resp, err := c.grpc.GetTrustedClusters(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	trustedClusters := make([]types.TrustedCluster, len(resp.TrustedClusters))
	for i, trustedCluster := range resp.TrustedClusters {
		trustedClusters[i] = trustedCluster
	}
	return trustedClusters, nil
}

// UpsertTrustedCluster creates or updates a Trusted Cluster.
func (c *Client) UpsertTrustedCluster(ctx context.Context, trusedCluster types.TrustedCluster) (types.TrustedCluster, error) {
	trustedCluster, ok := trusedCluster.(*types.TrustedClusterV2)
	if !ok {
		return nil, trace.BadParameter("invalid type %T", trusedCluster)
	}
	resp, err := c.grpc.UpsertTrustedCluster(ctx, trustedCluster)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// DeleteTrustedCluster deletes a Trusted Cluster by name.
func (c *Client) DeleteTrustedCluster(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("cannot delete trusted cluster, missing name")
	}
	_, err := c.grpc.DeleteTrustedCluster(ctx, &types.ResourceRequest{Name: name})
	return trail.FromGRPC(err)
}

// GetToken returns a provision token by name.
func (c *Client) GetToken(ctx context.Context, name string) (types.ProvisionToken, error) {
	if name == "" {
		return nil, trace.BadParameter("cannot get token, missing name")
	}
	resp, err := c.grpc.GetToken(ctx, &types.ResourceRequest{Name: name})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetTokens returns a list of active provision tokens for nodes and users.
func (c *Client) GetTokens(ctx context.Context) ([]types.ProvisionToken, error) {
	resp, err := c.grpc.GetTokens(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	tokens := make([]types.ProvisionToken, len(resp.ProvisionTokens))
	for i, token := range resp.ProvisionTokens {
		tokens[i] = token
	}
	return tokens, nil
}

// UpsertToken creates or updates a provision token.
func (c *Client) UpsertToken(ctx context.Context, token types.ProvisionToken) error {
	tokenV2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("invalid type %T", token)
	}

	_, err := c.grpc.UpsertTokenV2(ctx, &proto.UpsertTokenV2Request{
		Token: &proto.UpsertTokenV2Request_V2{
			V2: tokenV2,
		},
	})
	if err != nil {
		err := trail.FromGRPC(err)
		if trace.IsNotImplemented(err) {
			_, err := c.grpc.UpsertToken(ctx, tokenV2)
			return trail.FromGRPC(err)
		}
		return err
	}
	return nil
}

// CreateToken creates a provision token.
func (c *Client) CreateToken(ctx context.Context, token types.ProvisionToken) error {
	tokenV2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("invalid type %T", token)
	}

	_, err := c.grpc.CreateTokenV2(ctx, &proto.CreateTokenV2Request{
		Token: &proto.CreateTokenV2Request_V2{
			V2: tokenV2,
		},
	})
	if err != nil {
		err := trail.FromGRPC(err)
		if trace.IsNotImplemented(err) {
			_, err := c.grpc.CreateToken(ctx, tokenV2)
			return trail.FromGRPC(err)
		}
		return err
	}
	return nil
}

// DeleteToken deletes a provision token by name.
func (c *Client) DeleteToken(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("cannot delete token, missing name")
	}
	_, err := c.grpc.DeleteToken(ctx, &types.ResourceRequest{Name: name})
	return trail.FromGRPC(err)
}

// GetNode returns a node by name and namespace.
func (c *Client) GetNode(ctx context.Context, namespace, name string) (types.Server, error) {
	resp, err := c.grpc.GetNode(ctx, &types.ResourceInNamespaceRequest{
		Name:      name,
		Namespace: namespace,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetNodes returns a complete list of nodes that the user has access to in the given namespace.
func (c *Client) GetNodes(ctx context.Context, namespace string) ([]types.Server, error) {
	servers, err := GetAllResources[types.Server](ctx, c, &proto.ListResourcesRequest{
		ResourceType: types.KindNode,
		Namespace:    namespace,
	})

	return servers, trace.Wrap(err)
}

// UpsertNode is used by SSH servers to report their presence
// to the auth servers in form of heartbeat expiring after ttl period.
func (c *Client) UpsertNode(ctx context.Context, node types.Server) (*types.KeepAlive, error) {
	if node.GetNamespace() == "" {
		return nil, trace.BadParameter("missing node namespace")
	}
	serverV2, ok := node.(*types.ServerV2)
	if !ok {
		return nil, trace.BadParameter("invalid type %T", node)
	}
	keepAlive, err := c.grpc.UpsertNode(ctx, serverV2)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return keepAlive, nil
}

// DeleteNode deletes a node by name and namespace.
func (c *Client) DeleteNode(ctx context.Context, namespace, name string) error {
	if namespace == "" {
		return trace.BadParameter("missing parameter namespace")
	}
	if name == "" {
		return trace.BadParameter("missing parameter name")
	}
	_, err := c.grpc.DeleteNode(ctx, &types.ResourceInNamespaceRequest{
		Name:      name,
		Namespace: namespace,
	})
	return trail.FromGRPC(err)
}

// DeleteAllNodes deletes all nodes in a given namespace.
func (c *Client) DeleteAllNodes(ctx context.Context, namespace string) error {
	if namespace == "" {
		return trace.BadParameter("missing parameter namespace")
	}
	_, err := c.grpc.DeleteAllNodes(ctx, &types.ResourcesInNamespaceRequest{Namespace: namespace})
	return trail.FromGRPC(err)
}

// StreamSessionEvents streams audit events from a given session recording.
func (c *Client) StreamSessionEvents(ctx context.Context, sessionID string, startIndex int64) (chan events.AuditEvent, chan error) {
	request := &proto.StreamSessionEventsRequest{
		SessionID:  sessionID,
		StartIndex: int32(startIndex),
	}

	ch := make(chan events.AuditEvent)
	e := make(chan error, 1)

	stream, err := c.grpc.StreamSessionEvents(ctx, request)
	if err != nil {
		e <- trace.Wrap(err)
		return ch, e
	}

	go func() {
	outer:
		for {
			oneOf, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					e <- trace.Wrap(trail.FromGRPC(err))
				} else {
					close(ch)
				}

				break outer
			}

			event, err := events.FromOneOf(*oneOf)
			if err != nil {
				e <- trace.Wrap(trail.FromGRPC(err))
				break outer
			}

			select {
			case ch <- event:
			case <-ctx.Done():
				e <- trace.Wrap(ctx.Err())
				break outer
			}
		}
	}()

	return ch, e
}

// SearchEvents allows searching for events with a full pagination support.
func (c *Client) SearchEvents(ctx context.Context, fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]events.AuditEvent, string, error) {
	request := &proto.GetEventsRequest{
		Namespace:  namespace,
		StartDate:  fromUTC,
		EndDate:    toUTC,
		EventTypes: eventTypes,
		Limit:      int32(limit),
		StartKey:   startKey,
		Order:      proto.Order(order),
	}

	response, err := c.grpc.GetEvents(ctx, request)
	if err != nil {
		return nil, "", trail.FromGRPC(err)
	}

	decodedEvents := make([]events.AuditEvent, 0, len(response.Items))
	for _, rawEvent := range response.Items {
		event, err := events.FromOneOf(*rawEvent)
		if err != nil {
			if trace.IsBadParameter(err) {
				log.Warnf("skipping unknown event: %v", err)
				continue
			}
			return nil, "", trace.Wrap(err)
		}
		decodedEvents = append(decodedEvents, event)
	}

	return decodedEvents, response.LastKey, nil
}

// SearchUnstructuredEvents allows searching for events with a full pagination support
// and returns events in an unstructured format (json like).
// This method is used by the Teleport event-handler plugin to receive events
// from the auth server wihout having to support the Protobuf event schema.
func (c *Client) SearchUnstructuredEvents(ctx context.Context, fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]*auditlogpb.EventUnstructured, string, error) {
	request := &auditlogpb.GetUnstructuredEventsRequest{
		Namespace:  namespace,
		StartDate:  timestamppb.New(fromUTC),
		EndDate:    timestamppb.New(toUTC),
		EventTypes: eventTypes,
		Limit:      int32(limit),
		StartKey:   startKey,
		Order:      auditlogpb.Order(order),
	}

	response, err := c.grpc.GetUnstructuredEvents(ctx, request)
	if err != nil {
		err = trail.FromGRPC(err)
		// If the server does not support the unstructured events API,
		// fallback to the legacy API.
		if trace.IsNotImplemented(err) {
			return c.searchUnstructuredEventsFallback(ctx, fromUTC, toUTC, namespace, eventTypes, limit, order, startKey)
		}
		return nil, "", err
	}

	return response.Items, response.LastKey, nil
}

// searchUnstructuredEventsFallback is a fallback implementation of the
// SearchUnstructuredEvents method that is used when the server does not
// support the unstructured events API.
// This method converts the events at event handler plugin side, which can cause
// the plugin to miss some events if the plugin is not updated to the latest
// version.
// TODO(tigrato): DELETE IN 15.0.0
func (c *Client) searchUnstructuredEventsFallback(ctx context.Context, fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]*auditlogpb.EventUnstructured, string, error) {
	eventsRcv, next, err := c.SearchEvents(ctx, fromUTC, toUTC, namespace, eventTypes, limit, order, startKey)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	items := make([]*auditlogpb.EventUnstructured, 0, len(eventsRcv))
	for _, evt := range eventsRcv {
		item, err := events.ToUnstructured(evt)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		items = append(items, item)
	}
	return items, next, nil
}

// StreamUnstructuredSessionEvents streams audit events from a given session recording in an unstructured format.
// This method is used by the Teleport event-handler plugin to receive events
// from the auth server wihout having to support the Protobuf event schema.
func (c *Client) StreamUnstructuredSessionEvents(ctx context.Context, sessionID string, startIndex int64) (chan *auditlogpb.EventUnstructured, chan error) {
	request := &auditlogpb.StreamUnstructuredSessionEventsRequest{
		SessionId:  sessionID,
		StartIndex: int32(startIndex),
	}

	ch := make(chan *auditlogpb.EventUnstructured)
	e := make(chan error, 1)

	stream, err := c.grpc.StreamUnstructuredSessionEvents(ctx, request)
	if err != nil {
		if trace.IsNotImplemented(trail.FromGRPC(err)) {
			// If the server does not support the unstructured events API,
			// fallback to the legacy API.
			// This code patch shouldn't be triggered because the server
			// returns the error only if the client calls Recv() on the stream.
			// However, we keep this code patch here just in case there is a bug
			// on the client grpc side.
			c.streamUnstructuredSessionEventsFallback(ctx, sessionID, startIndex, ch, e)
		} else {
			e <- trace.Wrap(trail.FromGRPC(err))
		}
		return ch, e
	}
	go func() {
		for {
			event, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					// If the server does not support the unstructured events API, it will
					// return an error with code Unimplemented. This error is received
					// the first time the client calls Recv() on the stream.
					// If the client receives this error, it should fallback to the legacy
					// API that spins another goroutine to convert the events to the
					// unstructured format and sends them to the channel ch.
					// Once we decide to spin the goroutine, we can leave this loop without
					// reporting any error to the caller.
					if trace.IsNotImplemented(trail.FromGRPC(err)) {
						// If the server does not support the unstructured events API,
						// fallback to the legacy API.
						go c.streamUnstructuredSessionEventsFallback(ctx, sessionID, startIndex, ch, e)
						return
					}
					e <- trace.Wrap(trail.FromGRPC(err))
				} else {
					close(ch)
				}
				return
			}

			select {
			case ch <- event:
			case <-ctx.Done():
				e <- trace.Wrap(ctx.Err())
				return
			}
		}
	}()

	return ch, e
}

// streamUnstructuredSessionEventsFallback is a fallback implementation of the
// StreamUnstructuredSessionEvents method that is used when the server does not
// support the unstructured events API. This method uses the old API to stream
// events from the server and converts them to the unstructured format. This
// method converts the events at event handler plugin side, which can cause
// the plugin to miss some events if the plugin is not updated to the latest
// version.
// TODO(tigrato): DELETE IN 15.0.0
func (c *Client) streamUnstructuredSessionEventsFallback(ctx context.Context, sessionID string, startIndex int64, ch chan *auditlogpb.EventUnstructured, e chan error) {
	request := &proto.StreamSessionEventsRequest{
		SessionID:  sessionID,
		StartIndex: int32(startIndex),
	}

	stream, err := c.grpc.StreamSessionEvents(ctx, request)
	if err != nil {
		e <- trace.Wrap(err)
		return
	}

	go func() {
		for {
			oneOf, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					e <- trace.Wrap(trail.FromGRPC(err))
				} else {
					close(ch)
				}

				return
			}

			event, err := events.FromOneOf(*oneOf)
			if err != nil {
				e <- trace.Wrap(trail.FromGRPC(err))
				return
			}

			unstructedEvent, err := events.ToUnstructured(event)
			if err != nil {
				e <- trace.Wrap(err)
				return
			}

			select {
			case ch <- unstructedEvent:
			case <-ctx.Done():
				e <- trace.Wrap(ctx.Err())
				return
			}
		}
	}()
}

// SearchSessionEvents allows searching for session events with a full pagination support.
func (c *Client) SearchSessionEvents(ctx context.Context, fromUTC time.Time, toUTC time.Time, limit int, order types.EventOrder, startKey string) ([]events.AuditEvent, string, error) {
	request := &proto.GetSessionEventsRequest{
		StartDate: fromUTC,
		EndDate:   toUTC,
		Limit:     int32(limit),
		StartKey:  startKey,
		Order:     proto.Order(order),
	}

	response, err := c.grpc.GetSessionEvents(ctx, request)
	if err != nil {
		return nil, "", trail.FromGRPC(err)
	}

	decodedEvents := make([]events.AuditEvent, 0, len(response.Items))
	for _, rawEvent := range response.Items {
		event, err := events.FromOneOf(*rawEvent)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		decodedEvents = append(decodedEvents, event)
	}

	return decodedEvents, response.LastKey, nil
}

// GetClusterNetworkingConfig gets cluster networking configuration.
func (c *Client) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	resp, err := c.grpc.GetClusterNetworkingConfig(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// SetClusterNetworkingConfig sets cluster networking configuration.
func (c *Client) SetClusterNetworkingConfig(ctx context.Context, netConfig types.ClusterNetworkingConfig) error {
	netConfigV2, ok := netConfig.(*types.ClusterNetworkingConfigV2)
	if !ok {
		return trace.BadParameter("invalid type %T", netConfig)
	}
	_, err := c.grpc.SetClusterNetworkingConfig(ctx, netConfigV2)
	return trail.FromGRPC(err)
}

// ResetClusterNetworkingConfig resets cluster networking configuration to defaults.
func (c *Client) ResetClusterNetworkingConfig(ctx context.Context) error {
	_, err := c.grpc.ResetClusterNetworkingConfig(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// GetSessionRecordingConfig gets session recording configuration.
func (c *Client) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	resp, err := c.grpc.GetSessionRecordingConfig(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// SetSessionRecordingConfig sets session recording configuration.
func (c *Client) SetSessionRecordingConfig(ctx context.Context, recConfig types.SessionRecordingConfig) error {
	recConfigV2, ok := recConfig.(*types.SessionRecordingConfigV2)
	if !ok {
		return trace.BadParameter("invalid type %T", recConfig)
	}
	_, err := c.grpc.SetSessionRecordingConfig(ctx, recConfigV2)
	return trail.FromGRPC(err)
}

// ResetSessionRecordingConfig resets session recording configuration to defaults.
func (c *Client) ResetSessionRecordingConfig(ctx context.Context) error {
	_, err := c.grpc.ResetSessionRecordingConfig(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// GetAuthPreference gets cluster auth preference.
func (c *Client) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	pref, err := c.grpc.GetAuthPreference(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return pref, nil
}

// SetAuthPreference sets cluster auth preference.
func (c *Client) SetAuthPreference(ctx context.Context, authPref types.AuthPreference) error {
	authPrefV2, ok := authPref.(*types.AuthPreferenceV2)
	if !ok {
		return trace.BadParameter("invalid type %T", authPref)
	}
	_, err := c.grpc.SetAuthPreference(ctx, authPrefV2)
	return trail.FromGRPC(err)
}

// ResetAuthPreference resets cluster auth preference to defaults.
func (c *Client) ResetAuthPreference(ctx context.Context) error {
	_, err := c.grpc.ResetAuthPreference(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// GetClusterAuditConfig gets cluster audit configuration.
func (c *Client) GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error) {
	resp, err := c.grpc.GetClusterAuditConfig(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetInstaller gets all installer script resources
func (c *Client) GetInstallers(ctx context.Context) ([]types.Installer, error) {
	resp, err := c.grpc.GetInstallers(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	installers := make([]types.Installer, len(resp.Installers))
	for i, inst := range resp.Installers {
		installers[i] = inst
	}
	return installers, nil
}

// GetUIConfig gets the configuration for the UI served by the proxy service
func (c *Client) GetUIConfig(ctx context.Context) (types.UIConfig, error) {
	resp, err := c.grpc.GetUIConfig(ctx, &emptypb.Empty{})
	return resp, trail.FromGRPC(err)
}

// SetUIConfig sets the configuration for the UI served by the proxy service
func (c *Client) SetUIConfig(ctx context.Context, uic types.UIConfig) error {
	uicV1, ok := uic.(*types.UIConfigV1)
	if !ok {
		return trace.BadParameter("invalid type %T", uic)
	}
	_, err := c.grpc.SetUIConfig(ctx, uicV1)
	return trail.FromGRPC(err)
}

func (c *Client) DeleteUIConfig(ctx context.Context) error {
	_, err := c.grpc.DeleteUIConfig(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// GetInstaller gets the cluster installer resource
func (c *Client) GetInstaller(ctx context.Context, name string) (types.Installer, error) {
	resp, err := c.grpc.GetInstaller(ctx, &types.ResourceRequest{Name: name})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// SetInstaller sets the cluster installer resource
func (c *Client) SetInstaller(ctx context.Context, inst types.Installer) error {
	instV1, ok := inst.(*types.InstallerV1)
	if !ok {
		return trace.BadParameter("invalid type %T", inst)
	}
	_, err := c.grpc.SetInstaller(ctx, instV1)
	return trail.FromGRPC(err)
}

// DeleteInstaller deletes the cluster installer resource
func (c *Client) DeleteInstaller(ctx context.Context, name string) error {
	_, err := c.grpc.DeleteInstaller(ctx, &types.ResourceRequest{Name: name})
	return trail.FromGRPC(err)
}

// DeleteAllInstallers deletes all the installer resources.
func (c *Client) DeleteAllInstallers(ctx context.Context) error {
	_, err := c.grpc.DeleteAllInstallers(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// GetLock gets a lock by name.
func (c *Client) GetLock(ctx context.Context, name string) (types.Lock, error) {
	if name == "" {
		return nil, trace.BadParameter("missing lock name")
	}
	resp, err := c.grpc.GetLock(ctx, &proto.GetLockRequest{Name: name})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetLocks gets all/in-force locks that match at least one of the targets when specified.
func (c *Client) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
	targetPtrs := make([]*types.LockTarget, len(targets))
	for i := range targets {
		targetPtrs[i] = &targets[i]
	}
	resp, err := c.grpc.GetLocks(ctx, &proto.GetLocksRequest{
		InForceOnly: inForceOnly,
		Targets:     targetPtrs,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	locks := make([]types.Lock, 0, len(resp.Locks))
	for _, lock := range resp.Locks {
		locks = append(locks, lock)
	}
	return locks, nil
}

// UpsertLock upserts a lock.
func (c *Client) UpsertLock(ctx context.Context, lock types.Lock) error {
	lockV2, ok := lock.(*types.LockV2)
	if !ok {
		return trace.BadParameter("invalid type %T", lock)
	}
	_, err := c.grpc.UpsertLock(ctx, lockV2)
	return trail.FromGRPC(err)
}

// DeleteLock deletes a lock.
func (c *Client) DeleteLock(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing lock name")
	}
	_, err := c.grpc.DeleteLock(ctx, &proto.DeleteLockRequest{Name: name})
	return trail.FromGRPC(err)
}

// ReplaceRemoteLocks replaces the set of locks associated with a remote cluster.
func (c *Client) ReplaceRemoteLocks(ctx context.Context, clusterName string, locks []types.Lock) error {
	if clusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	lockV2s := make([]*types.LockV2, 0, len(locks))
	for _, lock := range locks {
		lockV2, ok := lock.(*types.LockV2)
		if !ok {
			return trace.BadParameter("unexpected lock type %T", lock)
		}
		lockV2s = append(lockV2s, lockV2)
	}
	_, err := c.grpc.ReplaceRemoteLocks(ctx, &proto.ReplaceRemoteLocksRequest{
		ClusterName: clusterName,
		Locks:       lockV2s,
	})
	return trail.FromGRPC(err)
}

// GetNetworkRestrictions retrieves the network restrictions
func (c *Client) GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error) {
	nr, err := c.grpc.GetNetworkRestrictions(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return nr, nil
}

// SetNetworkRestrictions updates the network restrictions
func (c *Client) SetNetworkRestrictions(ctx context.Context, nr types.NetworkRestrictions) error {
	restrictionsV4, ok := nr.(*types.NetworkRestrictionsV4)
	if !ok {
		return trace.BadParameter("invalid type %T", nr)
	}
	_, err := c.grpc.SetNetworkRestrictions(ctx, restrictionsV4)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// DeleteNetworkRestrictions deletes the network restrictions
func (c *Client) DeleteNetworkRestrictions(ctx context.Context) error {
	_, err := c.grpc.DeleteNetworkRestrictions(ctx, &emptypb.Empty{})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// CreateApp creates a new application resource.
func (c *Client) CreateApp(ctx context.Context, app types.Application) error {
	appV3, ok := app.(*types.AppV3)
	if !ok {
		return trace.BadParameter("unsupported application type %T", app)
	}
	_, err := c.grpc.CreateApp(ctx, appV3)
	return trail.FromGRPC(err)
}

// UpdateApp updates existing application resource.
func (c *Client) UpdateApp(ctx context.Context, app types.Application) error {
	appV3, ok := app.(*types.AppV3)
	if !ok {
		return trace.BadParameter("unsupported application type %T", app)
	}
	_, err := c.grpc.UpdateApp(ctx, appV3)
	return trail.FromGRPC(err)
}

// GetApp returns the specified application resource.
//
// Note that application resources here refers to "dynamically-added"
// applications such as applications created by `tctl create`, or the CreateApp
// API. Applications defined in the `app_service.apps` section of the service
// YAML configuration are not collected in this API.
//
// For a full list of registered applications that are served by an application
// service, use GetApplicationServers instead.
func (c *Client) GetApp(ctx context.Context, name string) (types.Application, error) {
	if name == "" {
		return nil, trace.BadParameter("missing application name")
	}
	app, err := c.grpc.GetApp(ctx, &types.ResourceRequest{Name: name})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return app, nil
}

// GetApps returns all application resources.
//
// Note that application resources here refers to "dynamically-added"
// applications such as applications created by `tctl create`, or the CreateApp
// API. Applications defined in the `app_service.apps` section of the service
// YAML configuration are not collected in this API.
//
// For a full list of registered applications that are served by an application
// service, use GetApplicationServers instead.
func (c *Client) GetApps(ctx context.Context) ([]types.Application, error) {
	items, err := c.grpc.GetApps(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	apps := make([]types.Application, len(items.Apps))
	for i := range items.Apps {
		apps[i] = items.Apps[i]
	}
	return apps, nil
}

// DeleteApp deletes specified application resource.
func (c *Client) DeleteApp(ctx context.Context, name string) error {
	_, err := c.grpc.DeleteApp(ctx, &types.ResourceRequest{Name: name})
	return trail.FromGRPC(err)
}

// DeleteAllApps deletes all application resources.
func (c *Client) DeleteAllApps(ctx context.Context) error {
	_, err := c.grpc.DeleteAllApps(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// CreateKubernetesCluster creates a new kubernetes cluster resource.
func (c *Client) CreateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	kubeClusterV3, ok := cluster.(*types.KubernetesClusterV3)
	if !ok {
		return trace.BadParameter("unsupported kubernetes cluster type %T", cluster)
	}
	_, err := c.grpc.CreateKubernetesCluster(ctx, kubeClusterV3)
	return trail.FromGRPC(err)
}

// UpdateKubernetesCluster updates existing kubernetes cluster resource.
func (c *Client) UpdateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	kubeClusterV3, ok := cluster.(*types.KubernetesClusterV3)
	if !ok {
		return trace.BadParameter("unsupported kubernetes cluster type %T", cluster)
	}
	_, err := c.grpc.UpdateKubernetesCluster(ctx, kubeClusterV3)
	return trail.FromGRPC(err)
}

// GetKubernetesCluster returns the specified kubernetes resource.
func (c *Client) GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error) {
	if name == "" {
		return nil, trace.BadParameter("missing kubernetes cluster name")
	}
	cluster, err := c.grpc.GetKubernetesCluster(ctx, &types.ResourceRequest{Name: name})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return cluster, nil
}

// GetKubernetesClusters returns all kubernetes cluster resources.
func (c *Client) GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error) {
	items, err := c.grpc.GetKubernetesClusters(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	clusters := make([]types.KubeCluster, len(items.KubernetesClusters))
	for i := range items.KubernetesClusters {
		clusters[i] = items.KubernetesClusters[i]
	}
	return clusters, nil
}

// DeleteKubernetesCluster deletes specified kubernetes cluster resource.
func (c *Client) DeleteKubernetesCluster(ctx context.Context, name string) error {
	_, err := c.grpc.DeleteKubernetesCluster(ctx, &types.ResourceRequest{Name: name})
	return trail.FromGRPC(err)
}

// DeleteAllKubernetesClusters deletes all kubernetes cluster resources.
func (c *Client) DeleteAllKubernetesClusters(ctx context.Context) error {
	_, err := c.grpc.DeleteAllKubernetesClusters(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// CreateDatabase creates a new database resource.
func (c *Client) CreateDatabase(ctx context.Context, database types.Database) error {
	databaseV3, ok := database.(*types.DatabaseV3)
	if !ok {
		return trace.BadParameter("unsupported database type %T", database)
	}
	_, err := c.grpc.CreateDatabase(ctx, databaseV3)
	return trail.FromGRPC(err)
}

// UpdateDatabase updates existing database resource.
func (c *Client) UpdateDatabase(ctx context.Context, database types.Database) error {
	databaseV3, ok := database.(*types.DatabaseV3)
	if !ok {
		return trace.BadParameter("unsupported database type %T", database)
	}
	_, err := c.grpc.UpdateDatabase(ctx, databaseV3)
	return trail.FromGRPC(err)
}

// GetDatabase returns the specified database resource.
//
// Note that database resources here refers to "dynamically-added" databases
// such as databases created by `tctl create`, the discovery service, or the
// CreateDatabase API. Databases discovered by the database agent (legacy
// discovery flow using `database_service.aws/database_service.azure`) and
// static databases defined in the `database_service.databases` section of the
// service YAML configuration are not collected in this API.
//
// For a full list of registered databases that are served by a database
// service, use GetDatabaseServers instead.
func (c *Client) GetDatabase(ctx context.Context, name string) (types.Database, error) {
	if name == "" {
		return nil, trace.BadParameter("missing database name")
	}
	database, err := c.grpc.GetDatabase(ctx, &types.ResourceRequest{Name: name})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return database, nil
}

// GetDatabases returns all database resources.
//
// Note that database resources here refers to "dynamically-added" databases
// such as databases created by `tctl create`, the discovery service, or the
// CreateDatabase API. Databases discovered by the database agent (legacy
// discovery flow using `database_service.aws/database_service.azure`) and
// static databases defined in the `database_service.databases` section of the
// service YAML configuration are not collected in this API.
//
// For a full list of registered databases that are served by a database
// service, use GetDatabaseServers instead.
func (c *Client) GetDatabases(ctx context.Context) ([]types.Database, error) {
	items, err := c.grpc.GetDatabases(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	databases := make([]types.Database, len(items.Databases))
	for i := range items.Databases {
		databases[i] = items.Databases[i]
	}
	return databases, nil
}

// DeleteDatabase deletes specified database resource.
func (c *Client) DeleteDatabase(ctx context.Context, name string) error {
	_, err := c.grpc.DeleteDatabase(ctx, &types.ResourceRequest{Name: name})
	return trail.FromGRPC(err)
}

// DeleteAllDatabases deletes all database resources.
func (c *Client) DeleteAllDatabases(ctx context.Context) error {
	_, err := c.grpc.DeleteAllDatabases(ctx, &emptypb.Empty{})
	return trail.FromGRPC(err)
}

// UpsertDatabaseService creates or updates existing DatabaseService resource.
func (c *Client) UpsertDatabaseService(ctx context.Context, service types.DatabaseService) (*types.KeepAlive, error) {
	serviceV1, ok := service.(*types.DatabaseServiceV1)
	if !ok {
		return nil, trace.BadParameter("unsupported DatabaseService type %T", serviceV1)
	}
	keepAlive, err := c.grpc.UpsertDatabaseService(ctx, &proto.UpsertDatabaseServiceRequest{
		Service: serviceV1,
	})

	return keepAlive, trail.FromGRPC(err)
}

// DeleteDatabaseService deletes a specific DatabaseService resource.
func (c *Client) DeleteDatabaseService(ctx context.Context, name string) error {
	_, err := c.grpc.DeleteDatabaseService(ctx, &types.ResourceRequest{Name: name})
	return trail.FromGRPC(err)
}

// DeleteAllDatabaseServices deletes all DatabaseService resources.
// If an error occurs, a partial delete may happen.
func (c *Client) DeleteAllDatabaseServices(ctx context.Context) error {
	_, err := c.grpc.DeleteAllDatabaseServices(ctx, &proto.DeleteAllDatabaseServicesRequest{})
	return trail.FromGRPC(err)
}

// GetWindowsDesktopServices returns all registered windows desktop services.
func (c *Client) GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error) {
	resp, err := c.grpc.GetWindowsDesktopServices(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	services := make([]types.WindowsDesktopService, 0, len(resp.GetServices()))
	for _, service := range resp.GetServices() {
		services = append(services, service)
	}
	return services, nil
}

// GetWindowsDesktopService returns a registered windows desktop service by name.
func (c *Client) GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error) {
	resp, err := c.grpc.GetWindowsDesktopService(ctx, &proto.GetWindowsDesktopServiceRequest{Name: name})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp.GetService(), nil
}

// UpsertWindowsDesktopService registers a new windows desktop service.
func (c *Client) UpsertWindowsDesktopService(ctx context.Context, service types.WindowsDesktopService) (*types.KeepAlive, error) {
	s, ok := service.(*types.WindowsDesktopServiceV3)
	if !ok {
		return nil, trace.BadParameter("invalid type %T", service)
	}
	keepAlive, err := c.grpc.UpsertWindowsDesktopService(ctx, s)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return keepAlive, nil
}

// DeleteWindowsDesktopService removes the specified windows desktop service.
func (c *Client) DeleteWindowsDesktopService(ctx context.Context, name string) error {
	_, err := c.grpc.DeleteWindowsDesktopService(ctx, &proto.DeleteWindowsDesktopServiceRequest{
		Name: name,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// DeleteAllWindowsDesktopServices removes all registered windows desktop services.
func (c *Client) DeleteAllWindowsDesktopServices(ctx context.Context) error {
	_, err := c.grpc.DeleteAllWindowsDesktopServices(ctx, &emptypb.Empty{})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// GetWindowsDesktops returns all registered windows desktop hosts.
func (c *Client) GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	resp, err := c.grpc.GetWindowsDesktops(ctx, &filter)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	desktops := make([]types.WindowsDesktop, 0, len(resp.GetDesktops()))
	for _, desktop := range resp.GetDesktops() {
		desktops = append(desktops, desktop)
	}
	return desktops, nil
}

// CreateWindowsDesktop registers a new windows desktop host.
func (c *Client) CreateWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error {
	d, ok := desktop.(*types.WindowsDesktopV3)
	if !ok {
		return trace.BadParameter("invalid type %T", desktop)
	}
	_, err := c.grpc.CreateWindowsDesktop(ctx, d)
	return trail.FromGRPC(err)
}

// UpdateWindowsDesktop updates an existing windows desktop host.
func (c *Client) UpdateWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error {
	d, ok := desktop.(*types.WindowsDesktopV3)
	if !ok {
		return trace.BadParameter("invalid type %T", desktop)
	}
	_, err := c.grpc.UpdateWindowsDesktop(ctx, d)
	return trail.FromGRPC(err)
}

// UpsertWindowsDesktop updates a windows desktop resource, creating it if it doesn't exist.
func (c *Client) UpsertWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error {
	d, ok := desktop.(*types.WindowsDesktopV3)
	if !ok {
		return trace.BadParameter("invalid type %T", desktop)
	}
	_, err := c.grpc.UpsertWindowsDesktop(ctx, d)
	return trail.FromGRPC(err)
}

// DeleteWindowsDesktop removes the specified windows desktop host.
// Note: unlike GetWindowsDesktops, this will delete at-most one desktop.
// Passing an empty host ID will not trigger "delete all" behavior. To delete
// all desktops, use DeleteAllWindowsDesktops.
func (c *Client) DeleteWindowsDesktop(ctx context.Context, hostID, name string) error {
	_, err := c.grpc.DeleteWindowsDesktop(ctx, &proto.DeleteWindowsDesktopRequest{
		Name:   name,
		HostID: hostID,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// DeleteAllWindowsDesktops removes all registered windows desktop hosts.
func (c *Client) DeleteAllWindowsDesktops(ctx context.Context) error {
	_, err := c.grpc.DeleteAllWindowsDesktops(ctx, &emptypb.Empty{})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// GenerateWindowsDesktopCert generates client certificate for Windows RDP authentication.
func (c *Client) GenerateWindowsDesktopCert(ctx context.Context, req *proto.WindowsDesktopCertRequest) (*proto.WindowsDesktopCertResponse, error) {
	resp, err := c.grpc.GenerateWindowsDesktopCert(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// ChangeUserAuthentication allows a user with a reset or invite token to change their password and if enabled also adds a new mfa device.
// Upon success, creates new web session and creates new set of recovery codes (if user meets requirements).
func (c *Client) ChangeUserAuthentication(ctx context.Context, req *proto.ChangeUserAuthenticationRequest) (*proto.ChangeUserAuthenticationResponse, error) {
	res, err := c.grpc.ChangeUserAuthentication(ctx, req)
	return res, trail.FromGRPC(err)
}

// StartAccountRecovery creates a recovery start token for a user who successfully verified their username and their recovery code.
// This token is used as part of a URL that will be emailed to the user (not done in this request).
// Represents step 1 of the account recovery process.
func (c *Client) StartAccountRecovery(ctx context.Context, req *proto.StartAccountRecoveryRequest) (types.UserToken, error) {
	res, err := c.grpc.StartAccountRecovery(ctx, req)
	return res, trail.FromGRPC(err)
}

// VerifyAccountRecovery creates a recovery approved token after successful verification of users password or second factor
// (authn depending on what user needed to recover). This token will allow users to perform protected actions while not logged in.
// Represents step 2 of the account recovery process after RPC StartAccountRecovery.
func (c *Client) VerifyAccountRecovery(ctx context.Context, req *proto.VerifyAccountRecoveryRequest) (types.UserToken, error) {
	res, err := c.grpc.VerifyAccountRecovery(ctx, req)
	return res, trail.FromGRPC(err)
}

// CompleteAccountRecovery sets a new password or adds a new mfa device,
// allowing user to regain access to their account using the new credentials.
// Represents the last step in the account recovery process after RPC's StartAccountRecovery and VerifyAccountRecovery.
func (c *Client) CompleteAccountRecovery(ctx context.Context, req *proto.CompleteAccountRecoveryRequest) error {
	_, err := c.grpc.CompleteAccountRecovery(ctx, req)
	return trail.FromGRPC(err)
}

// CreateAccountRecoveryCodes creates new set of recovery codes for a user, replacing and invalidating any previously owned codes.
func (c *Client) CreateAccountRecoveryCodes(ctx context.Context, req *proto.CreateAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
	res, err := c.grpc.CreateAccountRecoveryCodes(ctx, req)
	return res, trail.FromGRPC(err)
}

// GetAccountRecoveryToken returns a user token resource after verifying the token in
// request is not expired and is of the correct recovery type.
func (c *Client) GetAccountRecoveryToken(ctx context.Context, req *proto.GetAccountRecoveryTokenRequest) (types.UserToken, error) {
	res, err := c.grpc.GetAccountRecoveryToken(ctx, req)
	return res, trail.FromGRPC(err)
}

// GetAccountRecoveryCodes returns the user in context their recovery codes resource without any secrets.
func (c *Client) GetAccountRecoveryCodes(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
	res, err := c.grpc.GetAccountRecoveryCodes(ctx, req)
	return res, trail.FromGRPC(err)
}

// CreateAuthenticateChallenge creates and returns MFA challenges for a users registered MFA devices.
func (c *Client) CreateAuthenticateChallenge(ctx context.Context, in *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	resp, err := c.grpc.CreateAuthenticateChallenge(ctx, in)
	return resp, trail.FromGRPC(err)
}

// CreatePrivilegeToken is implemented by AuthService.CreatePrivilegeToken.
func (c *Client) CreatePrivilegeToken(ctx context.Context, req *proto.CreatePrivilegeTokenRequest) (*types.UserTokenV3, error) {
	resp, err := c.grpc.CreatePrivilegeToken(ctx, req)
	return resp, trail.FromGRPC(err)
}

// CreateRegisterChallenge creates and returns MFA register challenge for a new MFA device.
func (c *Client) CreateRegisterChallenge(ctx context.Context, in *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
	resp, err := c.grpc.CreateRegisterChallenge(ctx, in)
	return resp, trail.FromGRPC(err)
}

// GenerateCertAuthorityCRL generates an empty CRL for a CA.
func (c *Client) GenerateCertAuthorityCRL(ctx context.Context, req *proto.CertAuthorityRequest) (*proto.CRL, error) {
	resp, err := c.grpc.GenerateCertAuthorityCRL(ctx, req)
	return resp, trail.FromGRPC(err)
}

// ListResources returns a paginated list of nodes that the user has access to.
// `nextKey` is used as `startKey` in another call to ListResources to retrieve
// the next page. If you want to list all resources pages, check the
// `GetResourcesWithFilters` function.
// It will return a `trace.LimitExceeded` error if the page exceeds gRPC max
// message size.
func (c *Client) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.grpc.ListResources(ctx, &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	resources := make([]types.ResourceWithLabels, len(resp.GetResources()))
	for i, respResource := range resp.GetResources() {
		switch req.ResourceType {
		case types.KindDatabaseServer:
			resources[i] = respResource.GetDatabaseServer()
		case types.KindDatabaseService:
			resources[i] = respResource.GetDatabaseService()
		case types.KindAppServer:
			resources[i] = respResource.GetAppServer()
		case types.KindNode:
			resources[i] = respResource.GetNode()
		case types.KindWindowsDesktop:
			resources[i] = respResource.GetWindowsDesktop()
		case types.KindWindowsDesktopService:
			resources[i] = respResource.GetWindowsDesktopService()
		case types.KindKubernetesCluster:
			resources[i] = respResource.GetKubeCluster()
		case types.KindKubeServer:
			resources[i] = respResource.GetKubernetesServer()
		case types.KindUserGroup:
			resources[i] = respResource.GetUserGroup()
		default:
			return nil, trace.NotImplemented("resource type %s does not support pagination", req.ResourceType)
		}
	}

	return &types.ListResourcesResponse{
		Resources:  resources,
		NextKey:    resp.NextKey,
		TotalCount: int(resp.TotalCount),
	}, nil
}

// GetResources returns a paginated list of resources that the user has access to.
// `nextKey` is used as `startKey` in another call to GetResources to retrieve
// the next page.
// It will return a `trace.LimitExceeded` error if the page exceeds gRPC max
// message size.
func (c *Client) GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.grpc.ListResources(ctx, req)
	return resp, trail.FromGRPC(err)
}

// GetResourcesClient is an interface used by GetResources to abstract over implementations of
// the ListResources method.
type GetResourcesClient interface {
	GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error)
}

// ResourcePage holds a page of results from [GetResourcePage].
type ResourcePage[T types.ResourceWithLabels] struct {
	// Resources retrieved for a single [proto.ListResourcesRequest]. The length of
	// the slice will be at most [proto.ListResourcesRequest.Limit].
	Resources []T
	// Total number of all resources matching the request. It will be greater than
	// the length of [Resources] if the number of matches exceeds the request limit.
	Total int
	// NextKey is the start of the next page
	NextKey string
}

// GetResourcePage is a helper for getting a single page of resources that match the provide request.
func GetResourcePage[T types.ResourceWithLabels](ctx context.Context, clt GetResourcesClient, req *proto.ListResourcesRequest) (ResourcePage[T], error) {
	var out ResourcePage[T]

	// Set the limit to the default size if one was not provided within
	// an acceptable range.
	if req.Limit == 0 || req.Limit > int32(defaults.DefaultChunkSize) {
		req.Limit = int32(defaults.DefaultChunkSize)
	}

	for {
		resp, err := clt.GetResources(ctx, req)
		if err != nil {
			if trace.IsLimitExceeded(err) {
				// Cut chunkSize in half if gRPC max message size is exceeded.
				req.Limit /= 2
				// This is an extremely unlikely scenario, but better to cover it anyways.
				if req.Limit == 0 {
					return out, trace.Wrap(trail.FromGRPC(err), "resource is too large to retrieve")
				}

				continue
			}

			return out, trail.FromGRPC(err)
		}

		for _, respResource := range resp.Resources {
			var resource types.ResourceWithLabels
			switch req.ResourceType {
			case types.KindDatabaseServer:
				resource = respResource.GetDatabaseServer()
			case types.KindDatabaseService:
				resource = respResource.GetDatabaseService()
			case types.KindAppServer:
				resource = respResource.GetAppServer()
			case types.KindNode:
				resource = respResource.GetNode()
			case types.KindWindowsDesktop:
				resource = respResource.GetWindowsDesktop()
			case types.KindWindowsDesktopService:
				resource = respResource.GetWindowsDesktopService()
			case types.KindKubernetesCluster:
				resource = respResource.GetKubeCluster()
			case types.KindKubeServer:
				resource = respResource.GetKubernetesServer()
			case types.KindUserGroup:
				resource = respResource.GetUserGroup()
			default:
				out.Resources = nil
				return out, trace.NotImplemented("resource type %s does not support pagination", req.ResourceType)
			}

			t, ok := resource.(T)
			if !ok {
				out.Resources = nil
				return out, trace.BadParameter("received unexpected resource type %T", resource)
			}
			out.Resources = append(out.Resources, t)
		}

		out.NextKey = resp.NextKey
		out.Total = int(resp.TotalCount)

		return out, nil
	}
}

// GetAllResources is a helper for getting all existing resources that match the provided request. In addition to
// iterating pages, it also correctly handles downsizing pages when LimitExceeded errors are encountered.
func GetAllResources[T types.ResourceWithLabels](ctx context.Context, clt GetResourcesClient, req *proto.ListResourcesRequest) ([]T, error) {
	var out []T

	// Set the limit to the default size.
	req.Limit = int32(defaults.DefaultChunkSize)
	for {
		page, err := GetResourcePage[T](ctx, clt, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		out = append(out, page.Resources...)

		if page.NextKey == "" || len(page.Resources) == 0 {
			break
		}

		req.StartKey = page.NextKey
	}

	return out, nil
}

// ListResourcesClient is an interface used by GetResourcesWithFilters to abstract over implementations of
// the ListResources method.
type ListResourcesClient interface {
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

// GetResourcesWithFilters is a helper for getting a list of resources with optional filtering. In addition to
// iterating pages, it also correctly handles downsizing pages when LimitExceeded errors are encountered.
//
// GetAllResources or GetResourcePage should be preferred for client side operations to avoid converting
// from []types.ResourceWithLabels to concrete types.
func GetResourcesWithFilters(ctx context.Context, clt ListResourcesClient, req proto.ListResourcesRequest) ([]types.ResourceWithLabels, error) {
	// Retrieve the complete list of resources in chunks.
	var (
		resources []types.ResourceWithLabels
		startKey  string
		chunkSize = int32(defaults.DefaultChunkSize)
	)

	for {
		resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
			Namespace:           req.Namespace,
			ResourceType:        req.ResourceType,
			StartKey:            startKey,
			Limit:               chunkSize,
			Labels:              req.Labels,
			SearchKeywords:      req.SearchKeywords,
			PredicateExpression: req.PredicateExpression,
			UseSearchAsRoles:    req.UseSearchAsRoles,
		})
		if err != nil {
			if trace.IsLimitExceeded(err) {
				// Cut chunkSize in half if gRPC max message size is exceeded.
				chunkSize = chunkSize / 2
				// This is an extremely unlikely scenario, but better to cover it anyways.
				if chunkSize == 0 {
					return nil, trace.Wrap(trail.FromGRPC(err), "resource is too large to retrieve")
				}

				continue
			}

			return nil, trail.FromGRPC(err)
		}

		startKey = resp.NextKey
		resources = append(resources, resp.Resources...)
		if startKey == "" || len(resp.Resources) == 0 {
			break
		}

	}

	return resources, nil
}

// GetKubernetesResourcesWithFilters is a helper for getting a list of kubernetes resources with optional filtering. In addition to
// iterating pages, it also correctly handles downsizing pages when LimitExceeded errors are encountered.
func GetKubernetesResourcesWithFilters(ctx context.Context, clt kubeproto.KubeServiceClient, req *kubeproto.ListKubernetesResourcesRequest) ([]types.ResourceWithLabels, error) {
	var (
		resources []types.ResourceWithLabels
		startKey  = req.StartKey
		// Retrieve the complete list of resources in chunks.
		chunkSize = req.Limit
	)

	// Set the chunk size to the default if it is not set.
	if chunkSize == 0 {
		chunkSize = int32(defaults.DefaultChunkSize)
	}

	for {
		// Reset startKey to the previous page's nextKey.
		req.StartKey = startKey
		// Set the chunk size based on the previous chunk size error.
		req.Limit = chunkSize

		resp, err := clt.ListKubernetesResources(
			ctx,
			req,
		)
		if err != nil {
			if trace.IsLimitExceeded(err) {
				// Cut chunkSize in half if gRPC max message size is exceeded.
				chunkSize = chunkSize / 2
				// This is an extremely unlikely scenario, but better to cover it anyways.
				if chunkSize == 0 {
					return nil, trace.Wrap(trail.FromGRPC(err), "resource is too large to retrieve")
				}
				continue
			}
			return nil, trail.FromGRPC(err)
		}

		startKey = resp.NextKey

		resources = append(resources, types.KubeResources(resp.Resources).AsResources()...)
		if startKey == "" || len(resp.Resources) == 0 {
			break
		}
	}
	return resources, nil
}

// CreateSessionTracker creates a tracker resource for an active session.
func (c *Client) CreateSessionTracker(ctx context.Context, st types.SessionTracker) (types.SessionTracker, error) {
	v1, ok := st.(*types.SessionTrackerV1)
	if !ok {
		return nil, trace.BadParameter("invalid type %T, expected *types.SessionTrackerV1", st)
	}

	req := &proto.CreateSessionTrackerRequest{SessionTracker: v1}
	tracker, err := c.grpc.CreateSessionTracker(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return tracker, nil
}

// GetSessionTracker returns the current state of a session tracker for an active session.
func (c *Client) GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error) {
	req := &proto.GetSessionTrackerRequest{SessionID: sessionID}
	resp, err := c.grpc.GetSessionTracker(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetActiveSessionTrackers returns a list of active session trackers.
func (c *Client) GetActiveSessionTrackers(ctx context.Context) ([]types.SessionTracker, error) {
	stream, err := c.grpc.GetActiveSessionTrackers(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	var sessions []types.SessionTracker
	for {
		session, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, trace.Wrap(err)
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetActiveSessionTrackersWithFilter returns a list of active sessions filtered by a filter.
func (c *Client) GetActiveSessionTrackersWithFilter(ctx context.Context, filter *types.SessionTrackerFilter) ([]types.SessionTracker, error) {
	stream, err := c.grpc.GetActiveSessionTrackersWithFilter(ctx, filter)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	var sessions []types.SessionTracker
	for {
		session, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, trace.Wrap(err)
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// RemoveSessionTracker removes a tracker resource for an active session.
func (c *Client) RemoveSessionTracker(ctx context.Context, sessionID string) error {
	_, err := c.grpc.RemoveSessionTracker(ctx, &proto.RemoveSessionTrackerRequest{SessionID: sessionID})
	return trail.FromGRPC(err)
}

// UpdateSessionTracker updates a tracker resource for an active session.
func (c *Client) UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error {
	_, err := c.grpc.UpdateSessionTracker(ctx, req)
	return trail.FromGRPC(err)
}

// MaintainSessionPresence establishes a channel used to continuously verify the presence for a session.
func (c *Client) MaintainSessionPresence(ctx context.Context) (proto.AuthService_MaintainSessionPresenceClient, error) {
	stream, err := c.grpc.MaintainSessionPresence(ctx)
	return stream, trail.FromGRPC(err)
}

// GetDomainName returns local auth domain of the current auth server
func (c *Client) GetDomainName(ctx context.Context) (string, error) {
	resp, err := c.grpc.GetDomainName(ctx, &emptypb.Empty{})
	if err != nil {
		return "", trail.FromGRPC(err)
	}
	return resp.DomainName, nil
}

// GetClusterCACert returns the PEM-encoded TLS certs for the local cluster. If
// the cluster has multiple TLS certs, they will all be concatenated.
func (c *Client) GetClusterCACert(ctx context.Context) (*proto.GetClusterCACertResponse, error) {
	resp, err := c.grpc.GetClusterCACert(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetConnectionDiagnostic reads a connection diagnostic
func (c *Client) GetConnectionDiagnostic(ctx context.Context, name string) (types.ConnectionDiagnostic, error) {
	req := &proto.GetConnectionDiagnosticRequest{
		Name: name,
	}
	res, err := c.grpc.GetConnectionDiagnostic(ctx, req)
	return res, trail.FromGRPC(err)
}

// CreateConnectionDiagnostic creates a new connection diagnostic.
func (c *Client) CreateConnectionDiagnostic(ctx context.Context, connectionDiagnostic types.ConnectionDiagnostic) error {
	connectionDiagnosticV1, ok := connectionDiagnostic.(*types.ConnectionDiagnosticV1)
	if !ok {
		return trace.BadParameter("invalid type %T", connectionDiagnostic)
	}
	_, err := c.grpc.CreateConnectionDiagnostic(ctx, connectionDiagnosticV1)
	return trail.FromGRPC(err)
}

// UpdateConnectionDiagnostic updates a connection diagnostic.
func (c *Client) UpdateConnectionDiagnostic(ctx context.Context, connectionDiagnostic types.ConnectionDiagnostic) error {
	connectionDiagnosticV1, ok := connectionDiagnostic.(*types.ConnectionDiagnosticV1)
	if !ok {
		return trace.BadParameter("invalid type %T", connectionDiagnostic)
	}
	_, err := c.grpc.UpdateConnectionDiagnostic(ctx, connectionDiagnosticV1)
	return trail.FromGRPC(err)
}

// AppendDiagnosticTrace adds a new trace for the given ConnectionDiagnostic.
func (c *Client) AppendDiagnosticTrace(ctx context.Context, name string, t *types.ConnectionDiagnosticTrace) (types.ConnectionDiagnostic, error) {
	req := &proto.AppendDiagnosticTraceRequest{
		Name:  name,
		Trace: t,
	}
	connectionDiagnostic, err := c.grpc.AppendDiagnosticTrace(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connectionDiagnostic, nil
}

// GetClusterAlerts loads matching cluster alerts.
func (c *Client) GetClusterAlerts(ctx context.Context, query types.GetClusterAlertsRequest) ([]types.ClusterAlert, error) {
	rsp, err := c.grpc.GetClusterAlerts(ctx, &query)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return rsp.Alerts, nil
}

// UpsertClusterAlert creates a cluster alert.
func (c *Client) UpsertClusterAlert(ctx context.Context, alert types.ClusterAlert) error {
	_, err := c.grpc.UpsertClusterAlert(ctx, &proto.UpsertClusterAlertRequest{
		Alert: alert,
	})
	return trail.FromGRPC(err)
}

func (c *Client) ChangePassword(ctx context.Context, req *proto.ChangePasswordRequest) error {
	_, err := c.grpc.ChangePassword(ctx, req)
	return trail.FromGRPC(err)
}

// SubmitUsageEvent submits an external usage event.
func (c *Client) SubmitUsageEvent(ctx context.Context, req *proto.SubmitUsageEventRequest) error {
	_, err := c.grpc.SubmitUsageEvent(ctx, req)

	return trail.FromGRPC(err)
}

// GetLicense returns the license used to start the teleport enterprise auth server
func (c *Client) GetLicense(ctx context.Context) (string, error) {
	resp, err := c.grpc.GetLicense(ctx, &proto.GetLicenseRequest{})
	if err != nil {
		return "", trail.FromGRPC(err)
	}
	return string(resp.License), nil
}

// ListReleases returns a list of teleport enterprise releases
func (c *Client) ListReleases(ctx context.Context, req *proto.ListReleasesRequest) ([]*types.Release, error) {
	resp, err := c.grpc.ListReleases(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return resp.Releases, nil
}

// CreateAlertAck marks a cluster alert as acknowledged.
func (c *Client) CreateAlertAck(ctx context.Context, ack types.AlertAcknowledgement) error {
	_, err := c.grpc.CreateAlertAck(ctx, &ack)
	return trail.FromGRPC(err)
}

// GetAlertAcks gets active alert acknowledgements.
func (c *Client) GetAlertAcks(ctx context.Context) ([]types.AlertAcknowledgement, error) {
	rsp, err := c.grpc.GetAlertAcks(ctx, &proto.GetAlertAcksRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return rsp.Acks, nil
}

// ClearAlertAcks clears alert acknowledgments.
func (c *Client) ClearAlertAcks(ctx context.Context, req proto.ClearAlertAcksRequest) error {
	_, err := c.grpc.ClearAlertAcks(ctx, &req)
	return trail.FromGRPC(err)
}

// ListSAMLIdPServiceProviders returns a paginated list of SAML IdP service provider resources.
func (c *Client) ListSAMLIdPServiceProviders(ctx context.Context, pageSize int, nextKey string) ([]types.SAMLIdPServiceProvider, string, error) {
	resp, err := c.grpc.ListSAMLIdPServiceProviders(ctx, &proto.ListSAMLIdPServiceProvidersRequest{
		Limit:   int32(pageSize),
		NextKey: nextKey,
	})
	if err != nil {
		return nil, "", trail.FromGRPC(err)
	}
	serviceProviders := make([]types.SAMLIdPServiceProvider, 0, len(resp.GetServiceProviders()))
	for _, sp := range resp.GetServiceProviders() {
		serviceProviders = append(serviceProviders, sp)
	}
	return serviceProviders, resp.GetNextKey(), nil
}

// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
func (c *Client) GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	sp, err := c.grpc.GetSAMLIdPServiceProvider(ctx, &proto.GetSAMLIdPServiceProviderRequest{
		Name: name,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return sp, nil
}

// CreateSAMLIdPServiceProvider creates a new SAML IdP service provider resource.
func (c *Client) CreateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	spV1, ok := sp.(*types.SAMLIdPServiceProviderV1)
	if !ok {
		return trace.BadParameter("unsupported SAML IdP service provider type %T", sp)
	}

	_, err := c.grpc.CreateSAMLIdPServiceProvider(ctx, spV1)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// UpdateSAMLIdPServiceProvider updates an existing SAML IdP service provider resource.
func (c *Client) UpdateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	spV1, ok := sp.(*types.SAMLIdPServiceProviderV1)
	if !ok {
		return trace.BadParameter("unsupported SAML IdP service provider type %T", sp)
	}

	_, err := c.grpc.UpdateSAMLIdPServiceProvider(ctx, spV1)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// DeleteSAMLIdPServiceProvider removes the specified SAML IdP service provider resource.
func (c *Client) DeleteSAMLIdPServiceProvider(ctx context.Context, name string) error {
	_, err := c.grpc.DeleteSAMLIdPServiceProvider(ctx, &proto.DeleteSAMLIdPServiceProviderRequest{
		Name: name,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// DeleteAllSAMLIdPServiceProviders removes all SAML IdP service providers.
func (c *Client) DeleteAllSAMLIdPServiceProviders(ctx context.Context) error {
	_, err := c.grpc.DeleteAllSAMLIdPServiceProviders(ctx, &emptypb.Empty{})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// ListUserGroups returns a paginated list of SAML IdP service provider resources.
func (c *Client) ListUserGroups(ctx context.Context, pageSize int, nextKey string) ([]types.UserGroup, string, error) {
	resp, err := c.grpc.ListUserGroups(ctx, &proto.ListUserGroupsRequest{
		Limit:   int32(pageSize),
		NextKey: nextKey,
	})
	if err != nil {
		return nil, "", trail.FromGRPC(err)
	}
	userGroups := make([]types.UserGroup, 0, len(resp.GetUserGroups()))
	for _, ug := range resp.GetUserGroups() {
		userGroups = append(userGroups, ug)
	}
	return userGroups, resp.GetNextKey(), nil
}

// GetUserGroup returns the specified SAML IdP service provider resources.
func (c *Client) GetUserGroup(ctx context.Context, name string) (types.UserGroup, error) {
	ug, err := c.grpc.GetUserGroup(ctx, &proto.GetUserGroupRequest{
		Name: name,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return ug, nil
}

// CreateUserGroup creates a new user group resource.
func (c *Client) CreateUserGroup(ctx context.Context, ug types.UserGroup) error {
	ugV1, ok := ug.(*types.UserGroupV1)
	if !ok {
		return trace.BadParameter("unsupported user group type %T", ug)
	}

	_, err := c.grpc.CreateUserGroup(ctx, ugV1)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// UpdateUserGroup updates an existing user group resource.
func (c *Client) UpdateUserGroup(ctx context.Context, ug types.UserGroup) error {
	ugV1, ok := ug.(*types.UserGroupV1)
	if !ok {
		return trace.BadParameter("unsupported user group type %T", ug)
	}

	_, err := c.grpc.UpdateUserGroup(ctx, ugV1)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// DeleteUserGroup removes the specified user group resource.
func (c *Client) DeleteUserGroup(ctx context.Context, name string) error {
	_, err := c.grpc.DeleteUserGroup(ctx, &proto.DeleteUserGroupRequest{
		Name: name,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// DeleteAllUserGroups removes all user groups.
func (c *Client) DeleteAllUserGroups(ctx context.Context) error {
	_, err := c.grpc.DeleteAllUserGroups(ctx, &emptypb.Empty{})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// ExportUpgradeWindows is used to load derived upgrade window values for agents that
// need to export schedules to external upgraders.
func (c *Client) ExportUpgradeWindows(ctx context.Context, req proto.ExportUpgradeWindowsRequest) (proto.ExportUpgradeWindowsResponse, error) {
	rsp, err := c.grpc.ExportUpgradeWindows(ctx, &req)
	if err != nil {
		return proto.ExportUpgradeWindowsResponse{}, trail.FromGRPC(err)
	}
	return *rsp, nil
}

// GetClusterMaintenanceConfig gets the current maintenance window config singleton.
func (c *Client) GetClusterMaintenanceConfig(ctx context.Context) (types.ClusterMaintenanceConfig, error) {
	rsp, err := c.grpc.GetClusterMaintenanceConfig(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return rsp, nil
}

// UpdateClusterMaintenanceConfig updates the current maintenance window config singleton.
func (c *Client) UpdateClusterMaintenanceConfig(ctx context.Context, cmc types.ClusterMaintenanceConfig) error {
	req, ok := cmc.(*types.ClusterMaintenanceConfigV1)
	if !ok {
		return trace.BadParameter("unexpected maintenance config type: %T", cmc)
	}

	_, err := c.grpc.UpdateClusterMaintenanceConfig(ctx, req)
	return trail.FromGRPC(err)
}

// integrationsClient returns an unadorned Integration client, using the underlying
// Auth gRPC connection.
func (c *Client) integrationsClient() integrationpb.IntegrationServiceClient {
	return integrationpb.NewIntegrationServiceClient(c.conn)
}

// ListIntegrations returns a paginated list of Integrations.
// The response includes a nextKey which must be used to fetch the next page.
func (c *Client) ListIntegrations(ctx context.Context, pageSize int, nextKey string) ([]types.Integration, string, error) {
	resp, err := c.integrationsClient().ListIntegrations(ctx, &integrationpb.ListIntegrationsRequest{
		Limit:   int32(pageSize),
		NextKey: nextKey,
	})
	if err != nil {
		return nil, "", trail.FromGRPC(err)
	}

	integrations := make([]types.Integration, 0, len(resp.GetIntegrations()))
	for _, ig := range resp.GetIntegrations() {
		integrations = append(integrations, ig)
	}

	return integrations, resp.GetNextKey(), nil
}

// GetIntegration returns an Integration by its name.
func (c *Client) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	ig, err := c.integrationsClient().GetIntegration(ctx, &integrationpb.GetIntegrationRequest{
		Name: name,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return ig, nil
}

// CreateIntegration creates a new Integration.
func (c *Client) CreateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	igV1, ok := ig.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unsupported integration type %T", ig)
	}

	ig, err := c.integrationsClient().CreateIntegration(ctx, &integrationpb.CreateIntegrationRequest{Integration: igV1})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return ig, nil
}

// UpdateIntegration updates an existing Integration.
func (c *Client) UpdateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	igV1, ok := ig.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unsupported integration type %T", ig)
	}

	ig, err := c.integrationsClient().UpdateIntegration(ctx, &integrationpb.UpdateIntegrationRequest{Integration: igV1})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return ig, nil
}

// DeleteIntegration removes an Integration by its name.
func (c *Client) DeleteIntegration(ctx context.Context, name string) error {
	_, err := c.integrationsClient().DeleteIntegration(ctx, &integrationpb.DeleteIntegrationRequest{
		Name: name,
	})
	return trail.FromGRPC(err)
}

// DeleteAllIntegrations removes all Integrations.
func (c *Client) DeleteAllIntegrations(ctx context.Context) error {
	_, err := c.integrationsClient().DeleteAllIntegrations(ctx, &integrationpb.DeleteAllIntegrationsRequest{})
	return trail.FromGRPC(err)
}

// GenerateAWSOIDCToken generates a token to be used when executing an AWS OIDC Integration action.
func (c *Client) GenerateAWSOIDCToken(ctx context.Context, req types.GenerateAWSOIDCTokenRequest) (string, error) {
	resp, err := c.integrationsClient().GenerateAWSOIDCToken(ctx, &integrationpb.GenerateAWSOIDCTokenRequest{
		Issuer: req.Issuer,
	})
	if err != nil {
		return "", trail.FromGRPC(err)
	}

	return resp.GetToken(), nil
}

// PluginsClient returns an unadorned Plugins client, using the underlying
// Auth gRPC connection.
// Clients connecting to non-Enterprise clusters, or older Teleport versions,
// still get a plugins client when calling this method, but all RPCs will return
// "not implemented" errors (as per the default gRPC behavior).
func (c *Client) PluginsClient() pluginspb.PluginServiceClient {
	return pluginspb.NewPluginServiceClient(c.conn)
}

// GetLoginRule retrieves a login rule described by name.
func (c *Client) GetLoginRule(ctx context.Context, name string) (*loginrulepb.LoginRule, error) {
	rule, err := c.LoginRuleClient().GetLoginRule(ctx, &loginrulepb.GetLoginRuleRequest{
		Name: name,
	})
	return rule, trail.FromGRPC(err)
}

// CreateLoginRule creates a login rule if one with the same name does not
// already exist, else it returns an error.
func (c *Client) CreateLoginRule(ctx context.Context, rule *loginrulepb.LoginRule) (*loginrulepb.LoginRule, error) {
	rule, err := c.LoginRuleClient().CreateLoginRule(ctx, &loginrulepb.CreateLoginRuleRequest{
		LoginRule: rule,
	})
	return rule, trail.FromGRPC(err)
}

// UpsertLoginRule creates a login rule if one with the same name does not
// already exist, else it replaces the existing login rule.
func (c *Client) UpsertLoginRule(ctx context.Context, rule *loginrulepb.LoginRule) (*loginrulepb.LoginRule, error) {
	rule, err := c.LoginRuleClient().UpsertLoginRule(ctx, &loginrulepb.UpsertLoginRuleRequest{
		LoginRule: rule,
	})
	return rule, trail.FromGRPC(err)
}

// DeleteLoginRule deletes an existing login rule by name.
func (c *Client) DeleteLoginRule(ctx context.Context, name string) error {
	_, err := c.LoginRuleClient().DeleteLoginRule(ctx, &loginrulepb.DeleteLoginRuleRequest{
		Name: name,
	})
	return trail.FromGRPC(err)
}

// OktaClient returns an Okta client.
// Clients connecting older Teleport versions still get an okta client when
// calling this method, but all RPCs will return "not implemented" errors (as per
// the default gRPC behavior).
func (c *Client) OktaClient() *okta.Client {
	return okta.NewClient(oktapb.NewOktaServiceClient(c.conn))
}

// AccessListClient returns an access list client.
// Clients connecting to  older Teleport versions, still get an access list client
// when calling this method, but all RPCs will return "not implemented" errors
// (as per the default gRPC behavior).
func (c *Client) AccessListClient() accesslistv1.AccessListServiceClient {
	return accesslistv1.NewAccessListServiceClient(c.conn)
}

// GetCertAuthority retrieves a CA by type and domain.
func (c *Client) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	ca, err := c.TrustClient().GetCertAuthority(ctx, &trustpb.GetCertAuthorityRequest{
		Type:       string(id.Type),
		Domain:     id.DomainName,
		IncludeKey: loadKeys,
	})

	return ca, trail.FromGRPC(err)
}

// GetCertAuthorities retrieves CAs by type.
func (c *Client) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error) {
	resp, err := c.TrustClient().GetCertAuthorities(ctx, &trustpb.GetCertAuthoritiesRequest{
		Type:       string(caType),
		IncludeKey: loadKeys,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	cas := make([]types.CertAuthority, 0, len(resp.CertAuthoritiesV2))
	for _, ca := range resp.CertAuthoritiesV2 {
		cas = append(cas, ca)
	}

	return cas, nil
}

// DeleteCertAuthority removes a CA matching the type and domain.
func (c *Client) DeleteCertAuthority(ctx context.Context, id types.CertAuthID) error {
	_, err := c.TrustClient().DeleteCertAuthority(ctx, &trustpb.DeleteCertAuthorityRequest{
		Type:   string(id.Type),
		Domain: id.DomainName,
	})

	return trail.FromGRPC(err)
}

// UpsertCertAuthority creates or updates the provided cert authority.
func (c *Client) UpsertCertAuthority(ctx context.Context, ca types.CertAuthority) (types.CertAuthority, error) {
	cav2, ok := ca.(*types.CertAuthorityV2)
	if !ok {
		return nil, trace.BadParameter("unexpected ca type %T", ca)
	}

	out, err := c.TrustClient().UpsertCertAuthority(ctx, &trustpb.UpsertCertAuthorityRequest{
		CertAuthority: cav2,
	})

	return out, trail.FromGRPC(err)
}

// UpdateHeadlessAuthenticationState updates a headless authentication state.
func (c *Client) UpdateHeadlessAuthenticationState(ctx context.Context, id string, state types.HeadlessAuthenticationState, mfaResponse *proto.MFAAuthenticateResponse) error {
	_, err := c.grpc.UpdateHeadlessAuthenticationState(ctx, &proto.UpdateHeadlessAuthenticationStateRequest{
		Id:          id,
		State:       state,
		MfaResponse: mfaResponse,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// GetHeadlessAuthentication retrieves a headless authentication by id.
func (c *Client) GetHeadlessAuthentication(ctx context.Context, id string) (*types.HeadlessAuthentication, error) {
	headlessAuthn, err := c.grpc.GetHeadlessAuthentication(ctx, &proto.GetHeadlessAuthenticationRequest{
		Id: id,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return headlessAuthn, nil
}

// WatchPendingHeadlessAuthentications creates a watcher for pending headless authentication for the current user.
func (c *Client) WatchPendingHeadlessAuthentications(ctx context.Context) (types.Watcher, error) {
	cancelCtx, cancel := context.WithCancel(ctx)
	stream, err := c.grpc.WatchPendingHeadlessAuthentications(cancelCtx, &emptypb.Empty{})
	if err != nil {
		cancel()
		return nil, trail.FromGRPC(err)
	}
	w := &streamWatcher{
		stream:  stream,
		ctx:     cancelCtx,
		cancel:  cancel,
		eventsC: make(chan types.Event),
	}
	go w.receiveEvents()
	return w, nil
}

// CreateAssistantConversation creates a new conversation entry in the backend.
func (c *Client) CreateAssistantConversation(ctx context.Context, req *assist.CreateAssistantConversationRequest) (*assist.CreateAssistantConversationResponse, error) {
	resp, err := c.grpc.CreateAssistantConversation(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return resp, nil
}

// GetAssistantMessages retrieves assistant messages with given conversation ID.
func (c *Client) GetAssistantMessages(ctx context.Context, req *assist.GetAssistantMessagesRequest) (*assist.GetAssistantMessagesResponse, error) {
	messages, err := c.grpc.GetAssistantMessages(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return messages, nil
}

// DeleteAssistantConversation deletes a conversation entry in the backend.
func (c *Client) DeleteAssistantConversation(ctx context.Context, req *assist.DeleteAssistantConversationRequest) error {
	_, err := c.grpc.DeleteAssistantConversation(ctx, req)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// IsAssistEnabled returns true if the assist is enabled or not on the auth level.
func (c *Client) IsAssistEnabled(ctx context.Context) (*assist.IsAssistEnabledResponse, error) {
	resp, err := c.grpc.IsAssistEnabled(ctx, &assist.IsAssistEnabledRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// GetAssistantConversations returns all conversations started by a user.
func (c *Client) GetAssistantConversations(ctx context.Context, request *assist.GetAssistantConversationsRequest) (*assist.GetAssistantConversationsResponse, error) {
	messages, err := c.grpc.GetAssistantConversations(ctx, request)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return messages, nil
}

// CreateAssistantMessage saves a new conversation message.
func (c *Client) CreateAssistantMessage(ctx context.Context, in *assist.CreateAssistantMessageRequest) error {
	_, err := c.grpc.CreateAssistantMessage(ctx, in)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

// UpdateAssistantConversationInfo updates conversation info.
func (c *Client) UpdateAssistantConversationInfo(ctx context.Context, in *assist.UpdateAssistantConversationInfoRequest) error {
	_, err := c.grpc.UpdateAssistantConversationInfo(ctx, in)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

func (c *Client) GetAssistantEmbeddings(ctx context.Context, in *assist.GetAssistantEmbeddingsRequest) (*assist.GetAssistantEmbeddingsResponse, error) {
	result, err := c.EmbeddingClient().GetAssistantEmbeddings(ctx, in)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return result, nil
}

// GetUserPreferences returns the user preferences for a given user.
func (c *Client) GetUserPreferences(ctx context.Context, in *userpreferencespb.GetUserPreferencesRequest) (*userpreferencespb.GetUserPreferencesResponse, error) {
	resp, err := c.grpc.GetUserPreferences(ctx, in)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp, nil
}

// UpsertUserPreferences creates or updates user preferences for a given username.
func (c *Client) UpsertUserPreferences(ctx context.Context, in *userpreferencespb.UpsertUserPreferencesRequest) error {
	_, err := c.grpc.UpsertUserPreferences(ctx, in)
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}
