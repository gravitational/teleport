/*
Copyright 2024 Gravitational, Inc.

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

package accessgraph

import (
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

const (
	// AuthorizedKeyDefaultKeyTTL is the default TTL for an authorized key.
	AuthorizedKeyDefaultKeyTTL = 8 * time.Hour
)

// NewAuthorizedKey creates a new SSH authorized key resource.
func NewAuthorizedKey(spec *accessgraphv1pb.AuthorizedKeySpec) (*accessgraphv1pb.AuthorizedKey, error) {
	name := authKeyHashNameKey(spec)
	authKey := &accessgraphv1pb.AuthorizedKey{
		Kind:    types.KindAccessGraphSecretAuthorizedKey,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
			Expires: timestamppb.New(
				time.Now().Add(AuthorizedKeyDefaultKeyTTL),
			),
		},
		Spec: spec,
	}
	if err := ValidateAuthorizedKey(authKey); err != nil {
		return nil, trace.Wrap(err)
	}

	return authKey, nil
}

// ValidateAuthorizedKey checks that required parameters are set
// for the specified AuthorizedKey
func ValidateAuthorizedKey(k *accessgraphv1pb.AuthorizedKey) error {
	if k == nil {
		return trace.BadParameter("AuthorizedKey is nil")
	}
	if k.Metadata == nil {
		return trace.BadParameter("Metadata is nil")
	}
	if k.Spec == nil {
		return trace.BadParameter("Spec is nil")
	}

	if k.Spec.HostId == "" {
		return trace.BadParameter("HostId is unset")
	}
	if k.Spec.HostUser == "" {
		return trace.BadParameter("HostUser is unset")
	}
	if k.Spec.KeyFingerprint == "" {
		return trace.BadParameter("KeyFingerprint is unset")
	}

	if k.Spec.KeyType == "" {
		return trace.BadParameter("KeyType is unset")
	}

	if k.Metadata.Name == "" {
		return trace.BadParameter("Name is unset")
	}
	if k.Metadata.Name != authKeyHashNameKey(k.Spec) {
		return trace.BadParameter("Name must be derived from the key fields")
	}

	return nil
}

func authKeyHashNameKey(k *accessgraphv1pb.AuthorizedKeySpec) string {
	return hashComp(k.HostId, k.HostUser, k.KeyFingerprint)
}
