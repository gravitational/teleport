/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
)

type AutoUpdateGetter interface {
	// GetClusterAutoUpdateConfig gets the autoupdate configuration from the backend.
	GetClusterAutoUpdateConfig(ctx context.Context) (*autoupdate.ClusterAutoUpdateConfig, error)

	// GetAutoUpdateVersion gets the autoupdate version from the backend.
	GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error)
}

// AutoUpdateService stores the cluster autoupdate service.
type AutoUpdateService interface {
	AutoUpdateGetter

	// UpsertClusterAutoUpdateConfig sets cluster autoupdate configuration.
	UpsertClusterAutoUpdateConfig(ctx context.Context, c *autoupdate.ClusterAutoUpdateConfig) (*autoupdate.ClusterAutoUpdateConfig, error)

	// DeleteClusterAutoUpdateConfig deletes types.ClusterAutoUpdateConfig from the backend.
	DeleteClusterAutoUpdateConfig(ctx context.Context) error

	// UpsertAutoUpdateVersion sets autoupdate version.
	UpsertAutoUpdateVersion(ctx context.Context, c *autoupdate.AutoUpdateVersion) (*autoupdate.AutoUpdateVersion, error)

	// DeleteAutoUpdateVersion deletes types.AutoUpdateVersion from the backend.
	DeleteAutoUpdateVersion(ctx context.Context) error
}
