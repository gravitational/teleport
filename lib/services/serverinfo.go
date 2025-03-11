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

package services

import (
	"context"

	"github.com/coreos/go-semver/semver"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/authinfo/v1"
)

// AuthInfoService stores information about auth server.
type AuthInfoService interface {
	// GetTeleportVersion reads the last known Teleport version from storage.
	GetTeleportVersion(ctx context.Context, serverID string) (*semver.Version, error)
	// WriteTeleportVersion writes the last known Teleport version to the storage.
	WriteTeleportVersion(ctx context.Context, serverID string, version *semver.Version) error
	// GetAuthInfoList returns list of all registered auth servers in cluster.
	GetAuthInfoList(ctx context.Context) ([]*authinfo.AuthInfo, error)
}
