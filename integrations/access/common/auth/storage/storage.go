/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package storage

import (
	"context"
	"time"
)

// Credentials represents the short-lived OAuth2 credentials.
type Credentials struct {
	// AccessToken is the Bearer token used to access the provider's API
	AccessToken string
	// RefreshToken is used to acquire a new access token.
	RefreshToken string
	// ExpiresAt marks the end of validity period for the access token.
	// The application must use the refresh token to acquire a new access token
	// before this time.
	ExpiresAt time.Time
}

// Store defines the interface for persisting the short-lived OAuth2 credentials.
type Store interface {
	GetCredentials(context.Context) (*Credentials, error)
	PutCredentials(context.Context, *Credentials) error
}
