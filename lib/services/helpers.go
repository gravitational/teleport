/*
Copyright 2020 Gravitational, Inc.

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

package services

import (
	"crypto/rsa"
	"crypto/x509/pkix"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"

	"golang.org/x/crypto/ssh"
)

// NewTestCA returns new test authority with a test key as a public and
// signing key
func NewTestCA(caType CertAuthType, clusterName string, privateKeys ...[]byte) *CertAuthorityV2 {
	// privateKeys is to specify another RSA private key
	if len(privateKeys) == 0 {
		privateKeys = [][]byte{fixtures.PEMBytes["rsa"]}
	}
	keyBytes := privateKeys[0]
	rsaKey, err := ssh.ParseRawPrivateKey(keyBytes)
	if err != nil {
		panic(err)
	}

	signer, err := ssh.NewSignerFromKey(rsaKey)
	if err != nil {
		panic(err)
	}

	key, cert, err := tlsca.GenerateSelfSignedCAWithPrivateKey(rsaKey.(*rsa.PrivateKey), pkix.Name{
		CommonName:   clusterName,
		Organization: []string{clusterName},
	}, nil, defaults.CATTL)
	if err != nil {
		panic(err)
	}

	return &CertAuthorityV2{
		Kind:    KindCertAuthority,
		SubKind: string(caType),
		Version: V2,
		Metadata: Metadata{
			Name:      clusterName,
			Namespace: defaults.Namespace,
		},
		Spec: CertAuthoritySpecV2{
			Type:         caType,
			ClusterName:  clusterName,
			CheckingKeys: [][]byte{ssh.MarshalAuthorizedKey(signer.PublicKey())},
			SigningKeys:  [][]byte{keyBytes},
			TLSKeyPairs:  []TLSKeyPair{{Cert: cert, Key: key}},
		},
	}
}
