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

// AutoupdateServiceGetter defines only read-only service methods.
type AutoupdateServiceGetter interface {
	// GetAutoupdateConfig gets the autoupdate configuration from the backend.
	GetAutoupdateConfig(ctx context.Context) (*autoupdate.AutoupdateConfig, error)

	// GetAutoupdateVersion gets the autoupdate version from the backend.
	GetAutoupdateVersion(ctx context.Context) (*autoupdate.AutoupdateVersion, error)
}

// AutoupdateService stores the autoupdate service.
type AutoupdateService interface {
	AutoupdateServiceGetter

	// CreateAutoupdateConfig creates a AutoupdateConfig.
	CreateAutoupdateConfig(ctx context.Context, config *autoupdate.AutoupdateConfig) (*autoupdate.AutoupdateConfig, error)

	// UpdateAutoupdateConfig updates a AutoupdateConfig.
	UpdateAutoupdateConfig(ctx context.Context, config *autoupdate.AutoupdateConfig) (*autoupdate.AutoupdateConfig, error)

	// UpsertAutoupdateConfig sets autoupdate configuration.
	UpsertAutoupdateConfig(ctx context.Context, c *autoupdate.AutoupdateConfig) (*autoupdate.AutoupdateConfig, error)

	// DeleteAutoupdateConfig deletes types.AutoupdateConfig from the backend.
	DeleteAutoupdateConfig(ctx context.Context) error

	// CreateAutoupdateVersion creates a AutoupdateVersion.
	CreateAutoupdateVersion(ctx context.Context, config *autoupdate.AutoupdateVersion) (*autoupdate.AutoupdateVersion, error)

	// UpdateAutoupdateVersion updates a AutoupdateVersion.
	UpdateAutoupdateVersion(ctx context.Context, config *autoupdate.AutoupdateVersion) (*autoupdate.AutoupdateVersion, error)

	// UpsertAutoupdateVersion sets autoupdate version.
	UpsertAutoupdateVersion(ctx context.Context, c *autoupdate.AutoupdateVersion) (*autoupdate.AutoupdateVersion, error)

	// DeleteAutoupdateVersion deletes types.AutoupdateVersion from the backend.
	DeleteAutoupdateVersion(ctx context.Context) error
}
