// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package genericoidc

import (
	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"google.golang.org/protobuf/types/known/structpb"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// IDTokenClaims contains all parsed claims for an OIDC token. It just wraps the
// upstream IDTokenClaims, which already contains duplicates of all JWT values
// in its `Claims` field.
type IDTokenClaims oidc.IDTokenClaims

func (c *IDTokenClaims) JoinAttrs() (*workloadidentityv1pb.JoinAttrsGenericOIDC, error) {
	claims, err := structpb.NewStruct(c.Claims)
	if err != nil {
		return nil, trace.Wrap(err, "converting claims to a protobuf struct")
	}

	return workloadidentityv1pb.JoinAttrsGenericOIDC_builder{
		Claims: claims,
	}.Build(), nil
}

// MarshalJSON delegates our wrapper type's marshal/unmarshal to the upstream
// IDTokenClaims, which has special handling for custom claims.
func (c *IDTokenClaims) MarshalJSON() ([]byte, error) {
	return (*oidc.IDTokenClaims)(c).MarshalJSON()
}

// UnmarshalJSON delegates our wrapper type's marshal/unmarshal to the upstream
// IDTokenClaims, which has special handling for custom claims.
func (c *IDTokenClaims) UnmarshalJSON(data []byte) error {
	return (*oidc.IDTokenClaims)(c).UnmarshalJSON(data)
}
