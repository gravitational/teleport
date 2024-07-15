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

package imds

import (
	"context"

	"github.com/gravitational/teleport/api/types"
)

// Client is an interface for fetching information from a cloud
// service's instance metadata.
type Client interface {
	// IsAvailable checks if instance metadata is available.
	IsAvailable(ctx context.Context) bool
	// GetTags gets all of the instance's tags.
	GetTags(ctx context.Context) (map[string]string, error)
	// GetHostname gets the hostname set by the cloud instance that Teleport
	// should use, if any.
	GetHostname(ctx context.Context) (string, error)
	// GetType gets the cloud instance type.
	GetType() types.InstanceMetadataType
	// GetID gets the cloud instance ID.
	GetID(ctx context.Context) (string, error)
}

// DisabledClient is a [Client] that is always disabled. This is faster
// than the default client when not testing instance metadata behavior.
type DisabledClient struct{}

// NewDisabledIMDSClient creates a new DisabledClient.
func NewDisabledIMDSClient() Client {
	return &DisabledClient{}
}

func (d *DisabledClient) IsAvailable(ctx context.Context) bool {
	return false
}

func (d *DisabledClient) GetTags(ctx context.Context) (map[string]string, error) {
	return nil, nil
}

func (d *DisabledClient) GetHostname(ctx context.Context) (string, error) {
	return "", nil
}

func (d *DisabledClient) GetType() types.InstanceMetadataType {
	return types.InstanceMetadataTypeDisabled
}

func (d *DisabledClient) GetID(ctx context.Context) (string, error) {
	return "", nil
}
