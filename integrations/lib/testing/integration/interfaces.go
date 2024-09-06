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

package integration

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	libauth "github.com/gravitational/teleport/lib/auth"
)

// AuthHelper is the interface one must implement to run the AccessRequestSuite.
// It can be implemented by an OSS Auth server, or an Enterprise auth server
// (in teleport.e).
type AuthHelper interface {
	StartServer(t *testing.T) *client.Client
	ServerAddr() string
	CredentialsForUser(t *testing.T, ctx context.Context, user types.User) client.Credentials
	SignIdentityForUser(t *testing.T, ctx context.Context, user types.User) string
	Auth() *libauth.Server
}

// NewAccessRequestClient returns a new integration.Client.
func NewAccessRequestClient(client *client.Client) *Client {
	return &Client{client}
}
