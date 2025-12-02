/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package token

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/provision"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "kubetoken")

type InClusterValidator interface {
	Validate(ctx context.Context, token, clusterName string) (*ValidationResult, error)
}

type JWKSValidator func(now time.Time, jwksData []byte, clusterName string, token string) (*ValidationResult, error)

// CheckIDTokenParams are parameters used to validate Kubernetes ID tokens, of
// all subtypes (as specified by the ProvisionToken).
type CheckIDTokenParams struct {
	ProvisionToken     provision.Token
	Clock              clockwork.Clock
	ClusterName        string
	IDToken            []byte
	InClusterValidator InClusterValidator
	JWKSValidator      JWKSValidator
	OIDCValidator      *KubernetesOIDCTokenValidator
}

func (p *CheckIDTokenParams) validate() error {
	switch {
	case p.ProvisionToken == nil:
		return trace.BadParameter("ProvisionToken is required")
	case p.Clock == nil:
		return trace.BadParameter("Clock is required")
	case p.ClusterName == "":
		return trace.BadParameter("ClusterName is required")
	case len(p.IDToken) == 0:
		return trace.BadParameter("IDToken is required")
	case p.InClusterValidator == nil:
		return trace.BadParameter("InClusterValidator is required")
	case p.JWKSValidator == nil:
		return trace.BadParameter("JWKSValidator is required")
	case p.OIDCValidator == nil:
		return trace.BadParameter("OIDCValidator is required")
	}
	return nil
}

// CheckIDToken verifies a Kubernetes IDToken, with specific verification steps
// depending on the ProvisionToken in Teleport.
func CheckIDToken(
	ctx context.Context,
	params *CheckIDTokenParams,
) (*ValidationResult, error) {
	if err := params.validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	token, ok := params.ProvisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter(
			"kubernetes join method only supports ProvisionTokenV2, '%T' was provided",
			params.ProvisionToken,
		)
	}

	// Switch to join method subtype token validation.
	var result *ValidationResult
	var err error
	switch token.Spec.Kubernetes.Type {
	case types.KubernetesJoinTypeStaticJWKS:
		result, err = params.JWKSValidator(
			params.Clock.Now(),
			[]byte(token.Spec.Kubernetes.StaticJWKS.JWKS),
			params.ClusterName,
			string(params.IDToken),
		)
		if err != nil {
			return nil, trace.WrapWithMessage(err, "reviewing kubernetes token with static_jwks")
		}
	case types.KubernetesJoinTypeOIDC:
		result, err = params.OIDCValidator.ValidateToken(
			ctx,
			token.Spec.Kubernetes.OIDC.Issuer,
			params.ClusterName,
			string(params.IDToken),
		)
		if err != nil {
			return nil, trace.Wrap(err, "reviewing kubernetes token with oidc")
		}
	case types.KubernetesJoinTypeInCluster, types.KubernetesJoinTypeUnspecified:
		result, err = params.InClusterValidator.Validate(ctx, string(params.IDToken), params.ClusterName)
		if err != nil {
			return nil, trace.WrapWithMessage(err, "reviewing kubernetes token with in_cluster")
		}
	default:
		return nil, trace.BadParameter(
			"unsupported kubernetes join method type (%s)",
			token.Spec.Kubernetes.Type,
		)
	}

	log.InfoContext(ctx, "Kubernetes workload trying to join cluster",
		"validated_identity", result,
		"token", token.GetName(),
	)

	return result, trace.Wrap(checkKubernetesAllowRules(token, result))
}

func checkKubernetesAllowRules(pt *types.ProvisionTokenV2, got *ValidationResult) error {
	// If a single rule passes, accept the token
	for _, rule := range pt.Spec.Kubernetes.Allow {
		wantUsername := fmt.Sprintf("%s:%s", ServiceAccountNamePrefix, rule.ServiceAccount)
		if wantUsername != got.Username {
			continue
		}
		return nil
	}

	return trace.AccessDenied("kubernetes token did not match any allow rules")
}
