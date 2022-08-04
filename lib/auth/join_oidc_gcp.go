package auth

import (
	"context"

	"github.com/coreos/go-oidc"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// example jwks endpoint
// https://www.googleapis.com/oauth2/v3/certs

type gcpIDToken struct {
	Sub    string `json:"sub"`
	Google struct {
		ComputeEngine struct {
			ProjectID     string `json:"project_id"`
			ProjectNumber int    `json:"project_number"`
			InstanceID    string `json:"instance_id"`
			InstanceName  string `json:"instance_name"`
			Zone          string `json:"zone"`
		} `json:"compute_engine"`
	} `json:"google"`
}

type clusterNameProvider interface {
	GetClusterName(...services.MarshalOption) (types.ClusterName, error)
}
type gcpOIDCTokenValidator struct {
	provider            *oidc.Provider
	clock               clockwork.Clock
	clusterNameProvider clusterNameProvider
}

func newGCPOIDCTokenChecker(
	ctx context.Context,
	clock clockwork.Clock,
	cnp clusterNameProvider,
) (*gcpOIDCTokenValidator, error) {
	p, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &gcpOIDCTokenValidator{
		provider: p,
	}, nil
}

func (gcpOIDCTokenValidator) JoinMethod() types.JoinMethod {
	return types.JoinMethodOIDCGCP
}

func (v *gcpOIDCTokenValidator) Check(
	ctx context.Context,
	pt types.ProvisionToken,
	tokenString string,
) error {
	clusterName, err := v.clusterNameProvider.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	oidcVerifier := v.provider.Verifier(&oidc.Config{
		// Expect the audience of the token to be the name of the cluster.
		// This reduces the risk that a JWT leaked by another application can
		// be used against Teleport.
		ClientID: clusterName.GetClusterName(),
		Now:      v.clock.Now,
	})

	token, err := oidcVerifier.Verify(ctx, tokenString)
	if err != nil {
		return trace.Wrap(err)
	}

	var parsedClaims gcpIDToken
	if err := token.Claims(&parsedClaims); err != nil {
		return trace.Wrap(err)
	}

	// If a single rule passes the checks, accept the token
	return v.evaluateAllowRules(parsedClaims, pt)
}

func (v *gcpOIDCTokenValidator) evaluateAllowRules(token gcpIDToken, pt types.ProvisionToken) error {
	for _, rule := range pt.GetAllowRules() {
		if rule.Sub != "" && rule.Sub != token.Sub {
			continue
		}

		if rule.Google != nil && rule.Google.ComputeEngine != nil {
			rule := rule.Google.ComputeEngine
			token := token.Google.ComputeEngine
			if rule.ProjectID != "" && rule.ProjectID != token.ProjectID {
				continue
			}
			if rule.ProjectNumber != 0 && int(rule.ProjectNumber) != token.ProjectNumber {
				continue
			}
			if rule.InstanceID != "" && rule.InstanceID != token.InstanceID {
				continue
			}
			if rule.InstanceName != "" && rule.InstanceName != token.InstanceName {
				continue
			}
			if rule.Zone != "" && rule.Zone != token.Zone {
				continue
			}
		}

		// The rule passed, so we should return without error
		return nil
	}

	return ErrTokenNotMatchAllow
}
