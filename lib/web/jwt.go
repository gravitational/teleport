// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package web

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"
)

func (h *Handler) jwks(ctx context.Context, caType types.CertAuthType, includeBlankKeyID bool) (*JWKSResponse, error) {
	clusterName, err := h.GetProxyClient().GetDomainName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch the JWT public keys only.
	ca, err := h.GetProxyClient().GetCertAuthority(ctx, types.CertAuthID{
		Type:       caType,
		DomainName: clusterName,
	}, false /* loadKeys */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pairs := ca.GetTrustedJWTKeyPairs()

	// Create response and allocate space for the keys.
	var resp JWKSResponse
	resp.Keys = make([]jwt.JWK, 0, len(pairs))

	// Loop over and all add public keys in JWK format.
	for _, key := range pairs {
		jwk, err := jwt.MarshalJWK(key.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resp.Keys = append(resp.Keys, jwk)

		// Return an additional copy of the same JWK
		// with KeyID set to the empty string for compatibility.
		if includeBlankKeyID {
			jwk.KeyID = ""
			resp.Keys = append(resp.Keys, jwk)
		}
	}
	return &resp, nil
}
