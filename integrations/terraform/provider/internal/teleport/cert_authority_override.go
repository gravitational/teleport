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

package teleport

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewCertAuthorityOverrideClient returns a cert authority override client.
func NewCertAuthorityOverrideClient(c *client.Client) CertAuthorityOverrideClient {
	return CertAuthorityOverrideClient{client: c}
}

// CertAuthorityOverrideClient manages cert authority override resources.
type CertAuthorityOverrideClient struct {
	client *client.Client
}

// Get reads a cert authority override by id.
func (r CertAuthorityOverrideClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*subcav1.CertAuthorityOverride, error) {
	resp, err := r.client.SubCAClient().GetCertAuthorityOverride(ctx, &subcav1.GetCertAuthorityOverrideRequest{
		CaId: &subcav1.CertAuthorityOverrideID{
			CaType: id.Name,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.GetCaOverride(), nil
}

// Create creates a cert authority override.
func (r CertAuthorityOverrideClient) Create(ctx context.Context, override *subcav1.CertAuthorityOverride) error {
	if _, err := r.client.SubCAClient().CreateCertAuthorityOverride(ctx, &subcav1.CreateCertAuthorityOverrideRequest{
		CaOverride: override,
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a cert authority override.
func (r CertAuthorityOverrideClient) Upsert(ctx context.Context, override *subcav1.CertAuthorityOverride) error {
	if _, err := r.client.SubCAClient().UpsertCertAuthorityOverride(ctx, &subcav1.UpsertCertAuthorityOverrideRequest{
		CaOverride:            override,
		ForceImmediateDisable: true,
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a cert authority override by id.
func (r CertAuthorityOverrideClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	_, err := r.client.SubCAClient().DeleteCertAuthorityOverride(ctx, &subcav1.DeleteCertAuthorityOverrideRequest{
		CaId: &subcav1.CertAuthorityOverrideID{
			CaType: id.Name,
		},
		ForceImmediateDelete: true,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
