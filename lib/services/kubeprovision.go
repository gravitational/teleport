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

package services

import (
	"context"

	"github.com/gravitational/trace"

	kubeprovisionclient "github.com/gravitational/teleport/api/client/kubeprovision"
	kubeprovisionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubeprovision/v1"
)

var _ KubeProvisions = (*kubeprovisionclient.Client)(nil)

// KubeProvisions is an interface for the KubeProvision service.
type KubeProvisions interface {
	KubeProvisionsGetter
	// CreateKubeProvision creates a new KubeProvision resource.
	CreateKubeProvision(context.Context, *kubeprovisionv1.KubeProvision) (*kubeprovisionv1.KubeProvision, error)
	// UpdateKubeProvision updates an existing KubeProvision resource.
	UpdateKubeProvision(context.Context, *kubeprovisionv1.KubeProvision) (*kubeprovisionv1.KubeProvision, error)
	// UpsertKubeProvision upserts a KubeProvision resource.
	UpsertKubeProvision(context.Context, *kubeprovisionv1.KubeProvision) (*kubeprovisionv1.KubeProvision, error)
	// DeleteKubeProvision removes the specified KubeProvision resource.
	DeleteKubeProvision(ctx context.Context, name string) error
	// DeleteAllKubeProvisions removes all KubeProvisions.
	DeleteAllKubeProvisions(context.Context) error
}

// KubeProvisionsGetter defines methods for List/Read operations on KubeProvision Resources.
type KubeProvisionsGetter interface {
	// ListKubeProvisions returns a paginated list of all KubeProvision resources.
	ListKubeProvisions(ctx context.Context, pageSize int, nextToken string) ([]*kubeprovisionv1.KubeProvision, string, error)
	// GetKubeProvision returns the specified KubeProvision resources.
	GetKubeProvision(ctx context.Context, name string) (*kubeprovisionv1.KubeProvision, error)
}

// ValidateKubeProvision verifies that required fields for a new KubeProvision are present
func ValidateKubeProvision(b *kubeprovisionv1.KubeProvision) error {
	if b.Spec == nil {
		return trace.BadParameter("spec is required")
	}

	return nil
}

// MarshalKubeProvision marshals the KubeProvision object into a JSON byte array.
func MarshalKubeProvision(object *kubeprovisionv1.KubeProvision, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalKubeProvision unmarshals the KubeProvision object from a JSON byte array.
func UnmarshalKubeProvision(data []byte, opts ...MarshalOption) (*kubeprovisionv1.KubeProvision, error) {
	opts = append(opts, WithDiscardUnknown())
	return UnmarshalProtoResource[*kubeprovisionv1.KubeProvision](data, opts...)
}
