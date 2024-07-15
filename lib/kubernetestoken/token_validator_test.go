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

package kubernetestoken

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	ctest "k8s.io/client-go/testing"

	"github.com/gravitational/teleport/api/types"
)

var userGroups = []string{"system:serviceaccounts", "system:serviceaccounts:namespace", "system:authenticated"}

var boundTokenKubernetesVersion = version.Info{
	Major:      "1",
	Minor:      "23+",
	GitVersion: "v1.23.13-eks-fb459a0",
}

var legacyTokenKubernetesVersion = version.Info{
	Major:      "1",
	Minor:      "19",
	GitVersion: "v1.19.7",
}

// tokenReviewMock creates a testing.ReactionFunc validating the tokenReview request and answering it
func tokenReviewMock(t *testing.T, reviewResult *v1.TokenReview) func(ctest.Action) (bool, runtime.Object, error) {
	return func(action ctest.Action) (bool, runtime.Object, error) {
		createAction, ok := action.(ctest.CreateAction)
		require.True(t, ok)
		obj := createAction.GetObject()
		reviewRequest, ok := obj.(*v1.TokenReview)
		require.True(t, ok)

		require.Equal(t, reviewResult.Spec.Token, reviewRequest.Spec.Token)
		return true, reviewResult, nil
	}
}

// newFakeClientset builds a fake clientSet reporting a specific Kubernetes version
// This is used to test version-specific behaviors.
func newFakeClientset(version *version.Info) *fakeClientSet {
	cs := fakeClientSet{}
	cs.discovery = fakediscovery.FakeDiscovery{
		Fake:               &cs.Fake,
		FakedServerVersion: version,
	}
	return &cs
}

type fakeClientSet struct {
	fake.Clientset
	discovery fakediscovery.FakeDiscovery
}

// Discovery overrides the default fake.Clientset Discovery method and returns our custom discovery mock instead
func (c *fakeClientSet) Discovery() discovery.DiscoveryInterface {
	return &c.discovery
}

