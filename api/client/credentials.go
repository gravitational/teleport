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

// Credentials are used to authenticate to Auth.
type Credentials interface {
	// Dialer is used to create a dialer used to connect to Auth.
	Dialer(cfg Config) (ContextDialer, error)
	// TLSConfig returns TLS configuration used to connect to Auth.
	TLSConfig() (*tls.Config, error)
	// SSHClientConfig returns SSH configuration used to connect to Proxy through tunnel.
	SSHClientConfig() (*ssh.ClientConfig, error)
}

// LoadTLS is used to load credentials directly from another *tls.Config.
func LoadTLS(tlsConfig *tls.Config) *TLSConfigCreds {
	return &TLSConfigCreds{
		tlsConfig: tlsConfig,
	}
}

// TLSConfigCreds are used to authenticate the client
// with a predefined *tls.Config.
type TLSConfigCreds struct {
	tlsConfig *tls.Config
}

// Dialer is used to dial a connection to Auth.
func (c *TLSConfigCreds) Dialer(cfg Config) (ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig returns TLS configuration used to connect to Auth.
func (c *TLSConfigCreds) TLSConfig() (*tls.Config, error) {
	if c.tlsConfig == nil {
		return nil, trace.BadParameter("tls config is nil")
	}
	return configure(c.tlsConfig), nil
}

// SSHClientConfig returns SSH configuration used to connect to Proxy.
func (c *TLSConfigCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
	return nil, trace.NotImplemented("no ssh config")
}

// LoadKeyPair is used to load credentials from files on disk.
func LoadKeyPair(certFile string, keyFile string, caFile string) *KeyPairCreds {
	return &KeyPairCreds{
		certFile: certFile,
		keyFile:  keyFile,
		caFile:   caFile,
	}
}

// KeyPairCreds are used to authenticate the client
// with certificates generated in the given file paths.
type KeyPairCreds struct {
	certFile string
	keyFile  string
	caFile   string
}

// Dialer is used to dial a connection to Auth.
func (c *KeyPairCreds) Dialer(cfg Config) (ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig returns TLS configuration used to connect to Auth.
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

// SSHClientConfig returns SSH configuration used to connect to Proxy.
func (c *KeyPairCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
	return nil, trace.NotImplemented("no ssh config")
}

// LoadIdentityFile is used to load credentials from an identity file on disk.
func LoadIdentityFile(path string) *IdentityCreds {
	return &IdentityCreds{
		path: path,
	}
}

// IdentityCreds are used to authenticate the client
// with an identity file generated in the given file path.
type IdentityCreds struct {
	path         string
	identityFile *IdentityFile
}

// Dialer is used to dial a connection to Auth.
func (c *IdentityCreds) Dialer(cfg Config) (ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig returns TLS configuration used to connect to Auth.
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

// SSHClientConfig returns SSH configuration used to connect to Proxy.
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

// LoadProfile is used to load credentials from a tsh Profile.
// If dir is not specified, the default profile path will be used.
// If name is not specified, the current profile name will be used.
func LoadProfile(dir, name string) *ProfileCreds {
	return &ProfileCreds{
		dir:  dir,
		name: name,
	}
}

// ProfileCreds are used to authenticate the client
// with a tsh profile with the given directory and name.
type ProfileCreds struct {
	dir     string
	name    string
	profile *Profile
}

// Dialer is used to dial a connection to Auth.
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

// TLSConfig returns TLS configuration used to connect to Auth.
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

// SSHClientConfig returns SSH configuration used to connect to Proxy.
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
