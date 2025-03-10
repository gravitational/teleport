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

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/authinfo/v1"
	"github.com/gravitational/teleport/api/types"
	authinfotype "github.com/gravitational/teleport/api/types/authinfo"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// authInfoPrefix is a backend key for storing the auth server information.
	authInfoPrefix = "auth_info"
)

// AuthInfoService is responsible for managing the information about auth server.
type AuthInfoService struct {
	backed backend.Backend
}

// NewAuthInfoService returns a new AuthInfoService.
func NewAuthInfoService(b backend.Backend) (*AuthInfoService, error) {
	return &AuthInfoService{
		backed: b,
	}, nil
}

// GetTeleportVersion reads the last known Teleport version from storage.
func (s *AuthInfoService) GetTeleportVersion(ctx context.Context, serverID string) (*semver.Version, error) {
	item, err := s.backed.Get(ctx, backend.NewKey(authInfoPrefix, serverID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	info, err := services.UnmarshalProtoResource[*authinfo.AuthInfo](item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authinfotype.ValidateAuthInfo(info); err != nil {
		return nil, trace.Wrap(err)
	}
	version, err := semver.NewVersion(info.GetSpec().TeleportVersion)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return version, nil
}

// WriteTeleportVersion writes the last known Teleport version to the storage.
func (s *AuthInfoService) WriteTeleportVersion(ctx context.Context, serverID string, version *semver.Version) error {
	if serverID == "" {
		return trace.BadParameter("missing server ID")
	}
	if version == nil {
		return trace.BadParameter("wrong version parameter")
	}

	info, err := authinfotype.NewAuthInfo(serverID, &authinfo.AuthInfoSpec{
		TeleportVersion: version.String(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := authinfotype.ValidateAuthInfo(info); err != nil {
		return trace.Wrap(err)
	}
	rev, err := types.GetRevision(info)
	if err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalProtoResource[*authinfo.AuthInfo](info)
	if err != nil {
		return trace.Wrap(err)
	}
	expires, err := types.GetExpiry(info)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(authInfoPrefix, info.GetMetadata().GetName()),
		Value:    value,
		Expires:  expires,
		Revision: rev,
	}
	if _, err := s.backed.Put(ctx, item); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
