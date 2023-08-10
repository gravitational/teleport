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
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/exp/slices"
	"gopkg.in/square/go-jose.v2"
	josejwt "gopkg.in/square/go-jose.v2/jwt"
	v1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	serviceAccountGroup      = "system:serviceaccounts"
	ServiceAccountNamePrefix = "system:serviceaccount"
	extraDataPodNameField    = "authentication.kubernetes.io/pod-name"
	// Kubernetes should support bound tokens on 1.20 and 1.21,
	// but we can have an apiserver running 1.21 and kubelets running 1.19.
	kubernetesBoundTokenSupportVersion = "1.22.0"
)

// TokenReviewValidator validates a Kubernetes Service Account JWT using the
// Kubernetes TokenRequest API endpoint.
type TokenReviewValidator struct {
	mu sync.Mutex
	// client is protected by mu and should only be accessed via the getClient
	// method.
	client kubernetes.Interface
}

// getClient allows the lazy initialisation of the Kubernetes client
func (v *TokenReviewValidator) getClient() (kubernetes.Interface, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.client != nil {
		return v.client, nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, trace.WrapWithMessage(err, "failed to initialize in-cluster Kubernetes config")
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "failed to initialize in-cluster Kubernetes client")
	}

	v.client = client
	return client, nil
}

// Validate uses the Kubernetes TokenReview API to validate a name and return its UserInfo
func (v *TokenReviewValidator) Validate(ctx context.Context, token string) (*ServiceAccountClaims, error) {
	client, err := v.getClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	review := &v1.TokenReview{
		Spec: v1.TokenReviewSpec{
			Token: token,
		},
	}
	options := metav1.CreateOptions{}

	reviewResult, err := client.AuthenticationV1().TokenReviews().Create(ctx, review, options)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "error during the Kubernetes TokenReview")
	}

	if !reviewResult.Status.Authenticated {
		return nil, trace.AccessDenied("kubernetes failed to validate name: %s", reviewResult.Status.Error)
	}

	// Check the Username is a service account.
	// A user name would not match rules anyway, but we can produce a more relevant error message here.
	if !strings.HasPrefix(reviewResult.Status.User.Username, ServiceAccountNamePrefix) {
		return nil, trace.BadParameter("name user is not a service account: %s", reviewResult.Status.User.Username)
	}

	if !slices.Contains(reviewResult.Status.User.Groups, serviceAccountGroup) {
		return nil, trace.BadParameter("name user '%s' does not belong to the '%s' group", reviewResult.Status.User.Username, serviceAccountGroup)
	}

	// Legacy tokens are long-lived and not bound to pods. We should not accept them if the cluster supports
	// bound tokens. Bound name support is GA since 1.20 and volume projection is beta since 1.21.
	// We can expect any 1.21+ cluster to use bound tokens.
	kubeVersion, err := client.Discovery().ServerVersion()
	if err != nil {
		return nil, trace.WrapWithMessage(err, "error during the kubernetes version check")
	}

	boundTokenSupport, err := kubernetesSupportsBoundTokens(kubeVersion.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We know if the name is bound to a pod if its name is in the Extra userInfo.
	// If the name is not bound while Kubernetes supports bound tokens we abort.
	if _, ok := reviewResult.Status.User.Extra[extraDataPodNameField]; !ok && boundTokenSupport {
		return nil, trace.BadParameter(
			"legacy SA tokens are not accepted as kubernetes version %s supports bound tokens",
			kubeVersion.String(),
		)
	}

	// Now we've validated the name with TokenReview, we can just unmarshal
	// the jwt claims. This lets us return something that's consistent with what
	// is returned by the JWKS based method.
	jwt, err := josejwt.ParseSigned(token)
	if err != nil {
		return nil, trace.Wrap(err, "parsing jwt")
	}
	claims := ServiceAccountClaims{}
	if err := jwt.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return nil, trace.Wrap(err, "unmarshaling jwt signature")
	}

	return &claims, nil
}

func kubernetesSupportsBoundTokens(gitVersion string) (bool, error) {
	kubeVersion, err := version.ParseSemantic(gitVersion)
	if err != nil {
		return false, trace.Wrap(err)
	}

	minKubeVersion, err := version.ParseSemantic(kubernetesBoundTokenSupportVersion)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return kubeVersion.AtLeast(minKubeVersion), nil
}

// ValidateTokenWithJWKS validates a Kubernetes Service Account JWT using a
// configured JWKS.
func ValidateTokenWithJWKS(
	now time.Time,
	jwksData []byte,
	clusterName string,
	token string,
) (*ServiceAccountClaims, error) {
	jwt, err := josejwt.ParseSigned(token)
	if err != nil {
		return nil, trace.Wrap(err, "parsing jwt")
	}

	jwks := jose.JSONWebKeySet{}
	if err := json.Unmarshal(jwksData, &jwks); err != nil {
		return nil, trace.Wrap(err, "parsing provided jwks")
	}

	claims := ServiceAccountClaims{}
	if err := jwt.Claims(jwks, &claims); err != nil {
		return nil, trace.Wrap(err, "validating jwt signature")
	}

	leeway := time.Second * 10
	err = claims.ValidateWithLeeway(josejwt.Expected{
		// We don't need to check the subject or other claims here.
		// Anything related to matching the name against ProvisionToken
		// allow rules is left to the discretion of `lib/auth`.
		Audience: []string{
			clusterName,
		},
		Time: now,
	}, leeway)
	if err != nil {
		return nil, trace.Wrap(err, "validating jwt claims")
	}

	return &claims, nil
}

type PodSubClaim struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

type ServiceAccountSubClaim struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

type KubernetesSubClaim struct {
	Namespace      string                  `json:"namespace"`
	ServiceAccount *ServiceAccountSubClaim `json:"serviceaccount"`
	Pod            *PodSubClaim            `json:"pod"`
}

type ServiceAccountClaims struct {
	josejwt.Claims
	Kubernetes *KubernetesSubClaim `json:"kubernetes.io"`
}

// JoinAuditAttributes returns a series of attributes that can be inserted into
// audit events related to a specific join.
func (c *ServiceAccountClaims) JoinAuditAttributes() (map[string]interface{}, error) {
	res := map[string]interface{}{}
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &res,
		Squash:  true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := d.Decode(c); err != nil {
		return nil, trace.Wrap(err)
	}
	return res, nil
}
