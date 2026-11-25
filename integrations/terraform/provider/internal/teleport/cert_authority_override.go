// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
