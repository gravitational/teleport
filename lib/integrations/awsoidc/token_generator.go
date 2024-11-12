/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package awsoidc

import (
	"context"
	"crypto"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

// Cache is the subset of the cached resources that the AWS OIDC Token Generation queries.
type Cache interface {
	// GetIntegration returns the specified integration resources.
	GetIntegration(ctx context.Context, name string) (types.Integration, error)

	// GetClusterName returns local cluster name of the current auth server
	GetClusterName(...services.MarshalOption) (types.ClusterName, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)
}

// KeyStoreManager defines methods to get signers using the server's keystore.
type KeyStoreManager interface {
	// GetJWTSigner selects a usable JWT keypair from the given keySet and returns a [crypto.Signer].
	GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error)
}

// GenerateAWSOIDCTokenRequest contains the required elements to generate an AWS OIDC Token (JWT).
type GenerateAWSOIDCTokenRequest struct {
	// Integration is the AWS OIDC Integration name.
	// This field is only used to obtain custom Issuers (those stored at S3 buckets).
	// If empty, the default issuer for the cluster (its public endpoint URL) will be used.
	Integration string
	// Username is the JWT Username (on behalf of claim)
	Username string
	// Subject is the JWT Subject (subject claim)
	Subject string
	// Clock is used to mock time
	Clock clockwork.Clock
}

// CheckAndSetDefaults checks the request params.
func (g *GenerateAWSOIDCTokenRequest) CheckAndSetDefaults() error {
	if g.Username == "" {
		return trace.BadParameter("username missing")
	}
	if g.Subject == "" {
		return trace.BadParameter("missing subject")
	}
	if g.Clock == nil {
		g.Clock = clockwork.NewRealClock()
	}

	return nil
}

// IssuerForIntegration returns the issuer for a given integration.
// Returns the default Issuer (oidc.IssuerForCluster) if integrationName is empty.
// All calls should be replaced with oidc.IssuerForCluster when IssuerS3URI is removed (it is currently deprecated).
func issuerForIntegration(ctx context.Context, cacheClt Cache, integrationName string) (string, error) {
	if integrationName == "" {
		issuer, err := oidc.IssuerForCluster(ctx, cacheClt, "")
		return issuer, trace.Wrap(err)
	}

	integration, err := cacheClt.GetIntegration(ctx, integrationName)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if integration.GetSubKind() != types.IntegrationSubKindAWSOIDC {
		return "", trace.BadParameter("integration subkind (%s) mismatch", integration.GetSubKind())
	}

	if integration.GetAWSOIDCIntegrationSpec() == nil {
		return "", trace.BadParameter("missing spec fields for %q (%q) integration", integration.GetName(), integration.GetSubKind())
	}

	issuerS3URI := integration.GetAWSOIDCIntegrationSpec().IssuerS3URI
	if issuerS3URI == "" {
		issuer, err := oidc.IssuerForCluster(ctx, cacheClt, "")
		return issuer, trace.Wrap(err)
	}

	issuerS3URL, err := url.Parse(issuerS3URI)
	if err != nil {
		return "", trace.Wrap(err)
	}
	prefix := strings.TrimLeft(issuerS3URL.Path, "/")
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", issuerS3URL.Host, prefix), nil
}

// GenerateAWSOIDCToken generates a token to be used when executing an AWS OIDC Integration action.
func GenerateAWSOIDCToken(ctx context.Context, cacheClt Cache, keyStoreManager KeyStoreManager, req GenerateAWSOIDCTokenRequest) (string, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return "", trace.Wrap(err)
	}

	issuer, err := issuerForIntegration(ctx, cacheClt, req.Integration)
	if err != nil {
		return "", trace.Wrap(err)
	}

	clusterName, err := cacheClt.GetClusterName()
	if err != nil {
		return "", trace.Wrap(err)
	}

	ca, err := cacheClt.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OIDCIdPCA,
		DomainName: clusterName.GetClusterName(),
	}, true /*loadKeys*/)
	if err != nil {
		return "", trace.Wrap(err)
	}

	signer, err := keyStoreManager.GetJWTSigner(ctx, ca)
	if err != nil {
		return "", trace.Wrap(err)
	}

	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), req.Clock)
	if err != nil {
		return "", trace.Wrap(err)
	}

	token, err := privateKey.SignAWSOIDC(jwt.SignParams{
		Username: req.Username,
		Audience: types.IntegrationAWSOIDCAudience,
		Subject:  req.Subject,
		Issuer:   issuer,
		// Token expiration is not controlled by the Expires property.
		// It is defined by assumed IAM Role's "Maximum session duration" (usually 1h).
		Expires: req.Clock.Now().Add(time.Hour),
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}
