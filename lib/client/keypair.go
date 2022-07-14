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

package client

import (
	"github.com/gravitational/teleport/api/utils/sshutils/ppk"
	"github.com/gravitational/teleport/lib/auth/native"

	"github.com/gravitational/trace"
)

type KeyPair interface {
	PrivateKeyPEM() []byte
	PublicKeyPEM() []byte
	// PPKFile returns a PuTTY PPK-formatted keypair
	PPKFile() ([]byte, error)
}

type RSAKeyPair struct {
	privateKeyPEM []byte
	publicKeyPEM  []byte
}

func GenerateRSAKeyPair() (*RSAKeyPair, error) {
	priv, pub, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewRSAKeyPair(priv, pub), nil
}

func NewRSAKeyPair(priv, pub []byte) *RSAKeyPair {
	return &RSAKeyPair{
		privateKeyPEM: priv,
		publicKeyPEM:  pub,
	}
}

func (r *RSAKeyPair) PublicKeyPEM() []byte {
	return r.publicKeyPEM
}

func (r *RSAKeyPair) PrivateKeyPEM() []byte {
	return r.privateKeyPEM
}

func (r *RSAKeyPair) PPKFile() ([]byte, error) {
	ppkFile, err := ppk.ConvertToPPK(r.privateKeyPEM, r.publicKeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ppkFile, nil
}
