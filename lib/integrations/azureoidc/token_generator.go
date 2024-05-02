// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package azureoidc

import (
	"context"
	"crypto"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

// azureDefaultJWTAudience is the default audience used by Azure
// when setting up an enterprise application.
const azureDefaultJWTAudience = "api://AzureADTokenExchange"

// KeyStoreManager defines methods to get signers using the server's keystore.
type KeyStoreManager interface {
	// GetJWTSigner selects a usable JWT keypair from the given keySet and returns a [crypto.Signer].
	GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error)
}

// Cache is the subset of the cached resources that the AWS OIDC Token Generation queries.
type Cache interface {
	// GetCertAuthority returns cert authority by id
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetClusterName returns local cluster name of the current auth server
	GetClusterName(...services.MarshalOption) (types.ClusterName, error)

	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)
}

// GenerateEntraOIDCToken returns a JWT suitable for OIDC authentication to MS Graph API.
func GenerateEntraOIDCToken(ctx context.Context, cache Cache, manager KeyStoreManager, clock clockwork.Clock) (string, error) {
	clusterName, err := cache.GetClusterName()
	if err != nil {
		return "", trace.Wrap(err)
	}

	ca, err := cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OIDCIdPCA,
		DomainName: clusterName.GetClusterName(),
	}, true /*loadKeys*/)
	if err != nil {
		return "", trace.Wrap(err)
	}

	issuer, err := oidc.IssuerForCluster(ctx, cache)
	if err != nil {
		return "", trace.Wrap(err)
	}

	signer, err := manager.GetJWTSigner(ctx, ca)
	if err != nil {
		return "", trace.Wrap(err)
	}

	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), clock)
	if err != nil {
		return "", trace.Wrap(err)
	}

	token, err := privateKey.SignEntraOIDC(jwt.SignParams{
		Audience: azureDefaultJWTAudience,
		Subject:  "teleport-azure", // TODO(justinas): consider moving this to a constant or a field in the integration settings
		Issuer:   issuer,
		Expires:  clock.Now().Add(time.Hour),
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}
