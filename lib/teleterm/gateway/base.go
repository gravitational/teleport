/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package gateway

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/utils/keys"
	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// New creates an instance of Gateway. It starts a listener on the specified port but it doesn't
// start the proxy – that's the job of Serve.
func New(cfg Config) (Gateway, error) {
	switch {
	case cfg.TargetURI.IsDB():
		gateway, err := makeDatabaseGateway(cfg)
		return gateway, trace.Wrap(err)

	case cfg.TargetURI.IsKube():
		gateway, err := makeKubeGateway(cfg)
		return gateway, trace.Wrap(err)

	case cfg.TargetURI.IsApp():
		gateway, err := makeAppGateway(cfg)
		return gateway, trace.Wrap(err)

	default:
		return nil, trace.NotImplemented("gateway not supported for %v", cfg.TargetURI)
	}
}

// NewWithLocalPort initializes a copy of an existing gateway which has all config fields identical
// to the existing gateway with the exception of the local port.
func NewWithLocalPort(gateway Gateway, port string) (Gateway, error) {
	if port == gateway.LocalPort() {
		return nil, trace.BadParameter("port is already set to %s", port)
	}

	type configCloner interface {
		cloneConfig() Config
	}

	cloner, ok := gateway.(configCloner)
	if !ok {
		return nil, trace.BadParameter("failed to convert gateway to configCloner")
	}

	cfg := cloner.cloneConfig()
	cfg.LocalPort = port

	newGateway, err := New(cfg)
	return newGateway, trace.Wrap(err)
}

func newBase(cfg Config) (*base, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	closeContext, closeCancel := context.WithCancel(context.Background())
	return &base{
		cfg:          &cfg,
		closeContext: closeContext,
		closeCancel:  closeCancel,
	}, nil
}

// Close terminates gateway connection. Fails if called on an already closed gateway.
func (b *base) Close() error {
	b.closeCancel()

	var errs []error
	if b.localProxy != nil {
		errs = append(errs, b.localProxy.Close())
	}
	if b.forwardProxy != nil {
		errs = append(errs, b.forwardProxy.Close())
	}

	for _, cleanup := range b.onCloseFuncs {
		errs = append(errs, cleanup())
	}
	return trace.NewAggregate(errs...)
}

// Serve starts the underlying ALPN proxy. Blocks until closeContext is canceled.
func (b *base) Serve() error {
	b.cfg.Log.Info("Gateway is open.")
	defer b.cfg.Log.Info("Gateway has closed.")

	if b.forwardProxy != nil {
		return trace.Wrap(b.serveWithForwardProxy())
	}
	return trace.Wrap(b.localProxy.Start(b.closeContext))
}

func (b *base) serveWithForwardProxy() error {
	errChan := make(chan error, 2)
	go func() {
		if err := b.forwardProxy.Start(); err != nil {
			errChan <- err
		}
	}()
	go func() {
		if err := b.localProxy.Start(b.closeContext); err != nil {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return trace.NewAggregate(err, b.Close())
	case <-b.closeContext.Done():
		return nil
	}
}

func (b *base) URI() uri.ResourceURI {
	return b.cfg.URI
}

func (b *base) TargetURI() uri.ResourceURI {
	return b.cfg.TargetURI
}

func (b *base) TargetName() string {
	return b.cfg.TargetName
}

func (b *base) Protocol() string {
	return b.cfg.Protocol
}

func (b *base) TargetUser() string {
	return b.cfg.TargetUser
}

func (b *base) TargetSubresourceName() string {
	return b.cfg.TargetSubresourceName
}

func (b *base) SetTargetSubresourceName(value string) {
	b.cfg.TargetSubresourceName = value
}

func (b *base) Log() *logrus.Entry {
	return b.cfg.Log
}

func (b *base) LocalAddress() string {
	return b.cfg.LocalAddress
}

func (b *base) LocalPort() string {
	return b.cfg.LocalPort
}

// LocalPortInt returns the port of a gateway as an integer rather than a string.
func (b *base) LocalPortInt() int {
	// Ignoring the error here as Teleterm allows the user to pick only integer values for the port,
	// so the string itself will never be a service name that needs actual lookup.
	// For more details, see https://stackoverflow.com/questions/47992477/why-is-port-a-string-and-not-an-integer
	port, _ := strconv.Atoi(b.cfg.LocalPort)
	return port
}

// ReloadCert loads the key pair from cfg.CertPath & cfg.KeyPath and updates the cert of the running
// local proxy. This is typically done after the cert is reissued and saved to disk.
//
// In the future, we're probably going to make this method accept the cert as an arg rather than
// reading from disk.
// TODO(ravicious): Remove ReloadCert after adding MFA support to gateways.
func (b *base) ReloadCert() error {
	if len(b.onNewCertFuncs) == 0 {
		return nil
	}
	b.cfg.Log.Debug("Reloading cert")

	if b.cfg.CertPath == "" {
		return trace.Errorf("attempted to reload cert for a gateway, but cert path is empty")
	}

	if b.cfg.KeyPath == "" {
		return trace.Errorf("attempted to reload cert for a gateway, but key path is empty")
	}

	tlsCert, err := keys.LoadX509KeyPair(b.cfg.CertPath, b.cfg.KeyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	var errs []error
	for _, onNewCert := range b.onNewCertFuncs {
		errs = append(errs, onNewCert(tlsCert))
	}

	return trace.NewAggregate(errs...)
}

func (b *base) cloneConfig() Config {
	return *b.cfg
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
type base struct {
	cfg          *Config
	localProxy   *alpn.LocalProxy
	forwardProxy *alpn.ForwardProxy
	// onNewCertFuncs contains a list of callback functions that update the local
	// proxy when TLS certificate is reissued.
	// TODO(ravicious): Remove this field after adding MFA support to gateways.
	onNewCertFuncs []func(tls.Certificate) error
	// onCloseFuncs contains a list of extra cleanup functions called during Close.
	onCloseFuncs []func() error
	// closeContext and closeCancel are used to signal to any waiting goroutines
	// that the local proxy is now closed and to release any resources.
	closeContext context.Context
	closeCancel  context.CancelFunc
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
