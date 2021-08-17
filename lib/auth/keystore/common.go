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

package keystore

import (
	"crypto/x509/pkix"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

func newSSHKeyPair(keyStore KeyStore) (*types.SSHKeyPair, error) {
	sshKey, cryptoSigner, err := keyStore.GenerateRSA()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshSigner, err := ssh.NewSignerFromSigner(cryptoSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicKey := ssh.MarshalAuthorizedKey(sshSigner.PublicKey())
	return &types.SSHKeyPair{
		PublicKey:      publicKey,
		PrivateKey:     sshKey,
		PrivateKeyType: KeyType(sshKey),
	}, nil
}

func newTLSKeyPair(keyStore KeyStore, clusterName string) (*types.TLSKeyPair, error) {
	tlsKey, signer, err := keyStore.GenerateRSA()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tlsca.GenerateSelfSignedCAWithSigner(
		signer,
		pkix.Name{
			CommonName:   clusterName,
			Organization: []string{clusterName},
		}, nil, defaults.CATTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.TLSKeyPair{
		Cert:    tlsCert,
		Key:     tlsKey,
		KeyType: KeyType(tlsKey),
	}, nil
}

func newJWTKeyPair(keyStore KeyStore) (*types.JWTKeyPair, error) {
	jwtKey, signer, err := keyStore.GenerateRSA()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicKey, err := utils.MarshalPublicKey(signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.JWTKeyPair{
		PublicKey:      publicKey,
		PrivateKey:     jwtKey,
		PrivateKeyType: KeyType(jwtKey),
	}, nil
}
