/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package claims

import (
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

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
	jwt.Claims
	Kubernetes *KubernetesSubClaim `json:"kubernetes.io"`
}

// OIDCServiceAccountClaims is a variant of `ServiceAccountClaims` intended for
// use with the OIDC validator rather than plain JWKS.
type OIDCServiceAccountClaims struct {
	oidc.TokenClaims
	Kubernetes *KubernetesSubClaim `json:"kubernetes.io"`
}
