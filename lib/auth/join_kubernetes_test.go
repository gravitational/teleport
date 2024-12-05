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
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	kubetoken "github.com/gravitational/teleport/lib/kube/token"
)

type mockK8STokenReviewValidator struct {
	tokens map[string]*kubetoken.ValidationResult
}

func (m *mockK8STokenReviewValidator) Validate(_ context.Context, token, _ string) (*kubetoken.ValidationResult, error) {
	result, ok := m.tokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}

	return result, nil
}

func TestAuth_RegisterUsingToken_Kubernetes(t *testing.T) {
	// Test setup

	// Creating an auth server with mock Kubernetes token validator
	tokenReviewTokens := map[string]*kubetoken.ValidationResult{
		"matching-implicit-in-cluster": {Username: "system:serviceaccount:namespace1:service-account1"},
		// "matching-explicit-in-cluster" intentionally matches the second allow
		// rule of explicitInCluster to ensure all rules are processed.
		"matching-explicit-in-cluster": {Username: "system:serviceaccount:namespace2:service-account2"},
		"user-token":                   {Username: "namespace1:service-account1"},
	}
	jwksTokens := map[string]*kubetoken.ValidationResult{
		"jwks-matching-service-account":   {Username: "system:serviceaccount:static-jwks:matching"},
		"jwks-mismatched-service-account": {Username: "system:serviceaccount:static-jwks:mismatched"},
	}

	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir(), func(server *Server) error {
		server.k8sTokenReviewValidator = &mockK8STokenReviewValidator{tokens: tokenReviewTokens}
		server.k8sJWKSValidator = func(_ time.Time, _ []byte, _ string, token string) (*kubetoken.ValidationResult, error) {
			result, ok := jwksTokens[token]
			if !ok {
				return nil, errMockInvalidToken
			}
			return result, nil
		}
		return nil
	})
	require.NoError(t, err)
	auth := p.a

	// Creating and loading our two Kubernetes ProvisionTokens
	implicitInClusterPT, err := types.NewProvisionTokenFromSpec("implicit-in-cluster", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{ServiceAccount: "namespace1:service-account1"},
				{ServiceAccount: "namespace1:service-account2"},
			},
		},
	})
	require.NoError(t, err)
	explicitInClusterPT, err := types.NewProvisionTokenFromSpec("explicit-in-cluster", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Type: types.KubernetesJoinTypeInCluster,
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{ServiceAccount: "namespace2:service-account1"},
				{ServiceAccount: "namespace2:service-account2"},
			},
		},
	})
	require.NoError(t, err)
	staticJWKSPT, err := types.NewProvisionTokenFromSpec("static-jwks", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Type: types.KubernetesJoinTypeStaticJWKS,
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{ServiceAccount: "static-jwks:matching"},
			},
			StaticJWKS: &types.ProvisionTokenSpecV2Kubernetes_StaticJWKSConfig{
				JWKS: "fake-jwks",
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, auth.CreateToken(ctx, implicitInClusterPT))
	require.NoError(t, auth.CreateToken(ctx, explicitInClusterPT))
	require.NoError(t, auth.CreateToken(ctx, staticJWKSPT))

	// Building a joinRequest builder
	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	newRequest := func(token, idToken string) *types.RegisterUsingTokenRequest {
		return &types.RegisterUsingTokenRequest{
			Token:        token,
			HostID:       "host-id",
			Role:         types.RoleNode,
			IDToken:      idToken,
			PublicTLSKey: tlsPublicKey,
			PublicSSHKey: sshPublicKey,
		}
	}

	tests := []struct {
		name           string
		kubeToken      string
		provisionToken types.ProvisionToken
		expectedErr    error
	}{
		{
			"in_cluster (implicit): success",
			"matching-implicit-in-cluster",
			implicitInClusterPT,
			nil,
		},
		{
			"in_cluster (explicit): success",
			"matching-explicit-in-cluster",
			explicitInClusterPT,
			nil,
		},
		{
			"in_cluster: service account rule mismatch",
			"matching-explicit-in-cluster",
			implicitInClusterPT,
			trace.AccessDenied("kubernetes token did not match any allow rules"),
		},
		{
			"in_cluster: failed token join (unknown kubeToken)",
			"unknown",
			implicitInClusterPT,
			errMockInvalidToken,
		},
		{
			"in_cluster: failed token join (user token)",
			"user-token",
			implicitInClusterPT,
			trace.AccessDenied("kubernetes token did not match any allow rules"),
		},
		{
			"static_jwks: success",
			"jwks-matching-service-account",
			staticJWKSPT,
			nil,
		},
		{
			"static_jwks: service account rule mismatch",
			"jwks-mismatched-service-account",
			staticJWKSPT,
			trace.AccessDenied("kubernetes token did not match any allow rules"),
		},
		{
			"static_jwks: validation fails",
			"unknown",
			staticJWKSPT,
			errMockInvalidToken,
		},
	}

	// Doing the real test
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := auth.RegisterUsingToken(ctx, newRequest(tt.provisionToken.GetName(), tt.kubeToken))
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