func TestIDTokenValidator_Validate(t *testing.T) {
	tests := []struct {
		token         string
		review        *v1.TokenReview
		kubeVersion   *version.Info
		wantResult    *ValidationResult
		expectedError error
	}{
		{
			token: "valid",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: "valid",
				},
				Status: v1.TokenReviewStatus{
					Authenticated: true,
					User: v1.UserInfo{
						Username: "system:serviceaccount:namespace:my-service-account",
						UID:      "sa-uuid",
						Groups:   userGroups,
						Extra: map[string]v1.ExtraValue{
							"authentication.kubernetes.io/pod-name": {"podA"},
							"authentication.kubernetes.io/pod-uid":  {"podA-uuid"},
						},
					},
				},
			},
			wantResult: &ValidationResult{
				Type:     types.KubernetesJoinTypeInCluster,
				Username: "system:serviceaccount:namespace:my-service-account",
				// Raw will be filled in during test run to value of review
			},
			kubeVersion:   &boundTokenKubernetesVersion,
			expectedError: nil,
		},
		{
			token: "valid-not-bound",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: "valid-not-bound",
				},
				Status: v1.TokenReviewStatus{
					Authenticated: true,
					User: v1.UserInfo{
						Username: "system:serviceaccount:namespace:my-service-account",
						UID:      "sa-uuid",
						Groups:   userGroups,
						Extra:    nil,
					},
				},
			},
			wantResult: &ValidationResult{
				Type:     types.KubernetesJoinTypeInCluster,
				Username: "system:serviceaccount:namespace:my-service-account",
				// Raw will be filled in during test run to value of review
			},
			kubeVersion:   &legacyTokenKubernetesVersion,
			expectedError: nil,
		},
		{
			token: "valid-not-bound-on-modern-version",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: "valid-not-bound-on-modern-version",
				},
				Status: v1.TokenReviewStatus{
					Authenticated: true,
					User: v1.UserInfo{
						Username: "system:serviceaccount:namespace:my-service-account",
						UID:      "sa-uuid",
						Groups:   userGroups,
						Extra:    nil,
					},
				},
			},
			kubeVersion: &boundTokenKubernetesVersion,
			expectedError: trace.BadParameter(
				"legacy SA tokens are not accepted as kubernetes version %s supports bound tokens",
				boundTokenKubernetesVersion.GitVersion,
			),
		},
		{
			token: "valid-but-not-serviceaccount",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: "valid-but-not-serviceaccount",
				},
				Status: v1.TokenReviewStatus{
					Authenticated: true,
					User: v1.UserInfo{
						Username: "eve@example.com",
						UID:      "user-uuid",
						Groups:   []string{"system:authenticated", "some-other-group"},
						Extra:    nil,
					},
				},
			},
			kubeVersion:   &boundTokenKubernetesVersion,
			expectedError: trace.BadParameter("token user is not a service account: eve@example.com"),
		},
		{
			token: "valid-but-not-serviceaccount-group",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: "valid-but-not-serviceaccount-group",
				},
				Status: v1.TokenReviewStatus{
					Authenticated: true,
					User: v1.UserInfo{
						Username: "system:serviceaccount:namespace:my-service-account",
						UID:      "user-uuid",
						Groups:   []string{"system:authenticated", "some-other-group"},
						Extra:    nil,
					},
				},
			},
			kubeVersion:   &boundTokenKubernetesVersion,
			expectedError: trace.BadParameter("token user 'system:serviceaccount:namespace:my-service-account' does not belong to the 'system:serviceaccounts' group"),
		},
		{
			token: "invalid-expired",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: "invalid-expired",
				},
				Status: v1.TokenReviewStatus{
					Authenticated: false,
					Error:         "[invalid bearer token, Token has been invalidated, unknown]",
				},
			},
			kubeVersion:   &boundTokenKubernetesVersion,
			expectedError: trace.AccessDenied("kubernetes failed to validate token: [invalid bearer token, Token has been invalidated, unknown]"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.token, func(t *testing.T) {
			// Fill value of raw to avoid duplication in test table
			if tt.wantResult != nil {
				tt.wantResult.Raw = tt.review.Status
			}

			client := newFakeClientset(tt.kubeVersion)
			client.AddReactor("create", "tokenreviews", tokenReviewMock(t, tt.review))
			v := TokenReviewValidator{
				client: client,
			}
			result, err := v.Validate(context.Background(), tt.token)
			if tt.expectedError != nil {
				require.ErrorIs(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantResult, result)
		})
	}
}

func Test_kubernetesSupportsBoundTokens(t *testing.T) {
	tests := []struct {
		name              string
		gitVersion        string
		supportBoundToken bool
		expectErr         assert.ErrorAssertionFunc
	}{
		{
			name:              "No token support",
			gitVersion:        legacyTokenKubernetesVersion.String(),
			supportBoundToken: false,
			expectErr:         assert.NoError,
		},
		{
			name:              "Token support",
			gitVersion:        boundTokenKubernetesVersion.String(),
			supportBoundToken: true,
			expectErr:         assert.NoError,
		},
		{
			name:              "Invalid version",
			gitVersion:        "v123",
			supportBoundToken: false,
			expectErr:         assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := kubernetesSupportsBoundTokens(tt.gitVersion)
			tt.expectErr(t, err)
			require.Equal(t, tt.supportBoundToken, result)
		})
	}
}

func testSigner(t *testing.T) ([]byte, jose.Signer) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).
			WithType("JWT").
			WithHeader("kid", "foo"),
	)
	require.NoError(t, err)

	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{
			Key:       key.Public(),
			Use:       "sig",
			Algorithm: string(jose.RS256),
			KeyID:     "foo",
		},
	}}
	jwksData, err := json.Marshal(jwks)
	require.NoError(t, err)
	return jwksData, signer
}

