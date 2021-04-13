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
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/identity"
	"github.com/gravitational/teleport/api/profile"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http2"
)

// Credentials are used to authenticate the API auth client. Some Credentials
// also provide other functionality, such as automatic address discovery and ssh
// connectivity.
//
// Since there are several Credential loaders to choose from, here's a quick breakdown:
//
//  - Profile Credentials are the easiest to get started with. All you have to do is login
//    on your device with `tsh login`. Your Teleport proxy address and credentials will
//    automatically be located and used. However, the other options don't necessarily require
//    a login step and have the ability to authenticate long lived clients.
//
//  - IdentityFile Credentials are the most well rounded in terms of usability, functionality,
//    and customizability. Identity files can be generated through `tsh login` or `tctl auth sign`,
//    making them ideal for both long lived proxy and auth server connections.
//
//  - Key Pair Credentials have a much simpler implementation than the first two Credentials listed,
//    and may feel more familiar. These are good for authenticating client's hosted on the auth server.
//
//  - TLS Credentials leave everything up to the client user. This isn't recommended for most
//    users and is mostly used internally, but some users may find that this fits their use case best.
//
// Visit https://goteleport.com/docs/api-reference/#credentials to see a
// feature matrix comparing each loader directly.
//
// See the examples below for an example of each loader.
type Credentials interface {
	// Dialer is used to create a dialer used to connect to the Auth server.
	Dialer(cfg Config) (ContextDialer, error)
	// TLSConfig returns TLS configuration used to authenticate the client.
	TLSConfig() (*tls.Config, error)
	// SSHClientConfig returns SSH configuration used to connect to Auth through a reverse tunnel.
	SSHClientConfig() (*ssh.ClientConfig, error)
}

// LoadTLS is used to load Credentials directly from a *tls.Config.
func LoadTLS(tlsConfig *tls.Config) Credentials {
	return &tlsConfigCreds{
		tlsConfig: tlsConfig,
	}
}

// tlsConfigCreds use a defined *tls.Config to provide TLS authentication.
// tlsConfigCreds can only be used to connect directly to a local Teleport Auth server.
type tlsConfigCreds struct {
	tlsConfig *tls.Config
}

// Dialer is used to dial a connection to an Auth server.
func (c *tlsConfigCreds) Dialer(cfg Config) (ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig returns TLS configuration.
func (c *tlsConfigCreds) TLSConfig() (*tls.Config, error) {
	if c.tlsConfig == nil {
		return nil, trace.BadParameter("tls config is nil")
	}
	return configureTLS(c.tlsConfig), nil
}

// SSHClientConfig returns SSH configuration.
func (c *tlsConfigCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
	return nil, trace.NotImplemented("no ssh config")
}

// LoadKeyPair is used to load Credentials from a certicate keypair on disk.
// A new keypair can be generated with tctl.
// See the example below.
func LoadKeyPair(certFile string, keyFile string, caFile string) Credentials {
	return &keypairCreds{
		certFile: certFile,
		keyFile:  keyFile,
		caFile:   caFile,
	}
}

// keypairCreds use a certificate keypair to provide TLS authentication.
// keypairCreds can only be used to connect directly to a local Teleport Auth server.
type keypairCreds struct {
	certFile string
	keyFile  string
	caFile   string
}

// Dialer is used to dial a connection to an Auth server.
func (c *keypairCreds) Dialer(cfg Config) (ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig returns TLS configuration.
func (c *keypairCreds) TLSConfig() (*tls.Config, error) {
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

	return configureTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}), nil
}

// SSHClientConfig returns SSH configuration.
func (c *keypairCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
	return nil, trace.NotImplemented("no ssh config")
}

// LoadIdentityFile is used to load Credentials from an identity file on disk.
// A new identity file can be generated with tsh or tctl.
// See the example below.
func LoadIdentityFile(path string) Credentials {
	return &identityCreds{
		path: path,
	}
}

// identityCreds use an identity file to provide TLS authentication
// and ssh connectivity. They can be used to connect to an Auth server
// directly (local), or through the cluster's reverse proxy or web proxy.
type identityCreds struct {
	path         string
	identityFile *identity.IdentityFile
}

// Dialer is used to dial a connection to an Auth server.
func (c *identityCreds) Dialer(cfg Config) (ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig returns TLS configuration.
func (c *identityCreds) TLSConfig() (*tls.Config, error) {
	if err := c.load(); err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := c.identityFile.TLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return configureTLS(tlsConfig), nil
}

// SSHClientConfig returns SSH configuration.
func (c *identityCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
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
func (c *identityCreds) load() error {
	if c.identityFile != nil {
		return nil
	}
	var err error
	if c.identityFile, err = identity.ReadIdentityFile(c.path); err != nil {
		return trace.BadParameter("identity file could not be decoded: %v", err)
	}
	return nil
}

// LoadProfile is used to load Credentials from a tsh profile on disk.
// A new profile can be generated with tsh.
// See the example below.
func LoadProfile(dir, name string) Credentials {
	return &profileCreds{
		dir:  dir,
		name: name,
	}
}

// profileCreds use a tsh profile to provide TLS authentication, ssh
// connectivity, and automatic address discovery. They can be used to
// connect to an Auth server directly (local), or through the cluster's
// reverse proxy or web proxy. The cluster's web proxy address will
// be retrieved from the profile and used to connect over ssh.
type profileCreds struct {
	dir     string
	name    string
	profile *profile.Profile
}

// Dialer is used to dial a connection to an Auth server.
func (c *profileCreds) Dialer(cfg Config) (ContextDialer, error) {
	sshConfig, err := c.SSHClientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewTunnelDialerWithAddressDiscovery(
		*sshConfig,
		cfg.KeepAlivePeriod,
		cfg.DialTimeout,
		c.profile.WebProxyAddr,
		cfg.InsecureAddressDiscovery,
	), nil
}

// TLSConfig returns TLS configuration.
func (c *profileCreds) TLSConfig() (*tls.Config, error) {
	if err := c.load(); err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := c.profile.TLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return configureTLS(tlsConfig), nil
}

// SSHClientConfig returns SSH configuration.
func (c *profileCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
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
func (c *profileCreds) load() error {
	if c.profile != nil {
		return nil
	}
	var err error
	if c.profile, err = profile.ProfileFromDir(c.dir, c.name); err != nil {
		return trace.BadParameter("profile could not be decoded: %v", err)
	}
	return nil
}

func configureTLS(c *tls.Config) *tls.Config {
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
