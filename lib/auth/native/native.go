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
package native

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"sync"
	"time"

	"github.com/gravitational/log"
	"github.com/gravitational/trace"

	"golang.org/x/crypto/ssh"
)

const precalculatedKeysNum = 20

type keyPair struct {
	privPem  []byte
	pubBytes []byte
}

type nauth struct {
	generatedKeys []keyPair
	*sync.Mutex
}

func New() *nauth {
	return &nauth{
		generatedKeys: make([]keyPair, 0, precalculatedKeysNum),
		Mutex:         &sync.Mutex{},
	}
}

func (n *nauth) GenerateKeyPair(passphrase string) ([]byte, []byte, error) {
	n.Lock()
	defer n.Unlock()
	if len(n.generatedKeys) == 0 {
		return n.generateKeyPair()
	}
	go n.precalculateKey()
	key := n.generatedKeys[len(n.generatedKeys)-1]
	n.generatedKeys[len(n.generatedKeys)-1] = keyPair{}
	n.generatedKeys = n.generatedKeys[:len(n.generatedKeys)-1]

	return key.privPem, key.pubBytes, nil
}

func (n *nauth) precalculateKey() {
	for {
		privPem, pubBytes, err := n.generateKeyPair()
		if err != nil {
			log.Errorf(err.Error())
			continue
		}
		key := keyPair{
			privPem:  privPem,
			pubBytes: pubBytes,
		}
		n.Lock()
		defer n.Unlock()
		if len(n.generatedKeys) >= precalculatedKeysNum {
			return
		}
		n.generatedKeys = append(n.generatedKeys, key)
		return
	}
}

func (n *nauth) generateKeyPair() ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	privDer := x509.MarshalPKCS1PrivateKey(priv)
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDer,
	}
	privPem := pem.EncodeToMemory(&privBlock)

	pub, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	pubBytes := ssh.MarshalAuthorizedKey(pub)
	return privPem, pubBytes, nil
}

func (n *nauth) GenerateHostCert(pkey, key []byte, id, hostname, role string, ttl time.Duration) ([]byte, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(key)
	if err != nil {
		return nil, err
	}
	validBefore := uint64(ssh.CertTimeInfinity)
	if ttl != 0 {
		b := time.Now().Add(ttl)
		validBefore = uint64(b.UnixNano())
	}
	cert := &ssh.Certificate{
		ValidPrincipals: []string{hostname},
		Key:             pubKey,
		ValidBefore:     validBefore,
		CertType:        ssh.HostCert,
	}
	cert.Permissions.Extensions = make(map[string]string)
	cert.Permissions.Extensions["role"] = role
	signer, err := ssh.ParsePrivateKey(pkey)
	if err != nil {
		return nil, err
	}
	if err := cert.SignCert(rand.Reader, signer); err != nil {
		return nil, err
	}
	return ssh.MarshalAuthorizedKey(cert), nil
}

func (n *nauth) GenerateUserCert(pkey, key []byte, id, username string, ttl time.Duration) ([]byte, error) {
	if (ttl > MaxCertDuration) || (ttl < MinCertDuration) {
		return nil, trace.Errorf("wrong certificate ttl")
	}
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(key)
	if err != nil {
		return nil, err
	}
	validBefore := uint64(ssh.CertTimeInfinity)
	if ttl != 0 {
		b := time.Now().Add(ttl)
		validBefore = uint64(b.Unix())
	}
	cert := &ssh.Certificate{
		ValidPrincipals: []string{username},
		Key:             pubKey,
		ValidBefore:     validBefore,
		CertType:        ssh.UserCert,
	}
	signer, err := ssh.ParsePrivateKey(pkey)
	if err != nil {
		return nil, err
	}
	if err := cert.SignCert(rand.Reader, signer); err != nil {
		return nil, err
	}
	return ssh.MarshalAuthorizedKey(cert), nil
}

const (
	MinCertDuration = time.Minute
	MaxCertDuration = 30 * time.Hour
)
