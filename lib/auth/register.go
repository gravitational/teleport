/*
Copyright 2015 Gravitational, Inc.

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

package auth

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// LocalRegister is used to generate host keys when a node or proxy is running within the same process
// as the auth server. This method does not need to use provisioning tokens.
func LocalRegister(dataDir string, id IdentityID, authServer *AuthServer) error {
	keys, err := authServer.GenerateServerKeys(id.HostUUID, id.NodeName, teleport.Roles{id.Role})
	if err != nil {
		return trace.Wrap(err)
	}
	return writeKeys(dataDir, id, keys.Key, keys.Cert, keys.TLSCert, keys.TLSCACerts[0])
}

// Register is used to generate host keys when a node or proxy are running on different hosts
// than the auth server. This method requires provisioning tokens to prove a valid auth server
// was used to issue the joining request.
func Register(dataDir, token string, id IdentityID, servers []utils.NetAddr) error {
	tok, err := readToken(token)
	if err != nil {
		return trace.Wrap(err)
	}
	tlsConfig := utils.TLSConfig()
	certPath := filepath.Join(dataDir, defaults.CACertFile)
	certBytes, err := utils.ReadPath(certPath)
	if err != nil {
		// DELETE IN: 2.6.0
		// Only support secure cluster joins in the next releases
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		message := fmt.Sprintf(`Your configuration is insecure! Registering without TLS certificate authority, to fix this warning add ca.cert to %v, you can get ca.cert using 'tctl auth export --type=tls > ca.cert'`, dataDir)
		log.Warning(message)
		tlsConfig.InsecureSkipVerify = true
	} else {
		cert, err := tlsca.ParseCertificatePEM(certBytes)
		if err != nil {
			return trace.Wrap(err, "failed to parse certificate at %v", certPath)
		}
		log.Infof("Securely joining remote cluster %v.", cert.Subject.CommonName)
		certPool := x509.NewCertPool()
		certPool.AddCert(cert)
		tlsConfig.RootCAs = certPool
	}
	client, err := NewTLSClient(servers, tlsConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	// get the host certificate and keys
	keys, err := client.RegisterUsingToken(tok, id.HostUUID, id.NodeName, id.Role)
	if err != nil {
		return trace.Wrap(err)
	}

	return writeKeys(dataDir, id, keys.Key, keys.Cert, keys.TLSCert, keys.TLSCACerts[0])
}

// ReRegister renews the certificates  and private keys based on the existing
// identity ID
func ReRegister(dataDir string, clt ClientI, id IdentityID) error {
	hostID, err := id.HostID()
	if err != nil {
		return trace.Wrap(err)
	}
	keys, err := clt.GenerateServerKeys(
		hostID, id.NodeName, teleport.Roles{id.Role})
	if err != nil {
		return trace.Wrap(err)
	}
	return writeKeys(dataDir, id, keys.Key, keys.Cert, keys.TLSCert, keys.TLSCACerts[0])
}

func RegisterNewAuth(domainName, token string, servers []utils.NetAddr) error {
	tok, err := readToken(token)
	if err != nil {
		return trace.Wrap(err)
	}
	method, err := NewTokenAuth(domainName, tok)
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := NewTunClient(
		"auth.server.register",
		servers,
		domainName,
		method)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	return client.RegisterNewAuthServer(tok)
}

func readToken(token string) (string, error) {
	if !strings.HasPrefix(token, "/") {
		return token, nil
	}
	// treat it as a file
	out, err := ioutil.ReadFile(token)
	if err != nil {
		return "", nil
	}
	return string(out), nil
}

// PackedKeys is a collection of private key, SSH host certificate
// and TLS certificate and certificate authority issued the certificate
type PackedKeys struct {
	// Key is a private key
	Key []byte `json:"key"`
	// Cert is an SSH host cert
	Cert []byte `json:"cert"`
	// TLSCert is an X509 certificate
	TLSCert []byte `json:"tls_cert"`
	// TLSCACerts is a list of certificate authorities
	TLSCACerts [][]byte `json:"tls_ca_certs"`
}