func TestValidateTokenWithJWKS(t *testing.T) {
	jwks, signer := testSigner(t)
	_, wrongSigner := testSigner(t)

	now := time.Now()
	clusterName := "example.teleport.sh"
	validKubeSubclaim := &KubernetesSubClaim{
		ServiceAccount: &ServiceAccountSubClaim{
			Name: "my-service-account",
			UID:  "8b77ea6d-3144-4203-9a8b-36eb5ad65596",
		},
		Pod: &PodSubClaim{
			Name: "my-pod-797959fdf-wptbj",
			UID:  "413b22ca-4833-48d9-b6db-76219d583173",
		},
		Namespace: "default",
	}

	tests := []struct {
		name   string
		signer jose.Signer
		claims ServiceAccountClaims

		wantResult *ValidationResult
		wantErr    string
	}{
		{
			name:   "valid",
			signer: signer,
			claims: ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{clusterName},
					IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					Expiry:    jwt.NewNumericDate(now.Add(10 * time.Minute)),
				},
				Kubernetes: validKubeSubclaim,
			},
			wantResult: &ValidationResult{
				Type:     types.KubernetesJoinTypeStaticJWKS,
				Username: "system:serviceaccount:default:my-service-account",
			},
		},
		{
			name:   "missing bound pod claim",
			signer: signer,
			claims: ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{clusterName},
					IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					Expiry:    jwt.NewNumericDate(now.Add(10 * time.Minute)),
				},
				Kubernetes: &KubernetesSubClaim{
					ServiceAccount: &ServiceAccountSubClaim{
						Name: "my-service-account",
						UID:  "8b77ea6d-3144-4203-9a8b-36eb5ad65596",
					},
					Namespace: "default",
				},
			},
			wantErr: "static_jwks joining requires the use of projected pod bound service account token",
		},
		{
			name:   "signed by unknown key",
			signer: wrongSigner,
			claims: ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{clusterName},
					IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					Expiry:    jwt.NewNumericDate(now.Add(10 * time.Minute)),
				},
				Kubernetes: validKubeSubclaim,
			},
			wantErr: "error in cryptographic primitive",
		},
		{
			name:   "wrong audience",
			signer: signer,
			claims: ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{"wrong.audience"},
					IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					Expiry:    jwt.NewNumericDate(now.Add(10 * time.Minute)),
				},
				Kubernetes: validKubeSubclaim,
			},
			wantErr: "invalid audience claim",
		},
		{
			name:   "no expiry",
			signer: signer,
			claims: ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{clusterName},
					IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
				},
				Kubernetes: validKubeSubclaim,
			},
			wantErr: "static_jwks joining requires the use of a service account token with `exp`",
		},
		{
			name:   "no issued at",
			signer: signer,
			claims: ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{clusterName},
					NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					Expiry:    jwt.NewNumericDate(now.Add(10 * time.Minute)),
				},
				Kubernetes: validKubeSubclaim,
			},
			wantErr: "static_jwks joining requires the use of a service account token with `iat`",
		},
		{
			name:   "too long ttl",
			signer: signer,
			claims: ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{clusterName},
					IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
					Expiry:    jwt.NewNumericDate(now.Add(10 * time.Hour)),
				},
				Kubernetes: validKubeSubclaim,
			},
			wantResult: &ValidationResult{
				Type:     types.KubernetesJoinTypeStaticJWKS,
				Username: "system:serviceaccount:default:my-service-account",
			},
			wantErr: "static_jwks joining requires the use of a service account token with a TTL of less than 30m0s",
		},
		{
			name:   "expired",
			signer: signer,
			claims: ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{clusterName},
					IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Minute)),
					NotBefore: jwt.NewNumericDate(now.Add(-2 * time.Minute)),
					Expiry:    jwt.NewNumericDate(now.Add(-1 * time.Minute)),
				},
				Kubernetes: validKubeSubclaim,
			},
			wantErr: "token is expired",
		},
		{
			name:   "not yet valid",
			signer: signer,
			claims: ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{clusterName},
					IssuedAt:  jwt.NewNumericDate(now.Add(2 * time.Minute)),
					NotBefore: jwt.NewNumericDate(now.Add(2 * time.Minute)),
					Expiry:    jwt.NewNumericDate(now.Add(4 * time.Minute)),
				},
				Kubernetes: validKubeSubclaim,
			},
			wantErr: "token not valid yet",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fill value of raw to avoid duplication in test table
			if tt.wantResult != nil {
				tt.wantResult.Raw = tt.claims
			}

			token, err := jwt.Signed(tt.signer).Claims(tt.claims).CompactSerialize()
			require.NoError(t, err)

			result, err := ValidateTokenWithJWKS(
				now, jwks, clusterName, token,
			)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantResult, result)
		})
	}
}
