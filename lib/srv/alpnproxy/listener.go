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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"net"
	"sync"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ListenerMuxWrapper wraps the net.Listener and multiplex incoming connection from serviceListener and connection
// injected by HandleConnection handler.
type ListenerMuxWrapper struct {
	// net.Listener is the main service listener that is being wrapped.
	net.Listener
	// alpnListener is the ALPN service listener.
	alpnListener net.Listener
	connC        chan net.Conn
	errC         chan error
	close        chan struct{}
}

// NewMuxListenerWrapper creates a new instance of ListenerMuxWrapper
func NewMuxListenerWrapper(serviceListener, alpnListener net.Listener) *ListenerMuxWrapper {
	listener := &ListenerMuxWrapper{
		alpnListener: alpnListener,
		Listener:     serviceListener,
		connC:        make(chan net.Conn),
		errC:         make(chan error),
		close:        make(chan struct{}),
	}
	go listener.startAcceptingConnectionServiceListener()
	return listener
}

// HandleConnection allows injecting connection to the listener.
func (l *ListenerMuxWrapper) HandleConnection(ctx context.Context, conn net.Conn) error {
	select {
	case <-l.close:
		return trace.ConnectionProblem(nil, "listener is closed")
	case <-ctx.Done():
		return ctx.Err()
	case l.connC <- conn:
		return nil
	}
}

// Addr returns address of the listeners. If both serviceListener and alpnListener listeners were provided.
// function will return address obtained from the alpnListener listener.
func (l *ListenerMuxWrapper) Addr() net.Addr {
	if l.alpnListener != nil {
		return l.alpnListener.Addr()
	}
	return l.Listener.Addr()
}

// Accept waits for the next injected by HandleConnection or received from serviceListener and returns it.
func (l *ListenerMuxWrapper) Accept() (net.Conn, error) {
	select {
	case <-l.close:
		return nil, trace.ConnectionProblem(nil, "listener is closed")
	case err := <-l.errC:
		return nil, trace.Wrap(err)
	case conn := <-l.connC:
		return conn, nil
	}
}

func (l *ListenerMuxWrapper) startAcceptingConnectionServiceListener() {
	if l.Listener == nil {
		return
	}
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			l.errC <- err
			return
		}
		select {
		case l.connC <- conn:
		case <-l.close:
			return

		}
	}
}

// Close the ListenerMuxWrapper.
func (l *ListenerMuxWrapper) Close() error {
	var errs []error
	if l.Listener != nil {
		if err := l.Listener.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if l.alpnListener != nil {
		if err := l.alpnListener.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	// Close channel only once.
	select {
	case <-l.close:
	default:
		close(l.close)
	}
	return trace.NewAggregate(errs...)
}

// CertGenListenerConfig is the config for CertGenListener.
type CertGenListenerConfig struct {
	// ListenAddr is network address to listen.
	ListenAddr string
	// CA is the certificate authority for signing certificates.
	CA tls.Certificate
}

// CheckAndSetDefaults checks and sets default config values.
func (c *CertGenListenerConfig) CheckAndSetDefaults() error {
	if c.ListenAddr == "" {
		return trace.BadParameter("missing listener address")
	}
	if len(c.CA.Certificate) == 0 {
		return trace.BadParameter("missing CA certificate")
	}
	return nil
}

// CertGenListener is a HTTPS listener that generates TLS certificates based on
// SNI during HTTPS handshake.
type CertGenListener struct {
	net.Listener

	certAuthority      *tlsca.CertAuthority
	cfg                CertGenListenerConfig
	mu                 sync.RWMutex
	certificatesByHost map[string]*tls.Certificate
}

// NewCertGenListener creates a new CertGenListener and listens to the
// configured listen address.
func NewCertGenListener(config CertGenListenerConfig) (*CertGenListener, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	certAuthority, err := tlsca.FromTLSCertificate(config.CA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r := &CertGenListener{
		cfg:                config,
		certificatesByHost: make(map[string]*tls.Certificate),
		certAuthority:      certAuthority,
	}

	// Use CA for hostnames in the CA.
	for _, host := range r.certAuthority.Cert.DNSNames {
		r.certificatesByHost[host] = &config.CA
	}

	r.Listener, err = tls.Listen("tcp", r.cfg.ListenAddr, &tls.Config{
		GetCertificate: r.GetCertificate,
	})
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	return r, nil
}

// GetCertificate return TLS certificate based on SNI. Implements
// tls.Config.GetCertificate.
func (r *CertGenListener) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	// Requests to IPs have no server names. Default to CA.
	if clientHello.ServerName == "" {
		return &r.cfg.CA, nil
	}

	r.mu.RLock()
	if cert, found := r.certificatesByHost[clientHello.ServerName]; found {
		r.mu.RUnlock()
		return cert, nil
	}
	r.mu.RUnlock()

	cert, err := r.generateCertFor(clientHello.ServerName)
	if err != nil {
		log.WithError(err).Errorf("Failed to generate certificate for %q.", clientHello.ServerName)

		// Default to CA.
		return &r.cfg.CA, nil
	}

	return cert, err
}

// generateCertFor generates a new certificate for the specified host.
func (r *CertGenListener) generateCertFor(host string) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cert, found := r.certificatesByHost[host]; found {
		return cert, nil
	}

	certKey, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	subject := r.certAuthority.Cert.Subject
	subject.CommonName = host

	certPem, err := r.certAuthority.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: &certKey.PublicKey,
		Subject:   subject,
		NotAfter:  r.certAuthority.Cert.NotAfter,
		DNSNames:  []string{host},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := tls.X509KeyPair(certPem, tlsca.MarshalPrivateKeyPEM(certKey))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r.certificatesByHost[host] = &cert
	return &cert, nil
}
