/*
Copyright 2023 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/square/go-jose.v2"
	josejwt "gopkg.in/square/go-jose.v2/jwt"
)

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

type ServiceAccountTokenClaims struct {
	josejwt.Claims
	Sub        string              `json:"sub"`
	Issuer     string              `json:"issuer"`
	Kubernetes *KubernetesSubClaim `json:"kubernetes.io"`
}

// JoinAuditAttributes returns a series of attributes that can be inserted into
// audit events related to a specific join.
func (c *ServiceAccountTokenClaims) JoinAuditAttributes() (map[string]interface{}, error) {
	res := map[string]interface{}{}
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &res,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := d.Decode(c); err != nil {
		return nil, trace.Wrap(err)
	}
	return res, nil
}

func ValidateRemoteToken(_ context.Context, now time.Time, jwksData []byte, expectAudience string, token string) (*ServiceAccountTokenClaims, error) {
	jwt, err := josejwt.ParseSigned(token)
	if err != nil {
		return nil, trace.Wrap(err, "parsing jwt")
	}

	jwks := jose.JSONWebKeySet{}
	if err := json.Unmarshal(jwksData, &jwks); err != nil {
		return nil, trace.Wrap(err, "parsing provided jwks")
	}

	claims := ServiceAccountTokenClaims{}
	if err := jwt.Claims(jwks, &claims); err != nil {
		return nil, trace.Wrap(err, "validating jwt signature")
	}

	err = claims.ValidateWithLeeway(josejwt.Expected{
		// We don't need to check the subject or other claims here.
		// Anything related to matching the token against ProvisionToken
		// allow rules is left to the discretion of `lib/auth`.
		Audience: []string{
			expectAudience,
		},
		Time: now,
	}, time.Minute)
	if err != nil {
		return nil, trace.Wrap(err, "validating jwt claims")
	}

	return &claims, nil
}
