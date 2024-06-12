/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package reversetunnel

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
)

type certificateCache struct {
	mu sync.Mutex

	cache      *ttlmap.TTLMap
	authClient authclient.ClientI
}

// newHostCertificateCache creates a shared host certificate cache that is
// used by the forwarding server.
func newHostCertificateCache(authClient authclient.ClientI) (*certificateCache, error) {
	native.PrecomputeKeys() // ensure native package is set to precompute keys
	cache, err := ttlmap.New(defaults.HostCertCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &certificateCache{
		cache:      cache,
		authClient: authClient,
	}, nil
}

// getHostCertificate will fetch a certificate from the cache. If the certificate
// is not in the cache, it will be generated, put in the cache, and returned.
// Multiple callers can arrive and generate a host certificate at the same time.
// This is a tradeoff to prevent long delays here due to the expensive
// certificate generation call.
func (c *certificateCache) getHostCertificate(ctx context.Context, addr string, additionalPrincipals []string) (ssh.Signer, error) {
	var certificate ssh.Signer
	var err error
	var ok bool

	var principals []string
	principals = append(principals, addr)
	principals = append(principals, additionalPrincipals...)

	certificate, ok = c.get(strings.Join(principals, "."))
	if !ok {
		certificate, err = c.generateHostCert(ctx, principals)
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
func (c *certificateCache) generateHostCert(ctx context.Context, principals []string) (ssh.Signer, error) {
	if len(principals) == 0 {
		return nil, trace.BadParameter("at least one principal must be provided")
	}

	// Generate public/private keypair.
	privBytes, pubBytes, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate a SSH host certificate.
	clusterName, err := c.authClient.GetDomainName(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certBytes, err := c.authClient.GenerateHostCert(
		ctx,
		pubBytes,
		principals[0],
		principals[0],
		principals,
		clusterName,
		types.RoleNode,
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
