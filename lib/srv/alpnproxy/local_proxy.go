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

package alpnproxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/aws"
)

// LocalProxy allows upgrading incoming connection to TLS where custom TLS values are set SNI ALPN and
// updated connection is forwarded to remote ALPN SNI teleport proxy service.
type LocalProxy struct {
	cfg     LocalProxyConfig
	context context.Context
	cancel  context.CancelFunc
	certsMu sync.RWMutex
}

// LocalProxyConfig is configuration for LocalProxy.
type LocalProxyConfig struct {
	// RemoteProxyAddr is the downstream destination address of remote ALPN proxy service.
	RemoteProxyAddr string
	// Protocol set for the upstream TLS connection.
	Protocols []common.Protocol
	// InsecureSkipTLSVerify turns off verification for x509 upstream ALPN proxy service certificate.
	InsecureSkipVerify bool
	// Listener is listener running on local machine.
	Listener net.Listener
	// SNI is a ServerName value set for upstream TLS connection.
	SNI string
	// ParentContext is a parent context, used to signal global closure>
	ParentContext context.Context
	// SSHUser is an SSH username.
	SSHUser string
	// SSHUserHost is user host requested by ssh subsystem.
	SSHUserHost string
	// SSHHostKeyCallback is the function type used for verifying server keys.
	SSHHostKeyCallback ssh.HostKeyCallback
	// SSHTrustedCluster allows selecting trusted cluster ssh subsystem request.
	SSHTrustedCluster string
	// Certs are the client certificates used to connect to the remote Teleport Proxy.
	Certs []tls.Certificate
	// AWSCredentials are AWS Credentials used by LocalProxy for request's signature verification.
	AWSCredentials *credentials.Credentials
	// RootCAs overwrites the root CAs used in tls.Config if specified.
	RootCAs *x509.CertPool
	// ALPNConnUpgradeRequired specifies if ALPN connection upgrade is required.
	ALPNConnUpgradeRequired bool
	// Middleware provides callback functions to the local proxy.
	Middleware LocalProxyMiddleware
	// Clock is used to override time in tests.
	Clock clockwork.Clock
	// Log is the Logger.
	Log logrus.FieldLogger
}

// LocalProxyMiddleware provides callback functions for LocalProxy.
type LocalProxyMiddleware interface {
	// OnNewConnection is a callback triggered when a new downstream connection is
	// accepted by the local proxy. If an error is returned, the connection will be closed
	// by the local proxy.
	OnNewConnection(ctx context.Context, lp *LocalProxy, conn net.Conn) error
	// OnStart is a callback triggered when the local proxy starts.
	OnStart(ctx context.Context, lp *LocalProxy) error
}

// CheckAndSetDefaults verifies the constraints for LocalProxyConfig.
func (cfg *LocalProxyConfig) CheckAndSetDefaults() error {
	if cfg.RemoteProxyAddr == "" {
		return trace.BadParameter("missing remote proxy address")
	}
	if len(cfg.Protocols) == 0 {
		return trace.BadParameter("missing protocol")
	}
	if cfg.ParentContext == nil {
		return trace.BadParameter("missing parent context")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Log == nil {
		cfg.Log = logrus.WithField(trace.Component, "localproxy")
	}
	return nil
}

func (cfg *LocalProxyConfig) GetProtocols() []string {
	protos := make([]string, 0, len(cfg.Protocols))

	for _, proto := range cfg.Protocols {
		protos = append(protos, string(proto))
	}

	return protos
}

// NewLocalProxy creates a new instance of LocalProxy.
func NewLocalProxy(cfg LocalProxyConfig) (*LocalProxy, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(cfg.ParentContext)
	return &LocalProxy{
		cfg:     cfg,
		context: ctx,
		cancel:  cancel,
	}, nil
}

// Start starts the LocalProxy.
func (l *LocalProxy) Start(ctx context.Context) error {
	if l.cfg.Middleware != nil {
		err := l.cfg.Middleware.OnStart(ctx, l)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn, err := l.cfg.Listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) {
				return nil
			}
			l.cfg.Log.WithError(err).Error("Failed to accept client connection.")
			return trace.Wrap(err)
		}

		if l.cfg.Middleware != nil {
			if err := l.cfg.Middleware.OnNewConnection(ctx, l, conn); err != nil {
				l.cfg.Log.WithError(err).Error("Middleware failed to handle client connection.")
				if err := conn.Close(); err != nil && !utils.IsUseOfClosedNetworkError(err) {
					l.cfg.Log.WithError(err).Debug("Failed to close client connection.")
				}
				continue
			}
		}

		go func() {
			if err := l.handleDownstreamConnection(ctx, conn); err != nil {
				if utils.IsOKNetworkError(err) {
					return
				}
				l.cfg.Log.WithError(err).Error("Failed to handle connection.")
			}
		}()
	}
}

