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

package services

import (
	"context"

	userexternalcredentialsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
)

// UserExternalCredentialsService manages per-user credentials for external services.
// This is a backend-only resource with no gRPC service or cache.
type UserExternalCredentialsService interface {
	// GetUserExternalCredentials gets a UserExternalCredentials resource by user and name.
	GetUserExternalCredentials(ctx context.Context, user, name string) (*userexternalcredentialsv1.UserExternalCredentials, error)
	// UpsertUserExternalCredentials creates or updates a UserExternalCredentials resource.
	UpsertUserExternalCredentials(ctx context.Context, creds *userexternalcredentialsv1.UserExternalCredentials) (*userexternalcredentialsv1.UserExternalCredentials, error)
	// DeleteUserExternalCredentials deletes a UserExternalCredentials resource by user and name.
	DeleteUserExternalCredentials(ctx context.Context, user, name string) error
}
