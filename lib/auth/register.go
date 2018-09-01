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
func LocalRegister(id IdentityID, authServer *AuthServer, additionalPrincipals []string) (*Identity, error) {
	keys, err := authServer.GenerateServerKeys(GenerateServerKeysRequest{
		HostID:               id.HostUUID,
		NodeName:             id.NodeName,
		Roles:                teleport.Roles{id.Role},
		AdditionalPrincipals: additionalPrincipals,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ReadIdentityFromKeyPair(keys.Key, keys.Cert, keys.TLSCert, keys.TLSCACerts)
}

// RegisterParams specifies parameters
// for first time register operation with auth server
type RegisterParams struct {
	// DataDir is the data directory
	// storing CA certificate
	DataDir string
	// Token is a secure token to join the cluster
	Token string
	// ID is identity ID
	ID IdentityID
	// Servers is a list of auth servers to dial
	Servers []utils.NetAddr
	// AdditionalPrincipals is a list of additional principals to dial
	AdditionalPrincipals []string
	// PrivateKey is a PEM encoded private key (not passed to auth servers)
	PrivateKey []byte
	// PublicTLSKey is a server's public key to sign
	PublicTLSKey []byte
	// PublicSSHKey is a server's public SSH key to sign
	PublicSSHKey []byte
	// CipherSuites is a list of cipher suites to use for TLS client connection
	CipherSuites []uint16
}

// Register is used to generate host keys when a node or proxy are running on different hosts
// than the auth server. This method requires provisioning tokens to prove a valid auth server
// was used to issue the joining request.
func Register(params RegisterParams) (*Identity, error) {
	tok, err := readToken(params.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig := utils.TLSConfig(params.CipherSuites)
	certPath := filepath.Join(params.DataDir, defaults.CACertFile)
	certBytes, err := utils.ReadPath(certPath)
	if err != nil {
		// Only support secure cluster joins in the next releases
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		message := fmt.Sprintf(`Your configuration is insecure! Registering without TLS certificate authority, to fix this warning add ca.cert to %v, you can get ca.cert using 'tctl auth export --type=tls > ca.cert'`,
			params.DataDir)
		log.Warning(message)
		tlsConfig.InsecureSkipVerify = true
	} else {
		cert, err := tlsca.ParseCertificatePEM(certBytes)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse certificate at %v", certPath)
		}
		log.Infof("Joining remote cluster %v.", cert.Subject.CommonName)
		certPool := x509.NewCertPool()
		certPool.AddCert(cert)
		tlsConfig.RootCAs = certPool
	}
	client, err := NewTLSClient(params.Servers, tlsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer client.Close()

	// Get the SSH and X509 certificates
	keys, err := client.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:                tok,
		HostID:               params.ID.HostUUID,
		NodeName:             params.ID.NodeName,
		Role:                 params.ID.Role,
		AdditionalPrincipals: params.AdditionalPrincipals,
		PublicTLSKey:         params.PublicTLSKey,
		PublicSSHKey:         params.PublicSSHKey,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ReadIdentityFromKeyPair(
		params.PrivateKey, keys.Cert, keys.TLSCert, keys.TLSCACerts)
}

// ReRegisterParams specifies parameters for re-registering
// in the cluster (rotating certificates for existing members)
type ReRegisterParams struct {
	// Client is an authenticated client using old credentials
	Client ClientI
	// ID is identity ID
	ID IdentityID
	// AdditionalPrincipals is a list of additional principals to dial
	AdditionalPrincipals []string
	// PrivateKey is a PEM encoded private key (not passed to auth servers)
	PrivateKey []byte
	// PublicTLSKey is a server's public key to sign
	PublicTLSKey []byte
	// PublicSSHKey is a server's public SSH key to sign
	PublicSSHKey []byte
}

// ReRegister renews the certificates and private keys based on the client's existing identity.
func ReRegister(params ReRegisterParams) (*Identity, error) {
	hostID, err := params.ID.HostID()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keys, err := params.Client.GenerateServerKeys(GenerateServerKeysRequest{
		HostID:               hostID,
		NodeName:             params.ID.NodeName,
		Roles:                teleport.Roles{params.ID.Role},
		AdditionalPrincipals: params.AdditionalPrincipals,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ReadIdentityFromKeyPair(params.PrivateKey, keys.Cert, keys.TLSCert, keys.TLSCACerts)
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
	// trim newlines as tokens in files tend to have newlines
	return strings.TrimSpace(string(out)), nil
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
