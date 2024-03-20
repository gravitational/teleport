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

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// httpfallback.go holds endpoints that have been converted to gRPC
// but still need http fallback logic in the old client.

// TODO(Joerger): DELETE IN 16.0.0
func (c *Client) RotateCertAuthority(ctx context.Context, req types.RotateRequest) error {
	err := c.APIClient.RotateCertAuthority(ctx, req)
	if trace.IsNotImplemented(err) {
		// Fall back to HTTP implementation.
		_, err := c.PostJSON(ctx, c.Endpoint("authorities", string(req.Type), "rotate"), req)
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}

// TODO(Joerger): DELETE IN 16.0.0
func (c *Client) RotateExternalCertAuthority(ctx context.Context, ca types.CertAuthority) error {
	err := c.APIClient.RotateExternalCertAuthority(ctx, ca)
	if trace.IsNotImplemented(err) {
		// Fall back to HTTP implementation.
		data, err := services.MarshalCertAuthority(ca)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = c.PostJSON(ctx, c.Endpoint("authorities", string(ca.GetType()), "rotate", "external"),
			&rotateExternalCertAuthorityRawReq{CA: data})
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}
