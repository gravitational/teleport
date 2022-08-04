/*
Copyright 2022 Gravitational, Inc.

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

package keys

import (
	"crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"runtime"

	"github.com/gravitational/teleport/api/constants"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type RSAPrivateKey rsa.PrivateKey

func NewRSAPrivateKey(rsaPrivateKey *rsa.PrivateKey) *RSAPrivateKey {
	return (*RSAPrivateKey)(rsaPrivateKey)
}

// Returns a new RSAPrivateKey from an existing PEM-encoded RSA key pair.
func ParseRSAPrivateKey(priv []byte) (*RSAPrivateKey, error) {
	rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewRSAPrivateKey(rsaPrivateKey), nil
}

func (r *RSAPrivateKey) Public() crypto.PublicKey {
	return (*rsa.PrivateKey)(r).Public()
}

func (r *RSAPrivateKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	return (*rsa.PrivateKey)(r).Sign(rand, digest, opts)
}

func (r *RSAPrivateKey) Equal(other crypto.PrivateKey) bool {
	return (*rsa.PrivateKey)(r).Equal(other)
}

func (r *RSAPrivateKey) PrivateKeyDataPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey((*rsa.PrivateKey)(r)),
	})
}

func (r *RSAPrivateKey) TLSCertificate(certRaw []byte) (cert tls.Certificate, err error) {
	cert, err = tls.X509KeyPair(certRaw, r.PrivateKeyDataPEM())
	if err != nil {
		return cert, trace.Wrap(err)
	}
	return cert, nil
}

// AsAgentKeys converts Key struct to a []*agent.AddedKey. All elements
// of the []*agent.AddedKey slice need to be loaded into the agent!
func (r *RSAPrivateKey) AsAgentKeys(sshCert *ssh.Certificate) []agent.AddedKey {
	// put a teleport identifier along with the teleport user into the comment field
	comment := fmt.Sprintf("teleport:%v", sshCert.KeyId)

	// On all OS'es, return the certificate with the private key embedded.
	agents := []agent.AddedKey{
		{
			PrivateKey:       (*rsa.PrivateKey)(r),
			Certificate:      sshCert,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
	}

	if runtime.GOOS != constants.WindowsOS {
		// On Unix, return the certificate (with embedded private key) as well as
		// a private key.
		//
		// (2016-08-01) have a bug in how they use certificates that have been lo
		// This is done because OpenSSH clients older than OpenSSH 7.3/7.3p1aded
		// in an agent. Specifically when you add a certificate to an agent, you can't
		// just embed the private key within the certificate, you have to add the
		// certificate and private key to the agent separately. Teleport works around
		// this behavior to ensure OpenSSH interoperability.
		//
		// For more details see the following: https://bugzilla.mindrot.org/show_bug.cgi?id=2550
		// WARNING: callers expect the returned slice to be __exactly as it is__

		agents = append(agents, agent.AddedKey{
			PrivateKey:       (*rsa.PrivateKey)(r),
			Certificate:      nil,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		})
	}

	return agents
}