// GetAddr returns the LocalProxy listener address.
func (l *LocalProxy) GetAddr() string {
	return l.cfg.Listener.Addr().String()
}

// handleDownstreamConnection proxies the downstreamConn (connection established to the local proxy) and forward the
// traffic to the upstreamConn (TLS connection to remote host).
func (l *LocalProxy) handleDownstreamConnection(ctx context.Context, downstreamConn net.Conn) error {
	defer downstreamConn.Close()

	tlsConn, err := DialALPN(ctx, l.cfg.RemoteProxyAddr, ALPNDialerConfig{
		ALPNConnUpgradeRequired: l.cfg.ALPNConnUpgradeRequired,
		TLSConfig: &tls.Config{
			NextProtos:         l.cfg.GetProtocols(),
			InsecureSkipVerify: l.cfg.InsecureSkipVerify,
			ServerName:         l.cfg.SNI,
			Certificates:       l.getCerts(),
			RootCAs:            l.cfg.RootCAs,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer tlsConn.Close()

	var upstreamConn net.Conn = tlsConn
	if common.IsPingProtocol(common.Protocol(tlsConn.ConnectionState().NegotiatedProtocol)) {
		l.cfg.Log.Debug("Using ping connection")
		upstreamConn = NewPingConn(tlsConn)
	}

	return trace.Wrap(utils.ProxyConn(ctx, downstreamConn, upstreamConn))
}

func (l *LocalProxy) Close() error {
	l.cancel()
	if l.cfg.Listener != nil {
		if err := l.cfg.Listener.Close(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// StartAWSAccessProxy starts the local AWS CLI proxy.
func (l *LocalProxy) StartAWSAccessProxy(ctx context.Context) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			NextProtos:         l.cfg.GetProtocols(),
			InsecureSkipVerify: l.cfg.InsecureSkipVerify,
			ServerName:         l.cfg.SNI,
			Certificates:       l.getCerts(),
		},
	}
	proxy := &httputil.ReverseProxy{
		Director: func(outReq *http.Request) {
			outReq.URL.Scheme = "https"
			outReq.URL.Host = l.cfg.RemoteProxyAddr
		},
		Transport: tr,
	}
	err := http.Serve(l.cfg.Listener, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := aws.VerifyAWSSignature(req, l.cfg.AWSCredentials); err != nil {
			l.cfg.Log.WithError(err).Error("AWS signature verification failed.")
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		// Requests from forward proxy have original hostnames instead of
		// localhost. Set appropriate header to keep this information.
		if addr, err := utils.ParseAddr(req.Host); err == nil && !addr.IsLocal() {
			req.Header.Set("X-Forwarded-Host", req.Host)
		}

		proxy.ServeHTTP(rw, req)
	}))
	if err != nil && !utils.IsUseOfClosedNetworkError(err) {
		return trace.Wrap(err)
	}
	return nil
}

// getCerts returns the local proxy's configured TLS certificates.
// For thread-safety, it is important that the returned slice and its contents are not be mutated by callers,
// therefore this method is not exported.
func (l *LocalProxy) getCerts() []tls.Certificate {
	l.certsMu.RLock()
	defer l.certsMu.RUnlock()
	return l.cfg.Certs
}

// CheckDBCerts checks the proxy certificates for expiration and that the cert subject matches a database route.
func (l *LocalProxy) CheckDBCerts(dbRoute tlsca.RouteToDatabase) error {
	l.cfg.Log.Debug("checking local proxy database certs")
	l.certsMu.RLock()
	defer l.certsMu.RUnlock()
	if len(l.cfg.Certs) == 0 {
		return trace.NotFound("local proxy has no TLS certificates configured")
	}
	cert, err := utils.TLSCertToX509(l.cfg.Certs[0])
	if err != nil {
		return trace.Wrap(err)
	}

	// Check for cert expiration.
	if err := utils.VerifyCertificateExpiry(cert, l.cfg.Clock); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(CheckCertSubject(cert, dbRoute))
}

// CheckCertSubject checks if the route to the database from the cert matches the provided route in
// terms of username and database (if present).
func CheckCertSubject(cert *x509.Certificate, dbRoute tlsca.RouteToDatabase) error {
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return trace.Wrap(err)
	}
	if dbRoute.Username != "" && dbRoute.Username != identity.RouteToDatabase.Username {
		return trace.Errorf("certificate subject is for user %s, but need %s",
			identity.RouteToDatabase.Username, dbRoute.Username)
	}
	if dbRoute.Database != "" && dbRoute.Database != identity.RouteToDatabase.Database {
		return trace.Errorf("certificate subject is for database name %s, but need %s",
			identity.RouteToDatabase.Database, dbRoute.Database)
	}

	return nil
}

// SetCerts sets the local proxy's configured TLS certificates.
func (l *LocalProxy) SetCerts(certs []tls.Certificate) {
	l.certsMu.Lock()
	defer l.certsMu.Unlock()
	l.cfg.Certs = certs
}
