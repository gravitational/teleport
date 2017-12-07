/*
Copyright 2017 Gravitational, Inc.

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

package reversetunnel

import (
	"net"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"

	"github.com/gravitational/trace"
)

// certificateCache holds host certificates used by the recording proxy. It's
// created at the package level because both local site and remote site use it.
var certificateCache map[string]ssh.Signer = make(map[string]ssh.Signer)

// cacheMutex is for go routine safety.
var cacheMutex sync.Mutex

// getCertificate will fetch a certificate from the cache. If the certificate
// is not in the cache, it will be generated, put in the cache, and returned.
func getCertificate(addr string, authService auth.ClientI) (ssh.Signer, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	var certificate ssh.Signer
	var err error
	var ok bool

	// extract the principal from the address
	principal, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certificate, ok = certificateCache[principal]
	if !ok {
		certificate, err = generateHostCert(principal, authService)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		certificateCache[principal] = certificate
	}

	return certificate, nil
}

// generateHostCert will generate a SSH host certificate for a given principal.
func generateHostCert(principal string, authService auth.ClientI) (ssh.Signer, error) {
	keygen := native.New()
	defer keygen.Close()

	// generate public/private keypair
	privBytes, pubBytes, err := keygen.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// have auth server sign and return a host certificate to us
	clusterName, err := authService.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certBytes, err := authService.GenerateHostCert(pubBytes, principal, principal, clusterName, teleport.Roles{teleport.RoleNode}, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create a *ssh.Certificate
	privateKey, err := ssh.ParsePrivateKey(privBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		return nil, err
	}
	cert, ok := publicKey.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("not a certificate")
	}

	// return a ssh.Signer
	s, err := ssh.NewCertSigner(cert, privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s, nil
}
