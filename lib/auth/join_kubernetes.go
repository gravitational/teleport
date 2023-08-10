/*
Copyright 2022 Gravitational, Inc.

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
	"fmt"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kubernetestoken"
)

type kubernetesTokenValidator interface {
	Validate(context.Context, string) (*kubernetestoken.ServiceAccountClaims, error)
}

func (a *Server) checkKubernetesJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) (*kubernetestoken.ServiceAccountClaims, error) {
	if req.IDToken == "" {
		return nil, trace.BadParameter("IDToken not provided for Kubernetes join request")
	}
	unversionedToken, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, ok := unversionedToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter(
			"kubernetes join method only supports ProvisionTokenV2, '%T' was provided",
			unversionedToken,
		)
	}

	// Switch to join method subtype token validation.
	var claims *kubernetestoken.ServiceAccountClaims
	switch token.Spec.Kubernetes.Type {
	case types.KubernetesJoinTypeStaticJWKS:
		clusterName, err := a.GetDomainName()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		claims, err = kubernetestoken.ValidateTokenWithJWKS(
			a.clock.Now(),
			[]byte(token.Spec.Kubernetes.StaticJWKS.JWKS),
			clusterName,
			req.IDToken,
		)
		if err != nil {
			return nil, trace.WrapWithMessage(err, "reviewing kubernetes token with static_jwks")
		}
	case types.KubernetesJoinTypeInCluster, types.KubernetesJoinTypeUnspecified:
		claims, err = a.kubernetesTokenValidator.Validate(ctx, req.IDToken)
		if err != nil {
			return nil, trace.WrapWithMessage(err, "reviewing kubernetes token with in_cluster")
		}
	default:
		return nil, trace.BadParameter(
			"unsupported kubernetes join method type (%s)",
			token.Spec.Kubernetes.Type,
		)
	}

	log.WithFields(logrus.Fields{
		"claims": claims,
		"token":  token.GetName(),
	}).Info("Kubernetes workload trying to join cluster")

	return claims, trace.Wrap(checkKubernetesAllowRules(token, claims))
}

func checkKubernetesAllowRules(pt *types.ProvisionTokenV2, got *kubernetestoken.ServiceAccountClaims) error {
	// If a single rule passes, accept the token
	for _, rule := range pt.Spec.Kubernetes.Allow {
		wantSubject := fmt.Sprintf("%s:%s", kubernetestoken.ServiceAccountNamePrefix, rule.ServiceAccount)
		if wantSubject != got.Subject {
			continue
		}
		return nil
	}

	return trace.AccessDenied("kubernetes token user info did not match any allow rules")
}
