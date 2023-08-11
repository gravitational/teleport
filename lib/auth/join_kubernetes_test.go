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
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/kubernetestoken"
)

type mockK8STokenReviewValidator struct {
	tokens map[string]*kubernetestoken.ValidationResult
}

func (m *mockK8STokenReviewValidator) Validate(_ context.Context, token string) (*kubernetestoken.ValidationResult, error) {
	result, ok := m.tokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}

	return result, nil
}

func TestAuth_RegisterUsingToken_Kubernetes(t *testing.T) {
	// Test setup

	// Creating an auth server with mock Kubernetes token validator
	tokenReviewTokens := map[string]*kubernetestoken.ValidationResult{
		"matching-implicit-in-cluster": {Username: "system:serviceaccount:namespace1:service-account1"},
		// "matching-explicit-in-cluster" intentionally matches the second allow
		// rule of explicitInCluster to ensure all rules are processed.
		"matching-explicit-in-cluster": {Username: "system:serviceaccount:namespace2:service-account2"},
		"user-token":                   {Username: "namespace1:service-account1"},
	}
	jwksTokens := map[string]*kubernetestoken.ValidationResult{
		"jwks-matching-service-account":   {Username: "system:serviceaccount:static-jwks:matching"},
		"jwks-mismatched-service-account": {Username: "system:serviceaccount:static-jwks:mismatched"},
	}

	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir(), func(server *Server) error {
		server.k8sTokenReviewValidator = &mockK8STokenReviewValidator{tokens: tokenReviewTokens}
		server.k8sJWKSValidator = func(_ time.Time, _ []byte, _ string, token string) (*kubernetestoken.ValidationResult, error) {
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
