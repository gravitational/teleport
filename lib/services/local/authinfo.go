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
	"log/slog"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/authinfo/v1"
	"github.com/gravitational/teleport/api/types"
	authinfotype "github.com/gravitational/teleport/api/types/authinfo"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	// maxWriteRetry is the number of retries for conditional updates of the AuthInfo resource.
	maxWriteRetry = 5
	// authInfoPrefix is a backend key for storing the auth server information.
	authInfoPrefix = "auth_info"
)

// AuthInfoService is responsible for managing the information about auth server.
type AuthInfoService struct {
	logger *slog.Logger

	authInfo *generic.ServiceWrapper[*authinfo.AuthInfo]
}

// NewAuthInfoService returns a new AuthInfoService.
func NewAuthInfoService(b backend.Backend) (*AuthInfoService, error) {
	authInfo, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*authinfo.AuthInfo]{
			Backend:       b,
			ResourceKind:  types.KindAuthInfo,
			BackendPrefix: backend.NewKey(authInfoPrefix),
			MarshalFunc:   services.MarshalProtoResource[*authinfo.AuthInfo],
			UnmarshalFunc: services.UnmarshalProtoResource[*authinfo.AuthInfo],
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
		logger:   slog.With(teleport.ComponentKey, "AuthInfoService"),
	}, nil
}

// GetTeleportVersion reads the last known Teleport version from backend.
func (s *AuthInfoService) GetTeleportVersion(ctx context.Context) (semver.Version, error) {
	info, err := s.authInfo.GetResource(ctx, types.MetaNameAuthInfo)
	if err != nil {
		return semver.Version{}, trace.Wrap(err)
	}

	tVersion, err := version.EnsureSemver(info.GetSpec().TeleportVersion)
	if err != nil {
		return semver.Version{}, trace.Wrap(err)
	}

	return *tVersion, nil
}

// WriteTeleportVersion writes the last known Teleport version to the backend.
func (s *AuthInfoService) WriteTeleportVersion(ctx context.Context, version semver.Version) (err error) {
	var info *authinfo.AuthInfo
	for i := 0; i < maxWriteRetry; i++ {
		info, err = s.authInfo.GetResource(ctx, types.MetaNameAuthInfo)
		if trace.IsNotFound(err) {
			info, err = authinfotype.NewAuthInfo(&authinfo.AuthInfoSpec{TeleportVersion: version.String()})
			if err != nil {
				err = trace.Wrap(err)
				s.logger.WarnContext(ctx, "Failed to create AuthInfo resource",
					"error", err)
				continue
			}
			if _, err := s.authInfo.CreateResource(ctx, info); err != nil {
				s.logger.WarnContext(ctx, "Failed to create AuthInfo resource",
					"error", err)
				continue
			}
			return nil
		} else if err != nil {
			s.logger.WarnContext(ctx, "Failed to receive AuthInfo resource",
				"error", err)
			err = trace.Wrap(err)
			continue
		}
		info.GetSpec().TeleportVersion = version.String()
		if _, err := s.authInfo.ConditionalUpdateResource(ctx, info); err != nil {
			err = trace.Wrap(err)
			s.logger.WarnContext(ctx, "Failed to update AuthInfo resource",
				"version", version, "error", err)
			continue
		}
		return nil
	}
	return
}

// GetAuthInfo gets the AuthInfo singleton resource.
func (s *AuthInfoService) GetAuthInfo(ctx context.Context) (*authinfo.AuthInfo, error) {
	info, err := s.authInfo.GetResource(ctx, types.MetaNameAuthInfo)
	return info, trace.Wrap(err)
}

// CreateAuthInfo creates the AuthInfo singleton resource.
func (s *AuthInfoService) CreateAuthInfo(ctx context.Context, info *authinfo.AuthInfo) (*authinfo.AuthInfo, error) {
	info, err := s.authInfo.CreateResource(ctx, info)
	return info, trace.Wrap(err)
}

// UpdateAuthInfo updates the AuthInfo singleton resource.
func (s *AuthInfoService) UpdateAuthInfo(ctx context.Context, info *authinfo.AuthInfo) (*authinfo.AuthInfo, error) {
	info, err := s.authInfo.ConditionalUpdateResource(ctx, info)
	return info, trace.Wrap(err)
}

// DeleteAuthInfo deletes the AuthInfo singleton resource.
func (s *AuthInfoService) DeleteAuthInfo(ctx context.Context) error {
	err := s.authInfo.DeleteResource(ctx, types.MetaNameAuthInfo)
	return trace.Wrap(err)
}
