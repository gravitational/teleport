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
	// TLS is the client's TLS config
	TLS *tls.Config
}

// CheckAndSetDefaults checks and sets default credential values.
func (c *Credentials) CheckAndSetDefaults() error {
	if c.TLS == nil {
		return trace.BadParameter("missing TLS config")
	}
	// clone to make sure it doesn't alter a TLS config used elsewhere (auth client)
	c.TLS = c.TLS.Clone()
	c.TLS.NextProtos = []string{http2.NextProtoTLS}
	if c.TLS.ServerName == "" {
		c.TLS.ServerName = constants.APIDomain
	}
	return nil
}

// DefaultCreds attempts to load credentials, and defaults to loading from profile.
// LoadOptions can be provided to override the default behaviour, and instead attempt
// the provided methods until one succeeds or none do.
func DefaultCreds(opts ...CredsLoaderFn) (*Credentials, error) {
	// Default to loading creds from profile
	if len(opts) == 0 {
		creds, err := ProfileCreds("")
		return creds, trace.Wrap(err)
	}

	// Attempt to load credentials with the given load options.
	errs := []error{}
	for _, opt := range opts {
		creds, err := opt()
		if err == nil {
			return creds, nil
		}
		errs = append(errs, err)
	}

	return nil, trace.Wrap(trace.NewAggregate(errs...), "failed to load credentials with given options")
}

// CredsLoaderFn A list of CredsLoaderFns can be given to the function DefaultCreds
// to chain credential loading options.
type CredsLoaderFn func() (*Credentials, error)

// ProfileCredsLoader is a CredsLoaderFn for ProfileCreds. Multiple paths
// can be provided in order to fall back if the first paths fail. If no path
// is provided, this uses the default path.
func ProfileCredsLoader(paths ...string) CredsLoaderFn {
	return func() (*Credentials, error) {
		if len(paths) == 0 {
			return ProfileCreds("")
		}

		errs := []error{}
		for _, path := range paths {
			creds, err := ProfileCreds(path)
			if err == nil {
				return creds, err
			}
			errs = append(errs, err)
		}
		return nil, trace.NewAggregate(errs...)
	}
}

// IdentityFileCredsLoader is a CredsLoaderFn for IdentityFileCreds. Multiple
// paths can be provided in order to fall back if the first paths fail.
func IdentityFileCredsLoader(paths ...string) CredsLoaderFn {
	return func() (*Credentials, error) {
		if len(paths) == 0 {
			return nil, trace.Errorf("must provide at least one path to paths parameter")
		}

		errs := []error{}
		for _, path := range paths {
			creds, err := IdentityFileCreds(path)
			if err == nil {
				return creds, err
			}
			errs = append(errs, err)
		}
		return nil, trace.NewAggregate(errs...)
	}
}

// CertFilesCredsLoader is a CredsLoaderFn for CertFilesCreds. Multiple
// paths can be provided  in order to fall back if the first paths fail.
func CertFilesCredsLoader(paths ...string) CredsLoaderFn {
	return func() (*Credentials, error) {
		errs := []error{}
		for _, path := range paths {
			creds, err := CertFilesCreds(path)
			if err == nil {
				return creds, err
			}
			errs = append(errs, err)
		}
		return nil, trace.NewAggregate(errs...)
	}
}

// TLSCredsLoader is a CredsLoaderFn for TLSCreds
func TLSCredsLoader(tls *tls.Config) CredsLoaderFn {
	return func() (*Credentials, error) {
		return TLSCreds(tls), nil
	}
}

// ProfileCreds attempts to load credentials from the default profile,
// which is set by logging in with `tsh login`.
func ProfileCreds(path string) (*Credentials, error) {
	if path == "" {
		path = defaultProfilePath()
	}
	profile, err := ProfileFromDir(path, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tls, err := profile.TLS()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return TLSCreds(tls), nil
}

// IdentityFileCreds attempts to load credentials from the specified identity file's path.
// An identity file can be saved to disk by running `tsh login --out=identity_file_path`.
func IdentityFileCreds(path string) (*Credentials, error) {
	idf, err := ReadIdentityFile(path)
	if err != nil {
		return nil, trace.BadParameter("identity file could not be decoded: %v", err)
	}

	tls, err := idf.TLS()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return TLSCreds(tls), nil
}

// CertFilesCreds attempts to load credentials from the specified certificates path.
// These certs can be generated with `tctl auth sign --out=path`.
// EX: path=/certs/admin creates three files - /certs/admin.(key|crt|cas).
func CertFilesCreds(path string) (*Credentials, error) {
	cert, err := tls.LoadX509KeyPair(path+".crt", path+".key")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCerts, err := ioutil.ReadFile(path + ".cas")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCerts); !ok {
		return nil, trace.Errorf("invalid TLS CA cert PEM")
	}

	return TLSCreds(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}), nil
}

// TLSCreds returns Credentials with the given TLS config.
func TLSCreds(tls *tls.Config) *Credentials {
	return &Credentials{TLS: tls}
}
