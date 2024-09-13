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

// AutoUpdateServiceGetter defines only read-only service methods.
type AutoUpdateServiceGetter interface {
	// GetAutoUpdateConfig gets the autoupdate configuration from the backend.
	GetAutoUpdateConfig(ctx context.Context) (*autoupdate.AutoUpdateConfig, error)

	// GetAutoUpdateVersion gets the autoupdate version from the backend.
	GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error)
}

// AutoUpdateService stores the autoupdate service.
type AutoUpdateService interface {
	AutoUpdateServiceGetter

	// CreateAutoUpdateConfig creates an auto update configuration.
	CreateAutoUpdateConfig(ctx context.Context, config *autoupdate.AutoUpdateConfig) (*autoupdate.AutoUpdateConfig, error)

	// UpdateAutoUpdateConfig updates an auto update configuration.
	UpdateAutoUpdateConfig(ctx context.Context, config *autoupdate.AutoUpdateConfig) (*autoupdate.AutoUpdateConfig, error)

	// UpsertAutoUpdateConfig sets an auto update configuration.
	UpsertAutoUpdateConfig(ctx context.Context, c *autoupdate.AutoUpdateConfig) (*autoupdate.AutoUpdateConfig, error)

	// DeleteAutoUpdateConfig deletes the auto update configuration from the backend.
	DeleteAutoUpdateConfig(ctx context.Context) error

	// CreateAutoUpdateVersion creates an auto update version.
	CreateAutoUpdateVersion(ctx context.Context, config *autoupdate.AutoUpdateVersion) (*autoupdate.AutoUpdateVersion, error)

	// UpdateAutoUpdateVersion updates an auto update version.
	UpdateAutoUpdateVersion(ctx context.Context, config *autoupdate.AutoUpdateVersion) (*autoupdate.AutoUpdateVersion, error)

	// UpsertAutoUpdateVersion sets an auto update version.
	UpsertAutoUpdateVersion(ctx context.Context, c *autoupdate.AutoUpdateVersion) (*autoupdate.AutoUpdateVersion, error)

	// DeleteAutoUpdateVersion deletes the auto update version from the backend.
	DeleteAutoUpdateVersion(ctx context.Context) error
}
