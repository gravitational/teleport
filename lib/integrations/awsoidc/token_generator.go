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
	"path"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

// Cache is the subset of the cached resources that the AWS OIDC Token Generation queries.
type Cache interface {
	// GetIntegration returns the specified integration resource.
	GetIntegration(ctx context.Context, req *integrationpb.GetIntegrationRequest) (types.Integration, error)

	// GetClusterName returns local cluster name of the current auth server
	GetClusterName(ctx context.Context) (types.ClusterName, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetProxies returns a list of registered proxies.
	//
	// Deprecated: Prefer paginated variant [ListProxyServers].
	//
	// TODO(kiosion): DELETE IN 21.0.0
	GetProxies() ([]types.Server, error)

	// ListProxyServers returns a paginated list of registered proxies.
	ListProxyServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error)
}

// KeyStoreManager defines methods to get signers using the server's keystore.
type KeyStoreManager interface {
	// GetJWTSigner selects a usable JWT keypair from the given keySet and returns a [crypto.Signer].
	GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error)
}

// GenerateAWSOIDCTokenRequest contains the required elements to generate an AWS OIDC Token (JWT).
type GenerateAWSOIDCTokenRequest struct {
	// Integration is the AWS OIDC Integration name.
	// This field is only used to obtain custom Issuers (those stored at S3 buckets, or the
	// scope specific issuer of a scoped integration).
	// If empty, the default issuer for the cluster (its public endpoint URL) will be used.
	Integration string
	// Scope is the scope of the AWS OIDC Integration. Empty for unscoped integrations.
	Scope string
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

const ScopedIssuerPrefix = "/scope"

// ScopedIssuerPath returns the issuer path of a scoped AWS OIDC integration.
// As per RFD 0309, an integration under /staging/west::my-integration
// gets the issuer path that looks like: /scope/staging/west/my-integration.
func ScopedIssuerPath(scope, name string) (string, error) {
	if err := apiscopes.StrongValidate(scope); err != nil {
		return "", trace.Wrap(err)
	}
	if err := apiscopes.StrongValidateResourceName(name); err != nil {
		return "", trace.Wrap(err)
	}
	return path.Join(ScopedIssuerPrefix, scope, name), nil
}

// issuerForIntegration returns the issuer for a given integration.
// Returns the default Issuer (oidc.IssuerForCluster) if the integration name is empty.
// All unscoped calls should be replaced with oidc.IssuerForCluster when IssuerS3URI is removed (it is currently deprecated).
func issuerForIntegration(ctx context.Context, cacheClt Cache, integrationRef apiscopes.QualifiedName) (string, error) {
	if integrationRef.Name == "" {
		issuer, err := oidc.IssuerForCluster(ctx, cacheClt)
		return issuer, trace.Wrap(err)
	}

	integration, err := cacheClt.GetIntegration(ctx, integrationpb.GetIntegrationRequest_builder{
		Name:  integrationRef.Name,
		Scope: integrationRef.Scope,
	}.Build())
	if err != nil {
		return "", trace.Wrap(err)
	}

	if integration.GetSubKind() != types.IntegrationSubKindAWSOIDC {
		return "", trace.BadParameter("integration subkind (%s) mismatch", integration.GetSubKind())
	}

	spec := integration.GetAWSOIDCIntegrationSpec()
	if spec == nil {
		return "", trace.BadParameter("missing spec fields for %q (%q) integration", integration.GetName(), integration.GetSubKind())
	}

	if integration.GetScope() != "" {
		issuerPath, err := ScopedIssuerPath(integration.GetScope(), integration.GetName())
		if err != nil {
			return "", trace.Wrap(err)
		}
		issuer, err := oidc.IssuerForCluster(ctx, cacheClt, issuerPath)
		return issuer, trace.Wrap(err)
	}

	issuerS3URI := spec.IssuerS3URI
	if issuerS3URI == "" {
		issuer, err := oidc.IssuerForCluster(ctx, cacheClt)
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

	issuer, err := issuerForIntegration(ctx, cacheClt, apiscopes.QualifiedName{Scope: req.Scope, Name: req.Integration})
	if err != nil {
		return "", trace.Wrap(err)
	}

	clusterName, err := cacheClt.GetClusterName(ctx)
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
