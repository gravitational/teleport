/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package local

import (
	"context"

	"github.com/gravitational/trace"

	backendinfov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/backendinfo/v1"
	"github.com/gravitational/teleport/api/types"
	backendinfotype "github.com/gravitational/teleport/api/types/backendinfo"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	// backendInfoKeyComponent is the name of the backend item for storing the backend information.
	backendInfoKeyComponent = "backend_info"
)

// BackendInfoService is responsible for managing the information about backend.
type BackendInfoService struct {
	backendInfo *generic.ServiceWrapper[*backendinfov1.BackendInfo]
}

// NewBackendInfoService returns a new BackendInfoService.
func NewBackendInfoService(b backend.Backend) (*BackendInfoService, error) {
	backendInfo, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*backendinfov1.BackendInfo]{
			Backend:       b,
			ResourceKind:  types.KindBackendInfo,
			BackendPrefix: backend.NewKey(backendInfoKeyComponent),
			MarshalFunc:   services.MarshalProtoResource[*backendinfov1.BackendInfo],
			UnmarshalFunc: services.UnmarshalProtoResource[*backendinfov1.BackendInfo],
			ValidateFunc:  backendinfotype.ValidateBackendInfo,
			NameKeyFunc: func(string) string {
				return types.MetaNameBackendInfo
			},
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &BackendInfoService{
		backendInfo: backendInfo,
	}, nil
}

// GetBackendInfo gets the BackendInfo singleton resource.
func (s *BackendInfoService) GetBackendInfo(ctx context.Context) (*backendinfov1.BackendInfo, error) {
	info, err := s.backendInfo.GetResource(ctx, types.MetaNameBackendInfo)
	return info, trace.Wrap(err)
}

// CreateBackendInfo creates the BackendInfo singleton resource.
func (s *BackendInfoService) CreateBackendInfo(ctx context.Context, info *backendinfov1.BackendInfo) (*backendinfov1.BackendInfo, error) {
	info, err := s.backendInfo.CreateResource(ctx, info)
	return info, trace.Wrap(err)
}

// UpdateBackendInfo updates the BackendInfo singleton resource.
func (s *BackendInfoService) UpdateBackendInfo(ctx context.Context, info *backendinfov1.BackendInfo) (*backendinfov1.BackendInfo, error) {
	info, err := s.backendInfo.ConditionalUpdateResource(ctx, info)
	return info, trace.Wrap(err)
}

// UpsertBackendInfo creates or updates the BackendInfo singleton resource.
func (s *BackendInfoService) UpsertBackendInfo(ctx context.Context, info *backendinfov1.BackendInfo) (*backendinfov1.BackendInfo, error) {
	info, err := s.backendInfo.UpsertResource(ctx, info)
	return info, trace.Wrap(err)
}

// DeleteBackendInfo deletes the BackendInfo singleton resource.
func (s *BackendInfoService) DeleteBackendInfo(ctx context.Context) error {
	err := s.backendInfo.DeleteResource(ctx, types.MetaNameBackendInfo)
	return trace.Wrap(err)
}
