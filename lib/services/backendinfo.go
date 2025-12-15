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

	backendinfov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/backendinfo/v1"
)

// BackendInfoService stores information about the backend.
type BackendInfoService interface {
	// GetBackendInfo gets the BackendInfo singleton resource.
	GetBackendInfo(ctx context.Context) (*backendinfov1.BackendInfo, error)
	// CreateBackendInfo creates the BackendInfo singleton resource.
	CreateBackendInfo(ctx context.Context, info *backendinfov1.BackendInfo) (*backendinfov1.BackendInfo, error)
	// UpdateBackendInfo updates the BackendInfo singleton resource.
	UpdateBackendInfo(ctx context.Context, info *backendinfov1.BackendInfo) (*backendinfov1.BackendInfo, error)
	// UpsertBackendInfo create or update the BackendInfo singleton resource.
	UpsertBackendInfo(ctx context.Context, info *backendinfov1.BackendInfo) (*backendinfov1.BackendInfo, error)
	// DeleteBackendInfo deletes the BackendInfo singleton resource.
	DeleteBackendInfo(ctx context.Context) error
}
