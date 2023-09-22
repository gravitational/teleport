/*
Copyright 2021 Gravitational, Inc.

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

package gateway

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/utils/keys"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// New creates an instance of Gateway. It starts a listener on the specified port but it doesn't
// start the proxy – that's the job of Serve.
func New(cfg Config) (*Gateway, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := cfg.TCPPortAllocator.Listen(cfg.LocalAddress, cfg.LocalPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, closeCancel := context.WithCancel(context.Background())
	// make sure the listener is closed if gateway creation failed
	ok := false
	defer func() {
		if ok {
			return
		}

		closeCancel()
		if err := listener.Close(); err != nil {
			cfg.Log.WithError(err).Warn("Failed to close listener.")
		}
	}()

	// retrieve automatically assigned port number
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.LocalPort = port

	protocol, err := alpncommon.ToALPNProtocol(cfg.Protocol)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	address, err := utils.ParseAddr(cfg.WebProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsCert, err := keys.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkCertSubject(tlsCert, cfg.RouteToDatabase()); err != nil {
		return nil, trace.Wrap(err,
			"database certificate check failed, try restarting the database connection")
	}

	localProxyConfig := alpn.LocalProxyConfig{
		InsecureSkipVerify: cfg.Insecure,
		RemoteProxyAddr:    cfg.WebProxyAddr,
		Protocols:          []alpncommon.Protocol{protocol},
		Listener:           listener,
		ParentContext:      closeContext,
		SNI:                address.Host(),
		Certs:              []tls.Certificate{tlsCert},
		Clock:              cfg.Clock,
	}

	localProxyMiddleware := &localProxyMiddleware{
		log:     cfg.Log,
		dbRoute: cfg.RouteToDatabase(),
	}

	if cfg.OnExpiredCert != nil {
		localProxyConfig.Middleware = localProxyMiddleware
	}

	localProxy, err := alpn.NewLocalProxy(localProxyConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gateway := &Gateway{
		cfg:          &cfg,
		closeContext: closeContext,
		closeCancel:  closeCancel,
		localProxy:   localProxy,
	}

	if cfg.OnExpiredCert != nil {
		localProxyMiddleware.onExpiredCert = func(ctx context.Context) error {
			err := cfg.OnExpiredCert(ctx, gateway)
			return trace.Wrap(err)
		}
	}

	ok = true
	return gateway, nil
}

// NewWithLocalPort initializes a copy of an existing gateway which has all config fields identical
// to the existing gateway with the exception of the local port.
func NewWithLocalPort(gateway *Gateway, port string) (*Gateway, error) {
	if port == gateway.LocalPort() {
		return nil, trace.BadParameter("port is already set to %s", port)
	}

	cfg := *gateway.cfg
	cfg.LocalPort = port

	newGateway, err := New(cfg)
	return newGateway, trace.Wrap(err)
}

// Close terminates gateway connection. Fails if called on an already closed gateway.
func (g *Gateway) Close() error {
	g.closeCancel()

	if err := g.localProxy.Close(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Serve starts the underlying ALPN proxy. Blocks until closeContext is canceled.
func (g *Gateway) Serve() error {
	g.cfg.Log.Info("Gateway is open.")

	if err := g.localProxy.Start(g.closeContext); err != nil {
		return trace.Wrap(err)
	}

	g.cfg.Log.Info("Gateway has closed.")

	return nil
}

func (g *Gateway) URI() uri.ResourceURI {
	return g.cfg.URI
}

func (g *Gateway) SetURI(newURI uri.ResourceURI) {
	g.cfg.URI = newURI
}

func (g *Gateway) TargetURI() string {
	return g.cfg.TargetURI
}

func (g *Gateway) TargetName() string {
	return g.cfg.TargetName
}

func (g *Gateway) Protocol() string {
	return g.cfg.Protocol
}

func (g *Gateway) TargetUser() string {
	return g.cfg.TargetUser
}

func (g *Gateway) TargetSubresourceName() string {
	return g.cfg.TargetSubresourceName
}

func (g *Gateway) SetTargetSubresourceName(value string) {
	g.cfg.TargetSubresourceName = value
}

func (g *Gateway) Log() *logrus.Entry {
	return g.cfg.Log
}

func (g *Gateway) LocalAddress() string {
	return g.cfg.LocalAddress
}

func (g *Gateway) LocalPort() string {
	return g.cfg.LocalPort
}

// LocalPortInt returns the port of a gateway as an integer rather than a string.
func (g *Gateway) LocalPortInt() int {
	// Ignoring the error here as Teleterm allows the user to pick only integer values for the port,
	// so the string itself will never be a service name that needs actual lookup.
	// For more details, see https://stackoverflow.com/questions/47992477/why-is-port-a-string-and-not-an-integer
	port, _ := strconv.Atoi(g.cfg.LocalPort)
	return port
}

// CLICommand returns a command which launches a CLI client pointed at the gateway.
func (g *Gateway) CLICommand() (*api.GatewayCLICommand, error) {
	cmd, err := g.cfg.CLICommandProvider.GetCommand(g)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cmdString := strings.TrimSpace(
		fmt.Sprintf("%s %s",
			strings.Join(cmd.Env, " "),
			strings.Join(cmd.Args, " ")))

	return &api.GatewayCLICommand{
		Path:    cmd.Path,
		Args:    cmd.Args,
		Env:     cmd.Env,
		Preview: cmdString,
	}, nil
}

// RouteToDatabase returns tlsca.RouteToDatabase based on the config of the gateway.
//
// The tlsca.RouteToDatabase.Database field is skipped, as it's an optional field and gateways can
// change their Config.TargetSubresourceName at any moment.
func (g *Gateway) RouteToDatabase() tlsca.RouteToDatabase {
	return g.cfg.RouteToDatabase()
}

// ReloadCert loads the key pair from cfg.CertPath & cfg.KeyPath and updates the cert of the running
// local proxy. This is typically done after the cert is reissued and saved to disk.
//
// In the future, we're probably going to make this method accept the cert as an arg rather than
// reading from disk.
func (g *Gateway) ReloadCert() error {
	g.cfg.Log.Debug("Reloading cert")

	tlsCert, err := keys.LoadX509KeyPair(g.cfg.CertPath, g.cfg.KeyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := checkCertSubject(tlsCert, g.RouteToDatabase()); err != nil {
		return trace.Wrap(err,
			"database certificate check failed, try restarting the database connection")
	}

	g.localProxy.SetCerts([]tls.Certificate{tlsCert})

	return nil
}

// checkCertSubject checks if the cert subject matches the expected db route.
//
// Database certs are scoped per database server but not per database user or database name.
// It might happen that after we save the cert but before we load it, another process obtains a
// cert for another db user.
//
// Before using the cert for the proxy, we have to perform this check.
func checkCertSubject(tlsCert tls.Certificate, dbRoute tlsca.RouteToDatabase) error {
	cert, err := utils.TLSCertLeaf(tlsCert)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(alpn.CheckCertSubject(cert, dbRoute))
}

// Gateway describes local proxy that creates a gateway to the remote Teleport resource.
//
// Gateway is not safe for concurrent use in itself. However, all access to gateways is gated by
// daemon.Service which obtains a lock for any operation pertaining to gateways.
//
// In the future if Gateway becomes more complex it might be worthwhile to add an RWMutex to it.
type Gateway struct {
	cfg        *Config
	localProxy *alpn.LocalProxy
	// closeContext and closeCancel are used to signal to any waiting goroutines
	// that the local proxy is now closed and to release any resources.
	closeContext context.Context
	closeCancel  context.CancelFunc
}

// CLICommandProvider provides a CLI command for gateways which support CLI clients.
type CLICommandProvider interface {
	GetCommand(gateway *Gateway) (*exec.Cmd, error)
}

type TCPPortAllocator interface {
	Listen(localAddress, port string) (net.Listener, error)
}

type NetTCPPortAllocator struct{}

func (n NetTCPPortAllocator) Listen(localAddress, port string) (net.Listener, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", localAddress, port))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return listener, nil
}
