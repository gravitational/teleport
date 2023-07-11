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

	gateway := &Gateway{
		cfg:          &cfg,
		closeContext: closeContext,
		closeCancel:  closeCancel,
	}

	switch targetURI := uri.New(cfg.TargetURI); {
	case targetURI.IsDB():
		if err := gateway.makeLocalProxyForDB(listener); err != nil {
			return nil, trace.Wrap(err)
		}

	case targetURI.IsKube():
		if err := gateway.makeLocalProxiesForKube(listener); err != nil {
			return nil, trace.Wrap(err)
		}

	default:
		return nil, trace.NotImplemented("gateway not supported for %v", cfg.TargetURI)
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

	var errs []error
	if g.localProxy != nil {
		errs = append(errs, g.localProxy.Close())
	}
	if g.forwardProxy != nil {
		errs = append(errs, g.forwardProxy.Close())
	}

	for _, cleanup := range g.onCloseFuncs {
		errs = append(errs, cleanup())
	}
	return trace.NewAggregate(errs...)
}

// Serve starts the underlying ALPN proxy. Blocks until closeContext is canceled.
func (g *Gateway) Serve() error {
	g.cfg.Log.Info("Gateway is open.")
	defer g.cfg.Log.Info("Gateway has closed.")

	if g.forwardProxy != nil {
		return trace.Wrap(g.serveWithForwardProxy())
	}
	return trace.Wrap(g.localProxy.Start(g.closeContext))
}

func (g *Gateway) serveWithForwardProxy() error {
	errChan := make(chan error, 2)
	go func() {
		if err := g.forwardProxy.Start(); err != nil {
			errChan <- err
		}
	}()
	go func() {
		if err := g.localProxy.Start(g.closeContext); err != nil {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return trace.NewAggregate(err, g.Close())
	case <-g.closeContext.Done():
		return nil
	}
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

// ReloadCert loads the key pair from cfg.CertPath & cfg.KeyPath and updates the cert of the running
// local proxy. This is typically done after the cert is reissued and saved to disk.
//
// In the future, we're probably going to make this method accept the cert as an arg rather than
// reading from disk.
func (g *Gateway) ReloadCert() error {
	if len(g.onNewCertFuncs) == 0 {
		return nil
	}
	g.cfg.Log.Debug("Reloading cert")

	tlsCert, err := keys.LoadX509KeyPair(g.cfg.CertPath, g.cfg.KeyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	var errs []error
	for _, onNewCert := range g.onNewCertFuncs {
		errs = append(errs, onNewCert(tlsCert))
	}

	return trace.NewAggregate(errs...)
}

func (g *Gateway) onExpiredCert(ctx context.Context) error {
	if g.cfg.OnExpiredCert == nil {
		return nil
	}
	return trace.Wrap(g.cfg.OnExpiredCert(ctx, g))
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
	cfg          *Config
	localProxy   *alpn.LocalProxy
	forwardProxy *alpn.ForwardProxy
	// onNewCertFuncs contains a list of callback functions that update the local
	// proxy when TLS certificate is reissued.
	onNewCertFuncs []func(tls.Certificate) error
	// onCloseFuncs contains a list of extra cleanup functions called during Close.
	onCloseFuncs []func() error
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
