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
	"crypto/rsa"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

type certificateCache struct {
	cache       *utils.FnCache
	authClient  authclient.ClientI
	suiteGetter cryptosuites.GetSuiteFunc
}

// newHostCertificateCache creates a shared host certificate cache that is
// used by the forwarding server. [authPrefGetter] technically offers only a
// subset of [authClient], but it allows for using a cached view of the auth
// preference.
func newHostCertificateCache(authClient authclient.ClientI, authPrefGetter cryptosuites.AuthPreferenceGetter, clock clockwork.Clock) (*certificateCache, error) {
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   defaults.HostCertCacheTime,
		Clock: clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &certificateCache{
		cache:       cache,
		authClient:  authClient,
		suiteGetter: cryptosuites.GetCurrentSuiteFromAuthPreference(authPrefGetter),
	}, nil
}

// getHostCertificate will fetch a certificate from the cache. If the certificate
// is not in the cache, it will be generated, put in the cache, and returned.
// Multiple callers can arrive and generate a host certificate at the same time.
// This is a tradeoff to prevent long delays here due to the expensive
// certificate generation call.
func (c *certificateCache) getHostCertificate(ctx context.Context, addr string, additionalPrincipals []string) (ssh.Signer, error) {
	principals := append([]string{addr}, additionalPrincipals...)
	key := strings.Join(principals, ".")

	certificate, err := utils.FnCacheGet(ctx, c.cache, key, func(ctx context.Context) (ssh.Signer, error) {
		certificate, err := c.generateHostCert(ctx, principals)
		return certificate, trace.Wrap(err)
	})
	return certificate, trace.Wrap(err)
}

// generateHostCert will generate a SSH host certificate for a given
// principal.
func (c *certificateCache) generateHostCert(ctx context.Context, principals []string) (ssh.Signer, error) {
	if len(principals) == 0 {
		return nil, trace.BadParameter("at least one principal must be provided")
	}

	// Generate public/private keypair.
	hostKey, err := cryptosuites.GenerateKey(ctx, c.suiteGetter, cryptosuites.HostSSH)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, isRSA := hostKey.Public().(*rsa.PublicKey); isRSA {
		// Start precomputing RSA keys if we ever generate one.
		// [cryptosuites.PrecomputeRSAKeys] is idempotent.
		// Doing this lazily easily handles changing signature algorithm suites
		// and won't start precomputing keys if they are never needed (a major
		// benefit in tests).
		cryptosuites.PrecomputeRSAKeys()
	}

	sshPub, err := ssh.NewPublicKey(hostKey.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPub)

	// Generate a SSH host certificate.
	clusterName, err := c.authClient.GetDomainName(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := c.authClient.TrustClient().GenerateHostCert(ctx, &trustpb.GenerateHostCertRequest{
		Key:         pubBytes,
		HostId:      principals[0],
		NodeName:    principals[0],
		Principals:  principals,
		ClusterName: clusterName,
		Role:        string(types.RoleNode),
		Ttl:         durationpb.New(0),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certBytes := res.SshCertificate

	// create a *ssh.Certificate
	sshSigner, err := ssh.NewSignerFromSigner(hostKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := sshutils.ParseCertificate(certBytes)
	if err != nil {
		return nil, err
	}

	// return a ssh.Signer
	s, err := ssh.NewCertSigner(cert, sshSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s, nil
}
