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

	authinfov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/authinfo/v1"
	"github.com/gravitational/teleport/api/types"
	authinfotype "github.com/gravitational/teleport/api/types/authinfo"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	// authInfoKeyComponent is the name of the backend item for storing the auth server information.
	authInfoKeyComponent = "auth_info"
)

// AuthInfoService is responsible for managing the information about auth server.
type AuthInfoService struct {
	authInfo *generic.ServiceWrapper[*authinfov1.AuthInfo]
}

// NewAuthInfoService returns a new AuthInfoService.
func NewAuthInfoService(b backend.Backend) (*AuthInfoService, error) {
	authInfo, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*authinfov1.AuthInfo]{
			Backend:       b,
			ResourceKind:  types.KindAuthInfo,
			BackendPrefix: backend.NewKey(authInfoKeyComponent),
			MarshalFunc:   services.MarshalProtoResource[*authinfov1.AuthInfo],
			UnmarshalFunc: services.UnmarshalProtoResource[*authinfov1.AuthInfo],
			ValidateFunc:  authinfotype.ValidateAuthInfo,
			NameKeyFunc: func(string) string {
				return types.MetaNameAuthInfo
			},
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AuthInfoService{
		authInfo: authInfo,
	}, nil
}

// GetAuthInfo gets the AuthInfo singleton resource.
func (s *AuthInfoService) GetAuthInfo(ctx context.Context) (*authinfov1.AuthInfo, error) {
	info, err := s.authInfo.GetResource(ctx, types.MetaNameAuthInfo)
	return info, trace.Wrap(err)
}

// CreateAuthInfo creates the AuthInfo singleton resource.
func (s *AuthInfoService) CreateAuthInfo(ctx context.Context, info *authinfov1.AuthInfo) (*authinfov1.AuthInfo, error) {
	info, err := s.authInfo.CreateResource(ctx, info)
	return info, trace.Wrap(err)
}

// UpdateAuthInfo updates the AuthInfo singleton resource.
func (s *AuthInfoService) UpdateAuthInfo(ctx context.Context, info *authinfov1.AuthInfo) (*authinfov1.AuthInfo, error) {
	info, err := s.authInfo.ConditionalUpdateResource(ctx, info)
	return info, trace.Wrap(err)
}

// DeleteAuthInfo deletes the AuthInfo singleton resource.
func (s *AuthInfoService) DeleteAuthInfo(ctx context.Context) error {
	err := s.authInfo.DeleteResource(ctx, types.MetaNameAuthInfo)
	return trace.Wrap(err)
}
