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

package fakejoin

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	kubetoken "github.com/gravitational/teleport/lib/kube/token"
)

// KubernetesSigner is a JWT signer that mimicks the Kubernetes one. The signer mock Kubernetes and
// allows us to do Kube joining locally. This is useful in tests as this is currently the easiest
// delegated join method we can use without having to rely on external infrastructure/providers.
type KubernetesSigner struct {
	key    *rsa.PrivateKey
	signer jose.Signer
	jwks   *jose.JSONWebKeySet
	clock  clockwork.Clock
}

const fakeKeyID = "foo"

// NewKubernetesSigner generates a keypair and creates a new KubernetesSigner.
func NewKubernetesSigner(clock clockwork.Clock) (*KubernetesSigner, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, trace.Wrap(err, "generating key")
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).
			WithType("JWT").
			WithHeader("kid", fakeKeyID),
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating signer")
	}
	jwks := &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{
			Key:       key.Public(),
			Use:       "sig",
			Algorithm: string(jose.RS256),
			KeyID:     fakeKeyID,
		},
	}}
	return &KubernetesSigner{
		key:    key,
		signer: signer,
		jwks:   jwks,
		clock:  clock,
	}, nil
}

// GetMarshaledJWKS returns the KuberbetesSigner's JWKS marshaled in a string.
// The JWKS can then be directly passed into the JWKS field of a Kube Provision Token.
// This makes Teleport trust the KubernetesSigner. The signer can then issue tokens
// that ca be used to join Teleport.
func (s *KubernetesSigner) GetMarshaledJWKS() (string, error) {
	jwksData, err := json.Marshal(s.jwks)
	return string(jwksData), err
}

// SignServiceAccountJWT returns a signed JWT valid 30 minutes (1 min in the past, 29 in the future).
// This token has the Teleport cluster name in its audience as required by the Kubernetes JWKS join method.
func (s *KubernetesSigner) SignServiceAccountJWT(pod, namespace, serviceAccount, clusterName string) (string, error) {
	now := s.clock.Now()
	claims := kubetoken.ServiceAccountClaims{
		Claims: jwt.Claims{
			Subject:  fmt.Sprintf("system:serviceaccount:%s:%s", namespace, serviceAccount),
			Audience: jwt.Audience{clusterName},
			// To protect against time skew issues, we sign 1 min in the past.
			IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			// The Kubernetes JWKS join method rejects tokens valid more than 30 minutes.
			Expiry: jwt.NewNumericDate(now.Add(29 * time.Minute)),
		},
		Kubernetes: &kubetoken.KubernetesSubClaim{
			Namespace: namespace,
			ServiceAccount: &kubetoken.ServiceAccountSubClaim{
				Name: serviceAccount,
				UID:  uuid.New().String(),
			},
			Pod: &kubetoken.PodSubClaim{
				Name: pod,
				UID:  uuid.New().String(),
			},
		},
	}
	token, err := jwt.Signed(s.signer).Claims(claims).CompactSerialize()
	return token, trace.Wrap(err)
}
