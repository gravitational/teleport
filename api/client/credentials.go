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
	"fmt"
	"io/ioutil"

	"github.com/gravitational/teleport/api/constants"

	"github.com/gravitational/trace"
	"golang.org/x/net/http2"
)

// Credentials are used to authenticate the client's connection to the server
type Credentials struct {
	// TLS is the client's TLS config
	TLS *tls.Config
}

// CheckAndSetDefaults checks and sets default credential values.
func (c *Credentials) CheckAndSetDefaults() error {
	if c.TLS == nil {
		return trace.BadParameter("creds missing tls config")
	}
	c.TLS = c.TLS.Clone()
	c.TLS.NextProtos = []string{http2.NextProtoTLS}
	if c.TLS.ServerName == "" {
		c.TLS.ServerName = constants.APIDomain
	}
	return nil
}

// ProfileCreds attempts to load Credentials from the default Profile,
// which is set by logging in with `tsh login`.
func ProfileCreds() (Credentials, error) {
	profileDir := defaultProfilePath()
	profile, err := ProfileFromDir(profileDir, "")
	if err != nil {
		return Credentials{}, trace.Wrap(err)
	}

	tls, err := profile.TLS()
	if err != nil {
		return Credentials{}, trace.Wrap(err)
	}

	return TLSCreds(tls), nil
}

// IdentityCreds attempts to load Credentials from the specified identity file's path.
// You can create an identity file by running `tsh login --out=[full_file_path]`.
func IdentityCreds(path string) (Credentials, error) {
	idf, err := DecodeIdentityFile(path)
	if err != nil {
		return Credentials{}, trace.BadParameter("identity file could not be decoded: %v", err)
	}

	tls, err := idf.TLS()
	if err != nil {
		return Credentials{}, trace.Wrap(err)
	}

	return TLSCreds(tls), nil
}

// PathCreds establishes a gRPC connection to an auth server.
func PathCreds(path string) (Credentials, error) {
	cert, err := tls.LoadX509KeyPair(path+".crt", path+".key")
	if err != nil {
		return Credentials{}, trace.Wrap(err)
	}

	caCerts, err := ioutil.ReadFile(path + ".cas")
	if err != nil {
		return Credentials{}, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCerts); !ok {
		return Credentials{}, fmt.Errorf("invalid TLS CA cert PEM")
	}

	return TLSCreds(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}), nil
}

// TLSCreds returns Credentials with the given TLS config.
func TLSCreds(tls *tls.Config) Credentials {
	return Credentials{TLS: tls}
}
