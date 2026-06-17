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

package join_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/join/jointest"
	kubetoken "github.com/gravitational/teleport/lib/kube/token"
	"github.com/gravitational/teleport/lib/oidc/fakeissuer"
	"github.com/gravitational/teleport/lib/scopes/joining"
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

func newFakeIDP(t *testing.T) *fakeissuer.IDP {
	idp, err := fakeissuer.NewIDP(slog.Default())
	require.NoError(t, err)
	t.Cleanup(idp.Close)
	return idp
}

func TestJoinKubernetes(t *testing.T) {
	// Test setup

	// Creating an auth server with mock Kubernetes token validator
	tokenReviewTokens := map[string]*kubetoken.ValidationResult{
		"matching-implicit-in-cluster": {
			Username:                "system:serviceaccount:namespace1:service-account1",
			ServiceAccountName:      "service-account1",
			ServiceAccountNamespace: "namespace1",
		},
		// "matching-explicit-in-cluster" intentionally matches the second allow
		// rule of explicitInCluster to ensure all rules are processed.
		"matching-explicit-in-cluster": {
			Username:                "system:serviceaccount:namespace2:service-account2",
			ServiceAccountName:      "service-account2",
			ServiceAccountNamespace: "namespace2",
		},
		"user-token": {Username: "namespace1:service-account1"},
	}
	jwksTokens := map[string]*kubetoken.ValidationResult{
		"jwks-matching-service-account": {
			Username:                "system:serviceaccount:static-jwks:matching",
			ServiceAccountName:      "matching",
			ServiceAccountNamespace: "static-jwks",
		},
		"jwks-mismatched-service-account": {
			Username:                "system:serviceaccount:static-jwks:mismatched",
			ServiceAccountName:      "mismatched",
			ServiceAccountNamespace: "static-jwks",
		},
	}

	ctx := t.Context()

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)

	t.Cleanup(func() { assert.NoError(t, authServer.Shutdown(t.Context())) })
	auth := authServer.Auth()

	auth.SetK8sTokenReviewValidator(&mockK8STokenReviewValidator{tokens: tokenReviewTokens})
	auth.SetK8sJWKSValidator(func(_ time.Time, _ []byte, _ string, token string) (*kubetoken.ValidationResult, error) {
		result, ok := jwksTokens[token]
		if !ok {
			return nil, errMockInvalidToken
		}
		return result, nil
	})

	oidcIDP := newFakeIDP(t)
	wrongOIDCIDP := newFakeIDP(t)
	oidcIssuerURL := oidcIDP.IssuerURL()

	oidcIDToken, err := oidcIDP.IssueKubeToken("oidc-pod", "oidc-namespace", "oidc-service-account", authServer.ClusterName())
	require.NoError(t, err)
	oidcAllowMismatchToken, err := oidcIDP.IssueKubeToken("oidc-pod", "oidc-namespace", "other-service-account", authServer.ClusterName())
	require.NoError(t, err)
	oidcInvalidAudienceToken, err := oidcIDP.IssueKubeToken("oidc-pod", "oidc-namespace", "oidc-service-account", "wrong-audience")
	require.NoError(t, err)
	oidcInvalidIssuerToken, err := wrongOIDCIDP.IssueKubeToken("oidc-pod", "oidc-namespace", "oidc-service-account", authServer.ClusterName())
	require.NoError(t, err)

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
	oidcPT, err := types.NewProvisionTokenFromSpec("oidc", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Type: types.KubernetesJoinTypeOIDC,
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{ServiceAccount: "oidc-namespace:oidc-service-account"},
			},
			OIDC: &types.ProvisionTokenSpecV2Kubernetes_OIDCConfig{
				Issuer:                  oidcIssuerURL,
				InsecureAllowHTTPIssuer: true,
			},
		},
	})
	require.NoError(t, err)
	wildcardInClusterPT, err := types.NewProvisionTokenFromSpec("wildcard-in-cluster", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Type: types.KubernetesJoinTypeInCluster,
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{ServiceAccountName: "*-account1", ServiceAccountNamespace: "*"},
			},
		},
	})
	require.NoError(t, err)
	wildcardStaticJWKSPT, err := types.NewProvisionTokenFromSpec("wildcard-static-jwks", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Type: types.KubernetesJoinTypeStaticJWKS,
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{ServiceAccountName: "match*", ServiceAccountNamespace: "*"},
			},
			StaticJWKS: &types.ProvisionTokenSpecV2Kubernetes_StaticJWKSConfig{
				JWKS: "fake-jwks",
			},
		},
	})
	require.NoError(t, err)
	wildcardOIDCPT, err := types.NewProvisionTokenFromSpec("wildcard-oidc", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Type: types.KubernetesJoinTypeOIDC,
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{ServiceAccountName: "oidc-*", ServiceAccountNamespace: "*"},
			},
			OIDC: &types.ProvisionTokenSpecV2Kubernetes_OIDCConfig{
				Issuer:                  oidcIssuerURL,
				InsecureAllowHTTPIssuer: true,
			},
		},
	})
	require.NoError(t, err)
	exactInClusterPT, err := types.NewProvisionTokenFromSpec("exact-in-cluster", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Type: types.KubernetesJoinTypeInCluster,
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{ServiceAccountName: "service-account1", ServiceAccountNamespace: "namespace1"},
			},
		},
	})
	require.NoError(t, err)
	namespaceOnlyInClusterPT, err := types.NewProvisionTokenFromSpec("namespace-only-in-cluster", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Type: types.KubernetesJoinTypeInCluster,
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{ServiceAccountName: "*", ServiceAccountNamespace: "namespace1"},
			},
		},
	})
	require.NoError(t, err)
	combinedInClusterPT, err := types.NewProvisionTokenFromSpec("combined-in-cluster", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Type: types.KubernetesJoinTypeInCluster,
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{
					ServiceAccount:          "namespace1:service-account1",
					ServiceAccountName:      "service-account?",
					ServiceAccountNamespace: "namespace1",
				},
			},
		},
	})
	require.NoError(t, err)
	combinedStrictInClusterPT, err := types.NewProvisionTokenFromSpec("combined-strict-in-cluster", time.Now().Add(10*time.Minute), types.ProvisionTokenSpecV2{
		JoinMethod: types.JoinMethodKubernetes,
		Roles:      []types.SystemRole{types.RoleNode},
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Type: types.KubernetesJoinTypeInCluster,
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{
					ServiceAccount:          "namespace1:service-account1",
					ServiceAccountName:      "doesnt-match",
					ServiceAccountNamespace: "namespace1",
				},
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, auth.CreateToken(ctx, implicitInClusterPT))
	require.NoError(t, auth.CreateToken(ctx, explicitInClusterPT))
	require.NoError(t, auth.CreateToken(ctx, staticJWKSPT))
	require.NoError(t, auth.CreateToken(ctx, oidcPT))
	require.NoError(t, auth.CreateToken(ctx, wildcardInClusterPT))
	require.NoError(t, auth.CreateToken(ctx, wildcardStaticJWKSPT))
	require.NoError(t, auth.CreateToken(ctx, wildcardOIDCPT))
	require.NoError(t, auth.CreateToken(ctx, exactInClusterPT))
	require.NoError(t, auth.CreateToken(ctx, namespaceOnlyInClusterPT))
	require.NoError(t, auth.CreateToken(ctx, combinedInClusterPT))
	require.NoError(t, auth.CreateToken(ctx, combinedStrictInClusterPT))

	for _, pt := range []types.ProvisionToken{implicitInClusterPT, explicitInClusterPT, staticJWKSPT, oidcPT, wildcardInClusterPT, wildcardStaticJWKSPT, wildcardOIDCPT, exactInClusterPT, namespaceOnlyInClusterPT, combinedInClusterPT, combinedStrictInClusterPT} {
		ptv2, ok := pt.(*types.ProvisionTokenV2)
		require.True(t, ok, "expected provision token to be types.ProvisionTokenSpecV2")
		scoped, err := jointest.ScopedTokenFromProvisionTokenSpec(ptv2.Spec, joiningv1.ScopedToken_builder{
			Scope: "/test",
			Metadata: headerv1.Metadata_builder{
				Name: "scoped_" + pt.GetName(),
			}.Build(),
			Spec: joiningv1.ScopedTokenSpec_builder{
				AssignedScope: "/test/one",
				UsageMode:     string(joining.TokenUsageModeUnlimited),
			}.Build(),
		}.Build())
		require.NoError(t, err)
		_, err = auth.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{Token: scoped}.Build())
		require.NoError(t, err)
	}

	// Building a joinRequest builder
	sshPrivateKey, sshPublicKey, err := testauthority.GenerateKeyPair()
	require.NoError(t, err)
	tlsPublicKey, err := authtest.PrivateKeyToPublicKeyTLS(sshPrivateKey)
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
		assertError    require.ErrorAssertionFunc
	}{
		{
			"in_cluster (implicit): success",
			"matching-implicit-in-cluster",
			implicitInClusterPT,
			require.NoError,
		},
		{
			"in_cluster (explicit): success",
			"matching-explicit-in-cluster",
			explicitInClusterPT,
			require.NoError,
		},
		{
			"in_cluster: service account rule mismatch",
			"matching-explicit-in-cluster",
			implicitInClusterPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "kubernetes token did not match any allow rules")
			},
		},
		{
			"in_cluster: failed token join (unknown kubeToken)",
			"unknown",
			implicitInClusterPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "invalid token")
			},
		},
		{
			"in_cluster: failed token join (user token)",
			"user-token",
			implicitInClusterPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "kubernetes token did not match any allow rules")
			},
		},
		{
			"static_jwks: success",
			"jwks-matching-service-account",
			staticJWKSPT,
			require.NoError,
		},
		{
			"static_jwks: service account rule mismatch",
			"jwks-mismatched-service-account",
			staticJWKSPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "kubernetes token did not match any allow rules")
			},
		},
		{
			"static_jwks: validation fails",
			"unknown",
			staticJWKSPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "invalid token")
			},
		},
		{
			"oidc: success",
			oidcIDToken,
			oidcPT,
			require.NoError,
		},
		{
			"oidc: allow rule mismatch",
			oidcAllowMismatchToken,
			oidcPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "kubernetes token did not match any allow rules")
			},
		},
		{
			"oidc: invalid audience",
			oidcInvalidAudienceToken,
			oidcPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "audience is not valid")
			},
		},
		{
			"oidc: invalid issuer",
			oidcInvalidIssuerToken,
			oidcPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "issuer does not match")
			},
		},
		{
			"in_cluster wildcard: success",
			"matching-implicit-in-cluster",
			wildcardInClusterPT,
			require.NoError,
		},
		{
			"in_cluster wildcard: service account rule mismatch",
			"matching-explicit-in-cluster",
			wildcardInClusterPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "kubernetes token did not match any allow rules")
			},
		},
		{
			"static_jwks wildcard: success",
			"jwks-matching-service-account",
			wildcardStaticJWKSPT,
			require.NoError,
		},
		{
			"static_jwks wildcard: service account rule mismatch",
			"jwks-mismatched-service-account",
			wildcardStaticJWKSPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "kubernetes token did not match any allow rules")
			},
		},
		{
			"oidc wildcard: success",
			oidcIDToken,
			wildcardOIDCPT,
			require.NoError,
		},
		{
			"oidc wildcard: allow rule mismatch",
			oidcAllowMismatchToken,
			wildcardOIDCPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "kubernetes token did not match any allow rules")
			},
		},
		{
			"in_cluster exact: success",
			"matching-implicit-in-cluster",
			exactInClusterPT,
			require.NoError,
		},
		{
			"in_cluster exact: service account rule mismatch",
			"matching-explicit-in-cluster",
			exactInClusterPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "kubernetes token did not match any allow rules")
			},
		},
		{
			"in_cluster namespace-only: namespace mismatch",
			"matching-explicit-in-cluster",
			namespaceOnlyInClusterPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "kubernetes token did not match any allow rules")
			},
		},
		{
			"in_cluster combined: success",
			"matching-implicit-in-cluster",
			combinedInClusterPT,
			require.NoError,
		},
		{
			"in_cluster combined: glob filter rejects",
			"matching-implicit-in-cluster",
			combinedStrictInClusterPT,
			func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "kubernetes token did not match any allow rules")
			},
		},
	}

	// Doing the real test
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nopClient, err := authServer.NewClient(authtest.TestNop())
			require.NoError(t, err)

			t.Run("legacy", func(t *testing.T) {
				_, err := auth.RegisterUsingToken(ctx, newRequest(tt.provisionToken.GetName(), tt.kubeToken))
				tt.assertError(t, err)
			})

			t.Run("legacy joinclient", func(t *testing.T) {
				_, err := joinclient.LegacyJoin(t.Context(), joinclient.JoinParams{
					Token:      tt.provisionToken.GetName(),
					JoinMethod: types.JoinMethodKubernetes,
					ID: state.IdentityID{
						Role:     types.RoleNode,
						NodeName: "testnode",
						HostUUID: "host-id",
					},
					IDToken:    tt.kubeToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
			})

			t.Run("new joinclient", func(t *testing.T) {
				_, err := joinclient.Join(t.Context(), joinclient.JoinParams{
					Token:      tt.provisionToken.GetName(),
					JoinMethod: types.JoinMethodKubernetes,
					ID: state.IdentityID{
						Role:     types.RoleInstance, // RoleNode is not allowed
						NodeName: "testnode",
					},
					IDToken:    tt.kubeToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
			})

			t.Run("scoped join", func(t *testing.T) {
				_, err := joinclient.Join(t.Context(), joinclient.JoinParams{
					Token:      "scoped_" + tt.provisionToken.GetName(),
					JoinMethod: types.JoinMethodKubernetes,
					ID: state.IdentityID{
						Role:     types.RoleInstance, // RoleNode is not allowed
						NodeName: "testnode",
					},
					IDToken:    tt.kubeToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
			})
		})
	}
}
