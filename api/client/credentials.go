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

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"

	"github.com/gravitational/teleport/api/constants"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http2"
)

// Credentials are used to authenticate the API auth client. Some Credentials
// also provide other functionality, such as automatic address discovery and ssh
// connectivity.
// see the examples below (godoc)
type Credentials interface {
	// Dialer is used to create a dialer used to connect to the Auth server.
	Dialer(cfg Config) (ContextDialer, error)
	// TLSConfig returns TLS configuration used to authenticate the client.
	TLSConfig() (*tls.Config, error)
	// SSHClientConfig returns SSH configuration used to connect to Auth through a reverse tunnel.
	SSHClientConfig() (*ssh.ClientConfig, error)
}

// LoadTLS is used to load Credentials directly from a *tls.Config.
func LoadTLS(tlsConfig *tls.Config) *TLSConfigCreds {
	return &TLSConfigCreds{
		tlsConfig: tlsConfig,
	}
}

// TLSConfigCreds use a defined *tls.Config to provide TLS authentication.
// TLSConfigCreds can only be used to connect directly to a local Teleport Auth server.
type TLSConfigCreds struct {
	tlsConfig *tls.Config
}

// Dialer is used to dial a connection to an Auth server.
func (c *TLSConfigCreds) Dialer(cfg Config) (ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig returns TLS configuration.
func (c *TLSConfigCreds) TLSConfig() (*tls.Config, error) {
	if c.tlsConfig == nil {
		return nil, trace.BadParameter("tls config is nil")
	}
	return configure(c.tlsConfig), nil
}

// SSHClientConfig returns SSH configuration.
func (c *TLSConfigCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
	return nil, trace.NotImplemented("no ssh config")
}

// LoadKeyPair is used to load Credentials from a certicate keypair on disk.
// A new keypair can be generated with tctl.
// See the example below (godoc).
func LoadKeyPair(certFile string, keyFile string, caFile string) *KeyPairCreds {
	return &KeyPairCreds{
		certFile: certFile,
		keyFile:  keyFile,
		caFile:   caFile,
	}
}

// KeyPairCreds use a certificate keypair to provide TLS authentication.
// KeyPairCreds can only be used to connect directly to a local Teleport Auth server.
type KeyPairCreds struct {
	certFile string
	keyFile  string
	caFile   string
}

// Dialer is used to dial a connection to an Auth server.
func (c *KeyPairCreds) Dialer(cfg Config) (ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig returns TLS configuration.
func (c *KeyPairCreds) TLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(c.certFile, c.keyFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cas, err := ioutil.ReadFile(c.caFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(cas); !ok {
		return nil, trace.BadParameter("invalid TLS CA cert PEM")
	}

	return configure(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}), nil
}

// SSHClientConfig returns SSH configuration.
func (c *KeyPairCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
	return nil, trace.NotImplemented("no ssh config")
}

// LoadIdentityFile is used to load Credentials from an identity file on disk.
// A new identity file can be generated with tsh or tctl.
// See the example below (godoc).
func LoadIdentityFile(path string) *IdentityCreds {
	return &IdentityCreds{
		path: path,
	}
}

// IdentityCreds use an identity file to provide TLS authentication
// and ssh connectivity. They can be used to connect to an Auth server
// directly (local), or through the cluster's reverse proxy or web proxy.
type IdentityCreds struct {
	path         string
	identityFile *IdentityFile
}

// Dialer is used to dial a connection to an Auth server.
func (c *IdentityCreds) Dialer(cfg Config) (ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig returns TLS configuration.
func (c *IdentityCreds) TLSConfig() (*tls.Config, error) {
	if err := c.load(); err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := c.identityFile.TLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return configure(tlsConfig), nil
}

// SSHClientConfig returns SSH configuration.
func (c *IdentityCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
	if err := c.load(); err != nil {
		return nil, trace.Wrap(err)
	}

	sshConfig, err := c.identityFile.SSHClientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sshConfig, nil
}

// load is used to lazy load the identity file from persistent storage.
// This allows LoadIdentity to avoid possible errors for UX purposes.
func (c *IdentityCreds) load() error {
	if c.identityFile != nil {
		return nil
	}
	var err error
	if c.identityFile, err = ReadIdentityFile(c.path); err != nil {
		return trace.BadParameter("identity file could not be decoded: %v", err)
	}
	return nil
}

// LoadProfile is used to load Credentials from a tsh Profile.
// If dir is not specified, the default profile path will be used.
// If name is not specified, the current profile name will be used.

// LoadProfile is used to load Credentials from a tsh profile on disk.
// A new profile can be generated with tsh.
// See the example below (godoc).
func LoadProfile(dir, name string) *ProfileCreds {
	return &ProfileCreds{
		dir:  dir,
		name: name,
	}
}

// ProfileCreds are used to authenticate the client
// with a tsh profile with the given directory and name.

// ProfileCreds use a tsh profile to provide TLS authentication, ssh
// connectivity, and automatic address discovery. They can be used to
// connect to an Auth server directly (local), or through the cluster's
// reverse proxy or web proxy. The cluster's web proxy address will
// be retrieved from the profile and used to connect over ssh.
type ProfileCreds struct {
	dir     string
	name    string
	profile *Profile
}

// Dialer is used to dial a connection to an Auth server.
func (c *ProfileCreds) Dialer(cfg Config) (ContextDialer, error) {
	sshConfig, err := c.SSHClientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dialer := NewTunnelDialer(*sshConfig, cfg.KeepAlivePeriod, cfg.DialTimeout)
	return ContextDialerFunc(func(ctx context.Context, network, _ string) (conn net.Conn, err error) {
		// Ping web proxy to retrieve tunnel proxy address.
		pr, err := Find(ctx, c.profile.WebProxyAddr, cfg.InsecureAddressDiscovery, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		conn, err = dialer.DialContext(ctx, network, pr.Proxy.SSH.TunnelPublicAddr)
		if err != nil {
			// not wrapping on purpose to preserve the original error
			return nil, err
		}
		return conn, nil
	}), nil
}

// TLSConfig returns TLS configuration.
func (c *ProfileCreds) TLSConfig() (*tls.Config, error) {
	if err := c.load(); err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := c.profile.TLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return configure(tlsConfig), nil
}

// SSHClientConfig returns SSH configuration.
func (c *ProfileCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
	if err := c.load(); err != nil {
		return nil, trace.Wrap(err)
	}

	sshConfig, err := c.profile.SSHClientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sshConfig, nil
}

// load is used to lazy load the profile from persistent storage.
// This allows LoadProfile to avoid possible errors for UX purposes.
func (c *ProfileCreds) load() error {
	if c.profile != nil {
		return nil
	}
	var err error
	if c.profile, err = ProfileFromDir(c.dir, c.name); err != nil {
		return trace.BadParameter("profile could not be decoded: %v", err)
	}
	return nil
}

func configure(c *tls.Config) *tls.Config {
	tlsConfig := c.Clone()

	tlsConfig.NextProtos = []string{http2.NextProtoTLS}

	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = constants.APIDomain
	}

	// This logic still appears to be necessary to force client to always send
	// a certificate regardless of the server setting. Otherwise the client may pick
	// not to send the client certificate by looking at certificate request.
	if len(tlsConfig.Certificates) > 0 {
		cert := tlsConfig.Certificates[0]
		tlsConfig.Certificates = nil
		tlsConfig.GetClientCertificate = func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &cert, nil
		}
	}

	return tlsConfig
}
