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
	// if len(c.TLS.Certificates) == 0 {
	// 	return trace.BadParameter("invalid TLS, missing Certificates")
	// }
	// if c.TLS.RootCAs == nil {
	// 	return trace.BadParameter("invalid TLS, missing RootCAs")
	// }
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

// LoadKeyPair attempts to load credentials from files of a certificate key pair and root CAs
// These certs can be saved to disk with `tctl auth sign --out=path`.
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

// CredentialsProvider allow the client to load credentials dynamically on client initilization
// TODO(Joerger): and when the provider is updated asynchronously.
type CredentialsProvider interface {
	// Load attempts to load credentials from the provider
	Load() (*Credentials, error)
	// DetectReload detects if the provider's credentials source has been updated.
	DetectReload() (bool, error)
}

// IdentityFileProvider uses an identity file to load credentials.
// An identity file can be saved to disk by running `tsh login --out=identity_file_path`.
type IdentityFileProvider struct {
	Path string
}

// Load attempts to load credentials from the provider.
func (p *IdentityFileProvider) Load() (*Credentials, error) {
	creds, err := LoadIdentityFile(p.Path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := creds.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}

// DetectReload detects if the provider's identity file has been updated.
// This can be used to spin up a goroutine that reloads credentials for the client.
func (p *IdentityFileProvider) DetectReload() (bool, error) {
	// TODO (Joerger)
	return false, nil
}

// KeyPairProvider uses files of a certificate key pair and root CAs to load credentials.
// These files can be saved to disk with `tctl auth sign --out=path`.
// EX: path=/certs/admin creates three files - /certs/admin.(key|crt|cas).
type KeyPairProvider struct {
	CrtFile string
	KeyFile string
	CAsFile string
}

// Load attempts to load credentials from the provider.
func (p *KeyPairProvider) Load() (*Credentials, error) {
	creds, err := LoadKeyPair(p.CrtFile, p.KeyFile, p.CAsFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := creds.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}

// DetectReload detects if the provider's files have been updated.
// This can be used to spin up a goroutine that reloads credentials for the client.
func (p *KeyPairProvider) DetectReload() (bool, error) {
	// TODO (Joerger)
	return false, nil
}

// TLSProvider uses files of a certificate key pair and root CAs to load credentials.
// These files can be generated with `tctl auth sign --out=path`.
// EX: path=/certs/admin creates three files - /certs/admin.(key|crt|cas).
type TLSProvider struct {
	TLS *tls.Config
}

// Load attempts to load credentials from the provider's certificate paths
func (p *TLSProvider) Load() (*Credentials, error) {
	creds := LoadTLS(p.TLS)
	if err := creds.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}

// DetectReload detects if the provider's files have been updated.
// TLSProvider used a pointer to TLS, so a reload already automatically
// propagates to the client, so this always returns false.
func (p *TLSProvider) DetectReload() (bool, error) {
	return false, nil
}
