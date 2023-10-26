package auth

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

func (a *Server) RetrieveExternalCloudAuditCredentials(ctx context.Context, integration string) (aws.Credentials, error) {
	clusterName, err := a.GetDomainName()
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}

	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OIDCIdPCA,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}

	// Extract the JWT signing key and sign the claims.
	signer, err := a.GetKeyStore().GetJWTSigner(ctx, ca)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}

	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), a.clock)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}

	token, err := privateKey.SignAWSOIDC(jwt.SignParams{
		Username: a.ServerID,
		Audience: types.IntegrationAWSOIDCAudience,
		Subject:  "system:auth",
		Issuer:   "https://" + clusterName,
		Expires:  a.clock.Now().Add(time.Hour),
	})
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}

	ig, err := a.Integrations.GetIntegration(ctx, integration)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}
	awsoidcSpec := ig.GetAWSOIDCIntegrationSpec()
	if awsoidcSpec == nil {
		return aws.Credentials{}, trace.BadParameter("missing spec fields for %q (%q) integration", ig.GetName(), ig.GetSubKind())
	}
	roleARN := awsoidcSpec.RoleARN

	clusterAuditCfg, err := a.GetClusterAuditConfig(ctx)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(clusterAuditCfg.Region()), config.WithRetryMaxAttempts(10))
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}

	roleProvider := stscreds.NewWebIdentityRoleProvider(
		sts.NewFromConfig(cfg),
		roleARN,
		identityToken(token),
		func(wiro *stscreds.WebIdentityRoleOptions) {
			wiro.Duration = time.Hour
		},
	)

	creds, err := roleProvider.Retrieve(ctx)
	if err != nil {
		return aws.Credentials{}, trace.Wrap(err)
	}
	return creds, nil
}

// identityToken is an implementation of [stscreds.IdentityTokenRetriever] for returning a static token.
type identityToken string

// GetIdentityToken returns the token configured.
func (j identityToken) GetIdentityToken() ([]byte, error) {
	return []byte(j), nil
}
