/*
Copyright 2023 Gravitational, Inc.

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

package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/kubernetestoken"
	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"
	"time"
)

func (a *Server) RegisterUsingKubernetesRemoteMethod(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	solve client.KubernetesRemoteChallengeSolver,
) (*proto.Certs, error) {
	clientAddr, err := authz.ClientAddrFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.RemoteAddr = clientAddr.String()
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	unversionedToken, err := a.checkTokenJoinRequestCommon(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, ok := unversionedToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter(
			"%s join method only supports ProvisionTokenV2, '%T' was provided",
			types.JoinMethodKubernetesRemote,
			unversionedToken,
		)
	}
	if token.GetJoinMethod() != types.JoinMethodKubernetesRemote {
		return nil, trace.BadParameter(
			"%s join method mismatches token join method %q",
			types.JoinMethodKubernetesRemote,
			token.GetJoinMethod(),
		)
	}

	// Challenge the client to provide a JWT including the generated "challenge
	// audience".
	teleportCluster, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	challenge, err := kubernetesRemoteChallenge(teleportCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	solution, err := solve(challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Validate the solution JWT against the configured clusters and allow
	// rules.
	claims, cluster, err := validateKubernetesRemoteToken(
		ctx, token, a.clock.Now(), challenge, solution,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := checkKubernetesRemoteAllowRules(token, cluster, claims); err != nil {
		return nil, trace.Wrap(err)
	}

	// Validation success! We can return some certificates.
	if req.Role == types.RoleBot {
		certs, err := a.generateCertsBot(
			ctx,
			token,
			req,
			claims,
		)
		return certs, trace.Wrap(err)
	}
	certs, err := a.generateCerts(
		ctx,
		token,
		req,
		claims,
	)
	return certs, trace.Wrap(err)
}

func kubernetesRemoteChallenge(clusterName string) (string, error) {
	challenge, err := generateChallenge(base64.RawStdEncoding, 32)
	return fmt.Sprintf("%s/%s", clusterName, challenge), err
}

func validateKubernetesRemoteToken(
	ctx context.Context, token *types.ProvisionTokenV2, now time.Time, challenge string, solution string,
) (*kubernetestoken.ServiceAccountTokenClaims, string, error) {
	var errs []error

	// Search for the cluster which has signed the token.
	for _, c := range token.Spec.KubernetesRemote.Clusters {
		jwks, ok := c.Source.(*types.ProvisionTokenSpecV2KubernetesRemote_Cluster_StaticJWKS)
		if !ok {
			errs = append(errs, trace.BadParameter("cluster (%s): unsupported source %T", c.Name, c.Source))
			continue
		}
		claims, err := kubernetestoken.ValidateRemoteToken(
			ctx, now, []byte(jwks.StaticJWKS), challenge, solution,
		)
		if err == nil {
			// Successful match - we can return the validated claims.
			return claims, c.Name, nil
		}
		errs = append(errs, trace.Wrap(err, "cluster (%s): token did not validate", c.Name))
	}
	return nil, "", trace.Wrap(trace.NewAggregate(errs...), "jwt did not match any configured cluster")
}

func checkKubernetesRemoteAllowRules(token *types.ProvisionTokenV2, cluster string, claims *kubernetestoken.ServiceAccountTokenClaims) error {
	for _, rule := range token.Spec.KubernetesRemote.Allow {
		if len(rule.Clusters) > 0 {
			if !slices.Contains(rule.Clusters, cluster) {
				continue
			}
		}

		if claims.Sub != fmt.Sprintf("%s:%s", kubernetestoken.ServiceAccountNamePrefix, rule.ServiceAccount) {
			continue
		}
		// If a single rule passes, accept the IDToken.
		return nil
	}

	// No rules matched the token, so we reject.
	return trace.AccessDenied("id token claims did not match any allow rules")
}
