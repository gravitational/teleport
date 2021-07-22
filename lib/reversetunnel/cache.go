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
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshca"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
)

type certificateCache struct {
	mu sync.Mutex

	cache      *ttlmap.TTLMap
	authClient auth.ClientI
	keygen     sshca.Authority
}

// newHostCertificateCache creates a shared host certificate cache that is
// used by the forwarding server.
func newHostCertificateCache(keygen sshca.Authority, authClient auth.ClientI) (*certificateCache, error) {
	cache, err := ttlmap.New(defaults.HostCertCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &certificateCache{
		keygen:     keygen,
		cache:      cache,
		authClient: authClient,
	}, nil
}

// getHostCertificate will fetch a certificate from the cache. If the certificate
// is not in the cache, it will be generated, put in the cache, and returned. Mul
// Multiple callers can arrive and generate a host certificate at the same time.
// This is a tradeoff to prevent long delays here due to the expensive
// certificate generation call.
func (c *certificateCache) getHostCertificate(addr string, additionalPrincipals []string) (ssh.Signer, error) {
	var certificate ssh.Signer
	var err error
	var ok bool

	var principals []string
	principals = append(principals, addr)
	principals = append(principals, additionalPrincipals...)

	certificate, ok = c.get(strings.Join(principals, "."))
	if !ok {
		certificate, err = c.generateHostCert(principals)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = c.set(addr, certificate, defaults.HostCertCacheTime)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return certificate, nil
}

// get is goroutine safe and will return a ssh.Signer for a principal from
// the cache.
func (c *certificateCache) get(addr string) (ssh.Signer, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	certificate, ok := c.cache.Get(addr)
	if !ok {
		return nil, false
	}

	certificateSigner, ok := certificate.(ssh.Signer)
	if !ok {
		return nil, false
	}

	return certificateSigner, true
}

// set is goroutine safe and will set a ssh.Signer for a principal in
// the cache.
func (c *certificateCache) set(addr string, certificate ssh.Signer, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.cache.Set(addr, certificate, ttl)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// generateHostCert will generate a SSH host certificate for a given
// principal.
func (c *certificateCache) generateHostCert(principals []string) (ssh.Signer, error) {
	if len(principals) == 0 {
		return nil, trace.BadParameter("at least one principal must be provided")
	}

	// Generate public/private keypair.
	privBytes, pubBytes, err := c.keygen.GetNewKeyPairFromPool()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate a SSH host certificate.
	clusterName, err := c.authClient.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certBytes, err := c.authClient.GenerateHostCert(
		pubBytes,
		principals[0],
		principals[0],
		principals,
		clusterName,
		types.SystemRoles{types.RoleNode},
		0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create a *ssh.Certificate
	privateKey, err := ssh.ParsePrivateKey(privBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := sshutils.ParseCertificate(certBytes)
	if err != nil {
		return nil, err
	}

	// return a ssh.Signer
	s, err := ssh.NewCertSigner(cert, privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s, nil
}
