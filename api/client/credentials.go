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

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"
	"golang.org/x/net/http2"
)

const (
	// ProfileDir is the default directory location where tsh profiles (and session keys) are stored
	ProfileDir       = ".tsh"
	fileExtTLSCert   = "-x509.pem"
	sessionKeyDir    = "keys"
	fileNameTLSCerts = "certs.pem"
)

// Credentials are used to authenticate the client's connection to the server
type Credentials struct {
	// TLS is the client's TLS config
	tls *tls.Config
	// err is used to propogate errors from credential loading. This allows
	// users to chain credential loading inside of NewClient().
	err error
}

// CheckAndSetDefaults checks and sets default credential values
func (c *Credentials) CheckAndSetDefaults() error {
	if c.err != nil {
		return trace.WrapWithMessage(c.err, "error in loading API creds")
	}
	if c.tls == nil {
		return trace.BadParameter("creds missing tls config")
	}
	c.tls = c.tls.Clone()
	c.tls.NextProtos = []string{http2.NextProtoTLS}
	if c.tls.ServerName == "" {
		c.tls.ServerName = constants.APIDomain
	}
	return nil
}

// TLS returns the Credentials' tls config
func (c *Credentials) TLS() *tls.Config {
	return c.tls
}

// ProfileCreds attempts to load Teleport client.Credentials from ~/.tsh/profile,
// which is set by logging in with `tsh login`.
func ProfileCreds() Credentials {
	profileDir := defaultProfilePath()
	profile, err := ProfileFromDir(profileDir, "")
	if err != nil {
		return credentialsWithErr(trace.Wrap(err))
	}

	tls, err := profile.TLS()
	if err != nil {
		return credentialsWithErr(trace.Wrap(err))
	}

	return TLSCreds(tls)
}

// IdentityCreds attempts to load Teleport client.Credentials from the specified identity file's full path.
// You can create an identity file by running `tsh login --out=[full_file_path]`.
func IdentityCreds(path string) Credentials {
	idf, err := DecodeIdentityFile(path)
	if err != nil {
		return credentialsWithErr(trace.BadParameter("identity file could not be decoded", err))
	}

	tls, err := idf.TLS()
	if err != nil {
		return credentialsWithErr(trace.Wrap(err))
	}

	return TLSCreds(tls)
}

// TLSCreds returns Credentials with the given TLS config.
func TLSCreds(tls *tls.Config) Credentials {
	return Credentials{tls: tls}
}

func credentialsWithErr(err error) Credentials {
	return Credentials{err: trace.Wrap(err)}
}
