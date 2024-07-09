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

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
)

type PageToken string

const (
	EndOfList PageToken = ""
)

type ProvisioningStates interface {
	GetProvisioningState(context.Context, string) (*provisioningv1.PrincipalState, error)
	ListProvisioningStates(context.Context, PageToken) ([]*provisioningv1.PrincipalState, PageToken, error)
	CreateProvisioningState(context.Context, *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error)
	UpdateProvisioningState(context.Context, *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error)
	DeleteProvisioningState(context.Context, string) error
	DeleteAllProvisioningStates(context.Context) error
}

// MarshalProvisioningState marshals the User State object into a JSON byte array.
func MarshalProvisioningState(object *provisioningv1.PrincipalState, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalProvisioningState un-marshals the User State  object from a JSON byte array.
func UnmarshalProvisioningState(data []byte, opts ...MarshalOption) (*provisioningv1.PrincipalState, error) {
	return UnmarshalProtoResource[*provisioningv1.PrincipalState](data, opts...)
}
