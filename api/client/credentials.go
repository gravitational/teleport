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
	"github.com/gravitational/trace"
	"golang.org/x/net/http2"
)

// Credentials are used to authenticate the client's connection to the server.
type Credentials struct {
	TLS *tls.Config
}

// CheckAndSetDefaults checks and sets default credential values.
func (c *Credentials) CheckAndSetDefaults() error {
	if c.TLS == nil {
		return trace.BadParameter("missing TLS config")
	}
	c.TLS.NextProtos = []string{http2.NextProtoTLS}
	if c.TLS.ServerName == "" {
		c.TLS.ServerName = constants.APIDomain
	}
	return nil
}

// LoadTLS returns Credentials with the given TLS config.
func LoadTLS(tls *tls.Config) *Credentials {
	return &Credentials{TLS: tls}
}

// LoadIdentityFile attempts to load credentials from the specified identity file path.
// An identity file can be saved to disk by running `tsh login --out=identity_file_path`.
func LoadIdentityFile(path string) (*Credentials, error) {
	idf, err := ReadIdentityFile(path)
	if err != nil {
		return nil, trace.BadParameter("identity file could not be decoded: %v", err)
	}

	tls, err := idf.TLS()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return LoadTLS(tls), nil
}

// LoadKeyPair attempts to load credentials from a certificate key pair and root CAs.
// Those files can be saved to disk with `tctl auth sign --out=path`.
// EX: path=/certs/admin creates three files - /certs/admin.(key|crt|cas).
func LoadKeyPair(crtFile, keyFile, casFile string) (*Credentials, error) {
	cert, err := tls.LoadX509KeyPair(crtFile, keyFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCerts, err := ioutil.ReadFile(casFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCerts); !ok {
		return nil, trace.Errorf("invalid TLS CA cert PEM")
	}

	return LoadTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}), nil
}

// CredentialsProvider has a defined source of credentials, which can be dynamically loaded.
// CredentialsProviders are used to load and test credentials during client initialization.
// Use the function NewIdentityFileProvider or NewKeyPairProvider to create a CredentialLoader.
type CredentialsProvider interface {
	// Load attempts to load credentials from the provider.
	Load() (*Credentials, error)
}

// NewIdentityFileProvider returns a CredentialsProvider that uses an identity file to load credentials.
// An identity file can be saved to disk by running `tsh login --out=path`.
func NewIdentityFileProvider(path string) CredentialsProvider {
	return &identityFileProvider{path}
}

type identityFileProvider struct {
	path string
}

func (p *identityFileProvider) Load() (*Credentials, error) {
	creds, err := LoadIdentityFile(p.path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := creds.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}

// NewKeyPairProvider returns a CredentialsProvider that uses a certificate key pair and root CAs
// to load credentials. These files can be saved to disk with `tctl auth sign --out=path`.
// EX: path=/certs/admin creates three files - /certs/admin.(key|crt|cas).
func NewKeyPairProvider(crtFile, keyFile, casFile string) CredentialsProvider {
	return &keyPairProvider{crtFile, keyFile, casFile}
}

type keyPairProvider struct {
	crtFile string
	keyFile string
	casFile string
}

func (p *keyPairProvider) Load() (*Credentials, error) {
	creds, err := LoadKeyPair(p.crtFile, p.keyFile, p.casFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := creds.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}

// NewTLSProvider returns a CredentialsProvider that uses a preloaded tls.Config to load credentials.
// The tls.Config given is stored as a pointer and can be updated dynamically, but the provided
// Credentials won't be updated.
func NewTLSProvider(tls *tls.Config) CredentialsProvider {
	return &tlsProvider{tls}
}

type tlsProvider struct {
	tls *tls.Config
}

func (p *tlsProvider) Load() (*Credentials, error) {
	tls := p.tls.Clone()
	creds := LoadTLS(tls)
	if err := creds.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}
