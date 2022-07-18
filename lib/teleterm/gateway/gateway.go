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
	"strconv"

	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// New creates an instance of Gateway. It starts a listener on the specified port but it doesn't
// start the proxy – that's the job of Serve.
func New(cfg Config) (*Gateway, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := newListenerAndLocalProxy(cfg, cfg.LocalPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.LocalPort = result.LocalPort

	gateway := &Gateway{
		cfg:          &cfg,
		closeContext: result.closeContext,
		closeCancel:  result.closeCancel,
		localProxy:   result.localProxy,
	}

	return gateway, nil
}

type newListenerAndLocalProxyResult struct {
	LocalPort    string
	closeContext context.Context
	closeCancel  context.CancelFunc
	localProxy   *alpn.LocalProxy
}

func newListenerAndLocalProxy(cfg Config, port string) (*newListenerAndLocalProxyResult, error) {
	listener, err := cfg.TCPPortAllocator.Listen(cfg.LocalAddress, port)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, closeCancel := context.WithCancel(context.Background())
	// make sure the listener is closed if proxy creation failed
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
	_, localPort, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	localProxy, err := newLocalProxy(closeContext, cfg, listener)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ok = true
	return &newListenerAndLocalProxyResult{
			LocalPort:    localPort,
			closeContext: closeContext,
			closeCancel:  closeCancel,
			localProxy:   localProxy},
		nil
}

func newLocalProxy(closeContext context.Context, cfg Config, listener net.Listener) (*alpn.LocalProxy, error) {
	protocol, err := alpncommon.ToALPNProtocol(cfg.Protocol)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	address, err := utils.ParseAddr(cfg.WebProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	localProxy, err := alpn.NewLocalProxy(alpn.LocalProxyConfig{
		InsecureSkipVerify: cfg.Insecure,
		RemoteProxyAddr:    cfg.WebProxyAddr,
		Protocols:          []alpncommon.Protocol{protocol},
		Listener:           listener,
		ParentContext:      closeContext,
		SNI:                address.Host(),
		Certs:              []tls.Certificate{cert},
	})

	return localProxy, trace.Wrap(err)
}

// Close terminates gateway connection
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

// SetLocalPortAndRestart attempts to create a listener on the specified port. If successful, it
// obtains a listener on the new port, stops the old proxy, updates the fields on gateway and then
// starts the new proxy with the new listener. It starts the local proxy even if the previous one
// wasn't started – SetLocalPortAndRestart is expected to be called after Serve.
//
// If it fails it's imperative that the current proxy is kept intact and the fields on gateway are
// not changed. This way if the user attempts to change the port to one that cannot be obtained,
// they're able to correct that mistake and choose a different port.
//
// SetLocalPortAndRestart is a noop if port is equal to the existing port.
func (g *Gateway) SetLocalPortAndRestart(port string) error {
	if port == g.cfg.LocalPort {
		return nil
	}

	result, err := newListenerAndLocalProxy(*g.cfg, port)
	if err != nil {
		return trace.Wrap(err)
	}

	previousPort := g.cfg.LocalPort

	g.closeCancel()
	if err = g.localProxy.Close(); err != nil {
		g.cfg.Log.WithError(err).Warnf("Failed to close the previous proxy on port %s.", previousPort)
	}

	g.cfg.LocalPort = result.LocalPort
	g.localProxy = result.localProxy
	g.closeCancel = result.closeCancel
	g.closeContext = result.closeContext

	go func() {
		if err := g.Serve(); err != nil {
			g.cfg.Log.WithError(err).Warn("Failed to restart the gateway after changing the port.")
		}
	}()

	return nil
}

// LocalPortInt returns the port of a gateway as an integer rather than a string.
func (g *Gateway) LocalPortInt() int {
	// Ignoring the error here as Teleterm allows the user to pick only integer values for the port,
	// so the string itself will never be a service name that needs actual lookup.
	// For more details, see https://stackoverflow.com/questions/47992477/why-is-port-a-string-and-not-an-integer
	port, _ := strconv.Atoi(g.cfg.LocalPort)
	return port
}

// CLICommand returns a command which launches a CLI client pointed at the given gateway.
func (g *Gateway) CLICommand() (string, error) {
	cliCommand, err := g.cfg.CLICommandProvider.GetCommand(g)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return cliCommand, nil
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
	GetCommand(gateway *Gateway) (string, error)
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
