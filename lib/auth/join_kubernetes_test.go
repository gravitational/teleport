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
	v1 "k8s.io/api/authentication/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
)

type mockKubernetesTokenValidator struct {
	tokens map[string]v1.UserInfo
}

func (m *mockKubernetesTokenValidator) Validate(_ context.Context, token string) (*v1.UserInfo, error) {
	userInfo, ok := m.tokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}

	return &userInfo, nil
}

func TestAuth_RegisterUsingToken_Kubernetes(t *testing.T) {
	// Test setup

	// Creating an auth server with mock Kubernetes token validator
	tokens := map[string]v1.UserInfo{
		"matching-first-rule-token1":  {Username: "system:serviceaccount:namespace1:service-account1"},
		"matching-second-rule-token2": {Username: "system:serviceaccount:namespace2:service-account2"},
		"user-token":                  {Username: "namespace1:service-account1"},
	}

	var withTokenValidator ServerOption = func(server *Server) error {
		server.kubernetesTokenValidator = &mockKubernetesTokenValidator{tokens: tokens}
		return nil
	}

	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir(), withTokenValidator)
	require.NoError(t, err)
	auth := p.a

	// Creating and loading our two Kubernetes Provision tokens
	pt1, err := types.NewProvisionTokenFromSpec("my-token-1", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
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
	pt2, err := types.NewProvisionTokenFromSpec("my-token-2", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{ServiceAccount: "namespace2:service-account1"},
				{ServiceAccount: "namespace2:service-account2"},
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, auth.CreateToken(ctx, pt1))
	require.NoError(t, auth.CreateToken(ctx, pt2))

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
			"successful token join (first rule)",
			"matching-first-rule-token1",
			pt1,
			nil,
		},
		{
			"successful token join (second rule)",
			"matching-second-rule-token2",
			pt2,
			nil,
		},
		{
			"failed token join (wrong provisionToken)",
			"matching-second-rule-token2",
			pt1,
			trace.AccessDenied("kubernetes token user info did not match any allow rules"),
		},
		{
			"failed token join (unknown kubeToken)",
			"unknown",
			pt1,
			errMockInvalidToken,
		},
		{
			"failed token join (user token)",
			"user-token",
			pt1,
			trace.AccessDenied("kubernetes token user info did not match any allow rules"),
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
