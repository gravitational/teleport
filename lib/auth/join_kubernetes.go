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

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	kubetoken "github.com/gravitational/teleport/lib/kube/token"
)

// GetK8sTokenReviewValidator returns the currently configured validator for
// Kubernetes Token Review API (in-cluster) tokens.
func (a *Server) GetK8sTokenReviewValidator() kubetoken.InClusterValidator {
	return a.k8sTokenReviewValidator
}

// SetK8sTokenReviewValidator sets the token review validator implementation,
// used in tests.
func (a *Server) SetK8sTokenReviewValidator(validator kubetoken.InClusterValidator) {
	a.k8sTokenReviewValidator = validator
}

// GetK8sJWKSValidator returns the currently configured validator for Kubernetes
// static_jwks tokens.
func (a *Server) GetK8sJWKSValidator() kubetoken.JWKSValidator {
	return a.k8sJWKSValidator
}

// SetK8sJWKSValidator sets the Kubernetes JWKS validator implementation, used
// in tests.
func (a *Server) SetK8sJWKSValidator(validator kubetoken.JWKSValidator) {
	a.k8sJWKSValidator = validator
}

// GetK8sOIDCValidator returns the currently configured validator for Kubernetes
// OIDC tokens.
func (a *Server) GetK8sOIDCValidator() *kubetoken.KubernetesOIDCTokenValidator {
	return a.k8sOIDCValidator
}

func (a *Server) checkKubernetesJoinRequest(
	ctx context.Context,
	req *types.RegisterUsingTokenRequest,
	unversionedToken types.ProvisionToken,
) (*kubetoken.ValidationResult, error) {
	clusterName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := kubetoken.CheckIDToken(ctx, &kubetoken.CheckIDTokenParams{
		ProvisionToken:     unversionedToken,
		Clock:              a.GetClock(),
		ClusterName:        clusterName,
		IDToken:            []byte(req.IDToken),
		InClusterValidator: a.k8sTokenReviewValidator,
		JWKSValidator:      a.k8sJWKSValidator,
		OIDCValidator:      a.k8sOIDCValidator,
	})

	return result, trace.Wrap(err)
}
