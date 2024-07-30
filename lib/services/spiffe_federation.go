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

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
)

// SPIFFEFederation is an interface for the SPIFFEFederation service.
type SPIFFEFederation interface {
	// CreateBotInstance
	CreateSPIFFEFederation(ctx context.Context, spiffeFederation *machineidv1.SPIFFEFederation) (*machineidv1.SPIFFEFederation, error)

	// GetBotInstance
	GetSPIFFEFederation(ctx context.Context, name string) (*machineidv1.SPIFFEFederation, error)

	// ListBotInstances
	ListSPIFFEFederations(ctx context.Context, pageSize int, lastToken string) ([]*machineidv1.SPIFFEFederation, string, error)

	// DeleteBotInstance
	DeleteSPIFFEFederation(ctx context.Context, name string) error
}

// MarshalSPIFFEFederation marshals the SPIFFEFederation object into a JSON byte array.
func MarshalSPIFFEFederation(object *machineidv1.SPIFFEFederation, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalSPIFFEFederation unmarshals the CrownJewel object from a JSON byte array.
func UnmarshalSPIFFEFederation(data []byte, opts ...MarshalOption) (*machineidv1.SPIFFEFederation, error) {
	return UnmarshalProtoResource[*machineidv1.SPIFFEFederation](data, opts...)
}

func ValidateSPIFFEFederation(object *machineidv1.SPIFFEFederation) error {

}
