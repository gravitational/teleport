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

package oauth

import (
	"context"

	storage "github.com/gravitational/teleport/integrations/access/common/auth/storage"
)

// Authorizer is the composite interface of Exchanger and Refresher.
type Authorizer interface {
	Exchanger
	Refresher
}

// Exchanger implements the OAuth2 authorization code exchange operation.
type Exchanger interface {
	Exchange(ctx context.Context, authorizationCode string, redirectURI string) (*storage.Credentials, error)
}

// Refresher implements the OAuth2 bearer token refresh operation.
type Refresher interface {
	Refresh(ctx context.Context, refreshToken string) (*storage.Credentials, error)
}
