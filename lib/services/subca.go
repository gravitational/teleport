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

package services

import (
	"context"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
)

// SubCAServiceGetter is the read-only SubCAService interface.
//
// See lib/services/local.SubCAService.
type SubCAServiceGetter interface {
	// GetCertAuthorityOverride reads a CA override resource by ID.
	GetCertAuthorityOverride(ctx context.Context, id types.CertAuthorityOverrideID) (*subcav1.CertAuthorityOverride, error)

	// ListCertAuthorityOverrides lists all CA overrides.
	ListCertAuthorityOverrides(ctx context.Context, pageSize int, pageToken string) (_ []*subcav1.CertAuthorityOverride, nextPageToken string, _ error)
}

// SubCAService manages CertAuthorityOverride resources.
//
// See lib/services/local.SubCAService.
type SubCAService interface {
	SubCAServiceGetter

	// CreateCertAuthorityOverride creates a CA override.
	CreateCertAuthorityOverride(ctx context.Context, resource *subcav1.CertAuthorityOverride) (*subcav1.CertAuthorityOverride, error)

	// DeleteCertAuthorityOverride hard-deletes a CA override.
	DeleteCertAuthorityOverride(ctx context.Context, id types.CertAuthorityOverrideID) error

	// UpdateCertAuthorityOverride conditionally updates a CA override.
	UpdateCertAuthorityOverride(ctx context.Context, resource *subcav1.CertAuthorityOverride) (*subcav1.CertAuthorityOverride, error)

	// UpsertCertAuthorityOverride unconditionally creates or updates a CA override.
	UpsertCertAuthorityOverride(ctx context.Context, resource *subcav1.CertAuthorityOverride) (*subcav1.CertAuthorityOverride, error)
}

// MarshalCertAuthorityOverride marshals a CA override resource.
func MarshalCertAuthorityOverride(resource *subcav1.CertAuthorityOverride, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(resource, opts...)
}

// UnmarshalCertAuthorityOverride unmarshals a CA override resource.
func UnmarshalCertAuthorityOverride(data []byte, opts ...MarshalOption) (*subcav1.CertAuthorityOverride, error) {
	return UnmarshalProtoResource[*subcav1.CertAuthorityOverride](data, opts...)
}
