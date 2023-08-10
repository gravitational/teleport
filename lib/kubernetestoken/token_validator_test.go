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

package kubernetestoken

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
	"testing"
	"time"

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
	_, signer := testSigner(t)
	claims := &ServiceAccountClaims{
		Claims: jwt.Claims{
			Subject: "system:serviceaccount:default:my-service-account",
		},
		Kubernetes: &KubernetesSubClaim{
			ServiceAccount: &ServiceAccountSubClaim{
				Name: "my-service-account",
				UID:  "8b77ea6d-3144-4203-9a8b-36eb5ad65596",
			},
			Pod: &PodSubClaim{
				Name: "my-pod-797959fdf-wptbj",
				UID:  "413b22ca-4833-48d9-b6db-76219d583173",
			},
			Namespace: "default",
		},
	}
	token, err := jwt.Signed(signer).Claims(claims).CompactSerialize()
	require.NoError(t, err)

	tests := []struct {
		name          string
		review        *v1.TokenReview
		kubeVersion   *version.Info
		expectedError error
	}{
		{
			name: "valid",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: token,
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
			kubeVersion:   &boundTokenKubernetesVersion,
			expectedError: nil,
		},
		{
			name: "valid-not-bound",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: token,
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
			kubeVersion:   &legacyTokenKubernetesVersion,
			expectedError: nil,
		},
		{
			name: "valid-not-bound-on-modern-version",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: token,
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
			name: "valid-but-not-serviceaccount",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: token,
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
			name: "valid-but-not-serviceaccount-group",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: token,
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
			name: "invalid-expired",
			review: &v1.TokenReview{
				Spec: v1.TokenReviewSpec{
					Token: token,
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
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClientset(tt.kubeVersion)
			client.AddReactor("create", "tokenreviews", tokenReviewMock(t, tt.review))
			v := TokenReviewValidator{
				client: client,
			}
			gotClaims, err := v.Validate(context.Background(), tt.review.Spec.Token)
			if tt.expectedError != nil {
				require.ErrorIs(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, claims, gotClaims)
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
	// match kid of right key
	_, wrongSigner := testSigner(t)

	now := time.Now()
	clusterName := "example.teleport.sh"
	tests := []struct {
		name    string
		signer  jose.Signer
		claims  *ServiceAccountClaims
		wantErr string
	}{
		{
			name:   "valid",
			signer: signer,
			claims: &ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{clusterName},
					IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Second)),
					NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Second)),
					Expiry:    jwt.NewNumericDate(now.Add(10 * time.Second)),
				},
				Kubernetes: &KubernetesSubClaim{
					ServiceAccount: &ServiceAccountSubClaim{
						Name: "my-service-account",
						UID:  "8b77ea6d-3144-4203-9a8b-36eb5ad65596",
					},
					Pod: &PodSubClaim{
						Name: "my-pod-797959fdf-wptbj",
						UID:  "413b22ca-4833-48d9-b6db-76219d583173",
					},
					Namespace: "default",
				},
			},
		},
		{
			name:   "signed by unknown key",
			signer: wrongSigner,
			claims: &ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{clusterName},
					IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Second)),
					NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Second)),
					Expiry:    jwt.NewNumericDate(now.Add(10 * time.Second)),
				},
			},
			wantErr: "error in cryptographic primitive",
		},
		{
			name:   "expired",
			signer: signer,
			claims: &ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{clusterName},
					IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Minute)),
					NotBefore: jwt.NewNumericDate(now.Add(-2 * time.Minute)),
					Expiry:    jwt.NewNumericDate(now.Add(-1 * time.Minute)),
				},
			},
			wantErr: "name is expired",
		},
		{
			name:   "wrong audience",
			signer: signer,
			claims: &ServiceAccountClaims{
				Claims: jwt.Claims{
					Subject:   "system:serviceaccount:default:my-service-account",
					Audience:  jwt.Audience{"wrong.audience"},
					IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Minute)),
					NotBefore: jwt.NewNumericDate(now.Add(-2 * time.Minute)),
					Expiry:    jwt.NewNumericDate(now.Add(-1 * time.Minute)),
				},
			},
			wantErr: "invalid audience claim",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := jwt.Signed(tt.signer).Claims(tt.claims).CompactSerialize()
			require.NoError(t, err)

			claims, err := ValidateTokenWithJWKS(
				now, jwks, clusterName, token,
			)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.claims, claims)
		})
	}
}
