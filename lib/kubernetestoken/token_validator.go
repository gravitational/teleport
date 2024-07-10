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
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	v1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
)

const (
	serviceAccountGroup      = "system:serviceaccounts"
	ServiceAccountNamePrefix = "system:serviceaccount"
	extraDataPodNameField    = "authentication.kubernetes.io/pod-name"
	// Kubernetes should support bound tokens on 1.20 and 1.21,
	// but we can have an apiserver running 1.21 and kubelets running 1.19.
	kubernetesBoundTokenSupportVersion = "1.22.0"
)

type ValidationResult struct {
	// Raw contains the underlying information retrieved during the validation
	// process. This lets us ensure all pertinent information is presented in
	// audit logs.
	Raw any `json:"raw"`
	// Type indicates which form of validation was performed on the token.
	Type types.KubernetesJoinType `json:"type"`
	// Username is the Kubernetes username extracted from the identity.
	// This will be prepended with `system:serviceaccount:` for service
	// accounts.
	Username string `json:"username"`
}

// JoinAuditAttributes returns a series of attributes that can be inserted into
// audit events related to a specific join.
func (c *ValidationResult) JoinAuditAttributes() (map[string]interface{}, error) {
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

// Validate uses the Kubernetes TokenReview API to validate a token and return its UserInfo
func (v *TokenReviewValidator) Validate(ctx context.Context, token string) (*ValidationResult, error) {
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
		return nil, trace.AccessDenied("kubernetes failed to validate token: %s", reviewResult.Status.Error)
	}

	// Check the Username is a service account.
	// A user token would not match rules anyway, but we can produce a more relevant error message here.
	if !strings.HasPrefix(reviewResult.Status.User.Username, ServiceAccountNamePrefix) {
		return nil, trace.BadParameter("token user is not a service account: %s", reviewResult.Status.User.Username)
	}

	if !slices.Contains(reviewResult.Status.User.Groups, serviceAccountGroup) {
		return nil, trace.BadParameter("token user '%s' does not belong to the '%s' group", reviewResult.Status.User.Username, serviceAccountGroup)
	}

	// Legacy tokens are long-lived and not bound to pods. We should not accept them if the cluster supports
	// bound tokens. Bound token support is GA since 1.20 and volume projection is beta since 1.21.
	// We can expect any 1.21+ cluster to use bound tokens.
	kubeVersion, err := client.Discovery().ServerVersion()
	if err != nil {
		return nil, trace.WrapWithMessage(err, "error during the kubernetes version check")
	}

	boundTokenSupport, err := kubernetesSupportsBoundTokens(kubeVersion.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We know if the token is bound to a pod if its name is in the Extra userInfo.
	// If the token is not bound while Kubernetes supports bound tokens we abort.
	if _, ok := reviewResult.Status.User.Extra[extraDataPodNameField]; !ok && boundTokenSupport {
		return nil, trace.BadParameter(
			"legacy SA tokens are not accepted as kubernetes version %s supports bound tokens",
			kubeVersion.String(),
		)
	}

	return &ValidationResult{
		Raw:      reviewResult.Status,
		Type:     types.KubernetesJoinTypeInCluster,
		Username: reviewResult.Status.User.Username,
	}, nil
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

// PodSubClaim are the Pod-specific claims we expect to find on a Kubernetes Service Account JWT.
type PodSubClaim struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

// ServiceAccountSubClaim are the Service Account-specific claims we expect to find on a Kubernetes Service Account JWT.
type ServiceAccountSubClaim struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

// KubernetesSubClaim are the Kubernetes-specific claims (under kubernetes.io)
// we expect to find on a Kubernetes Service Account JWT.
type KubernetesSubClaim struct {
	Namespace      string                  `json:"namespace"`
	ServiceAccount *ServiceAccountSubClaim `json:"serviceaccount"`
	Pod            *PodSubClaim            `json:"pod"`
}

// ServiceAccountClaims are the claims we expect to find on a Kubernetes Service Account JWT.
type ServiceAccountClaims struct {
	josejwt.Claims
	Kubernetes *KubernetesSubClaim `json:"kubernetes.io"`
}

// ValidateTokenWithJWKS validates a Kubernetes Service Account JWT using a
// configured JWKS.
func ValidateTokenWithJWKS(
	now time.Time,
	jwksData []byte,
	clusterName string,
	token string,
) (*ValidationResult, error) {
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
		// Anything related to matching the token against ProvisionToken
		// allow rules is left to the discretion of `lib/auth`.
		Audience: []string{
			clusterName,
		},
		Time: now,
	}, leeway)
	if err != nil {
		return nil, trace.Wrap(err, "validating jwt claims")
	}

	// Ensure this is a pod-bound service account token
	if claims.Kubernetes == nil || claims.Kubernetes.Pod == nil || claims.Kubernetes.Pod.Name == "" {
		return nil, trace.BadParameter("static_jwks joining requires the use of projected pod bound service account token")
	}

	// Ensure the token has a TTL, and that this TTL is low. This ensures that
	// customers have correctly and securely configured the token and avoids
	// bad practice becoming common.
	// We recommend a configuration of 10 minutes (the kubernetes minimum), but
	// allow up to a 30 minute TTL here.
	if claims.Expiry == nil {
		return nil, trace.BadParameter("static_jwks joining requires the use of a service account token with `exp`")
	}
	if claims.IssuedAt == nil {
		return nil, trace.BadParameter("static_jwks joining requires the use of a service account token with `iat`")
	}
	maxAllowedTTL := time.Minute * 30
	if claims.Expiry.Time().Sub(claims.IssuedAt.Time()) > maxAllowedTTL {
		return nil, trace.BadParameter("static_jwks joining requires the use of a service account token with a TTL of less than %s", maxAllowedTTL)
	}

	return &ValidationResult{
		Raw:      claims,
		Type:     types.KubernetesJoinTypeStaticJWKS,
		Username: claims.Subject,
	}, nil
}
