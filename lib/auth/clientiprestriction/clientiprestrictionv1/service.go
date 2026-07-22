/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package clientiprestrictionv1

import (
	"context"

	"github.com/gravitational/trace"

	clientiprestrictionv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clientiprestriction/v1"
)

// Service implements the gRPC API layer for the singleton ClientIPRestriction resource.
// This OSS implementation returns a Cloud-only error for every RPC; the real
// implementation lives in teleport.e.
type Service struct {
	clientiprestrictionv1pb.UnimplementedClientIPRestrictionServiceServer
}

// NewService returns a new Service.
func NewService() *Service {
	return &Service{}
}

// GetClientIPRestriction returns the ClientIPRestriction singleton.
func (s *Service) GetClientIPRestriction(_ context.Context, _ *clientiprestrictionv1pb.GetClientIPRestrictionRequest) (*clientiprestrictionv1pb.GetClientIPRestrictionResponse, error) {
	return nil, requireCloud()
}

// CreateClientIPRestriction creates a new ClientIPRestriction.
func (s *Service) CreateClientIPRestriction(_ context.Context, _ *clientiprestrictionv1pb.CreateClientIPRestrictionRequest) (*clientiprestrictionv1pb.CreateClientIPRestrictionResponse, error) {
	return nil, requireCloud()
}

// UpdateClientIPRestriction updates an existing ClientIPRestriction.
func (s *Service) UpdateClientIPRestriction(_ context.Context, _ *clientiprestrictionv1pb.UpdateClientIPRestrictionRequest) (*clientiprestrictionv1pb.UpdateClientIPRestrictionResponse, error) {
	return nil, requireCloud()
}

// UpsertClientIPRestriction creates or replaces the ClientIPRestriction singleton.
func (s *Service) UpsertClientIPRestriction(_ context.Context, _ *clientiprestrictionv1pb.UpsertClientIPRestrictionRequest) (*clientiprestrictionv1pb.UpsertClientIPRestrictionResponse, error) {
	return nil, requireCloud()
}

// DeleteClientIPRestriction deletes the ClientIPRestriction singleton.
func (s *Service) DeleteClientIPRestriction(_ context.Context, _ *clientiprestrictionv1pb.DeleteClientIPRestrictionRequest) (*clientiprestrictionv1pb.DeleteClientIPRestrictionResponse, error) {
	return nil, requireCloud()
}

func requireCloud() error {
	return trace.AccessDenied(
		"client_ip_restriction resources are only available for Teleport Cloud users")
}
