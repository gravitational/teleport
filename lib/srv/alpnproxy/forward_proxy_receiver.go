/*
Copyright 2022 Gravitational, Inc.

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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"net"
	"net/http"
	"sync"

	"github.com/gravitational/teleport/api/constants"
	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// ForwardProxyReceiver defines an interface for a forward proxy receiver.
type ForwardProxyReceiver interface {
	// Want returns whether the request should be forwarded to this receiver.
	Want(req *http.Request) (wanted bool)

	// GetAddr returns the listener's network address.
	GetAddr() string
}

// HTTPSListenerReceiverConfig is the config for a HTTPS listener receiver.
type HTTPSListenerReceiverConfig struct {
	// ListenAddr is network address to listen.
	ListenAddr string
	// CA is the CA certificate for signing certificate.
	CA tls.Certificate
	// Want returns whether the request should be forwarded to this receiver.
	Want func(req *http.Request) (wanted bool)
	// Log is the logger.
	Log logrus.FieldLogger
}

// Check validates the config.
func (c *HTTPSListenerReceiverConfig) Check() error {
	if c.ListenAddr == "" {
		return trace.BadParameter("missing listener address")
	}
	if len(c.CA.Certificate) == 0 {
		return trace.BadParameter("missing CA certificate")
	}
	if c.Want == nil {
		return trace.BadParameter("missing Want function")
	}
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, "fwdproxy")
	}
	return nil
}

// HTTPSListenerReceiver is a HTTPS listener that receives from forward proxy.
//
// As a forward proxy receiver, HTTPSListenerReceiver is first asked by the
// forward proxy to receive a certain HTTPS request. If HTTPSListenerReceiver
// wants the request, it generates a certificate for the requested host, and
// signs it with the configured CA. Then when the forward proxy sends the
// request, HTTPSListenerReceiver serves the generated certficate based on SNI
// during HTTPS handshake.
type HTTPSListenerReceiver struct {
	net.Listener

	cfg                HTTPSListenerReceiverConfig
	mu                 sync.RWMutex
	certificatesByHost map[string]*tls.Certificate
}

// NewHTTPSListenerReceiver creates a new HTTPSListenerReceiver and listens to
// the configured listen address.
func NewHTTPSListenerReceiver(config HTTPSListenerReceiverConfig) (*HTTPSListenerReceiver, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	r := &HTTPSListenerReceiver{
		cfg:                config,
		certificatesByHost: make(map[string]*tls.Certificate),
	}

	var err error
	tlsConfig := &tls.Config{
		GetCertificate: r.GetCertificate,
	}
	if r.Listener, err = tls.Listen("tcp", r.cfg.ListenAddr, tlsConfig); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return r, nil
}

// NewHTTPSListenerReceiverForAWS creates a new HTTPSListenerReceiver for AWS APIs.
func NewHTTPSListenerReceiverForAWS(config HTTPSListenerReceiverConfig) (*HTTPSListenerReceiver, error) {
	config.Want = func(req *http.Request) (wanted bool) {
		return awsapiutils.IsAWSEndpoint(req.Host)
	}
	return NewHTTPSListenerReceiver(config)
}

// GetListener returns the HTTPS listener.
func (r *HTTPSListenerReceiver) GetListener() net.Listener {
	return r.Listener
}

// GetAddr returns the listener's network address. Implements
// ForwardProxyReceiver.
func (r *HTTPSListenerReceiver) GetAddr() string {
	return r.Listener.Addr().String()
}

// Want returns whether the request should be forwarded to this receiver.
// Implements ForwardProxyReceiver.
func (r *HTTPSListenerReceiver) Want(req *http.Request) (wanted bool) {
	if !r.cfg.Want(req) {
		return false
	}

	if err := r.generateCertFor(req.Host); err != nil {
		r.cfg.Log.WithError(err).Debugf("Failed to generate certificate for %q.", req.Host)
		return false
	}
	return true
}

// GetCertificate return TLS certificate based on SNI. Implements
// tls.Config.GetCertificate.
func (r *HTTPSListenerReceiver) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if cert, found := r.certificatesByHost[clientHello.ServerName]; found {
		return cert, nil
	}

	return &r.cfg.CA, nil
}

// generateCertFor generates a new certificate for the specified host.
func (r *HTTPSListenerReceiver) generateCertFor(host string) error {
	// Remove port.
	addr, err := utils.ParseAddr(host)
	if err != nil {
		return trace.Wrap(err)
	}
	host = addr.Host()

	r.mu.RLock()
	if _, found := r.certificatesByHost[host]; found {
		r.mu.RUnlock()
		return nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, found := r.certificatesByHost[host]; found {
		return nil
	}

	certKey, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
	if err != nil {
		return trace.Wrap(err)
	}

	ca, err := tlsca.FromTLSCertificate(r.cfg.CA)
	if err != nil {
		return trace.Wrap(err)
	}

	subject := ca.Cert.Subject
	subject.CommonName = host

	certPem, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: &certKey.PublicKey,
		Subject:   subject,
		NotAfter:  ca.Cert.NotAfter,
		DNSNames:  []string{host},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	cert, err := tls.X509KeyPair(certPem, tlsca.MarshalPrivateKeyPEM(certKey))
	if err != nil {
		return trace.Wrap(err)
	}

	r.cfg.Log.Debugf("Certificate generated for %q", host)
	r.certificatesByHost[host] = &cert
	return nil
}
