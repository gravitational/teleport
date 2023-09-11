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
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kubernetestoken"
)

type k8sTokenReviewValidator interface {
	Validate(context.Context, string) (*kubernetestoken.ValidationResult, error)
}

type k8sJWKSValidator func(now time.Time, jwksData []byte, clusterName string, token string) (*kubernetestoken.ValidationResult, error)

func (a *Server) checkKubernetesJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) (*kubernetestoken.ValidationResult, error) {
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
	var result *kubernetestoken.ValidationResult
	switch token.Spec.Kubernetes.Type {
	case types.KubernetesJoinTypeStaticJWKS:
		clusterName, err := a.GetDomainName()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result, err = a.k8sJWKSValidator(
			a.clock.Now(),
			[]byte(token.Spec.Kubernetes.StaticJWKS.JWKS),
			clusterName,
			req.IDToken,
		)
		if err != nil {
			return nil, trace.WrapWithMessage(err, "reviewing kubernetes token with static_jwks")
		}
	case types.KubernetesJoinTypeInCluster, types.KubernetesJoinTypeUnspecified:
		result, err = a.k8sTokenReviewValidator.Validate(ctx, req.IDToken)
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
		"validated_identity": result,
		"token":              token.GetName(),
	}).Info("Kubernetes workload trying to join cluster")

	return result, trace.Wrap(checkKubernetesAllowRules(token, result))
}

func checkKubernetesAllowRules(pt *types.ProvisionTokenV2, got *kubernetestoken.ValidationResult) error {
	// If a single rule passes, accept the token
	for _, rule := range pt.Spec.Kubernetes.Allow {
		wantUsername := fmt.Sprintf("%s:%s", kubernetestoken.ServiceAccountNamePrefix, rule.ServiceAccount)
		if wantUsername != got.Username {
			continue
		}
		return nil
	}

	return trace.AccessDenied("kubernetes token did not match any allow rules")
}
