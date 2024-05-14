// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package auth

import (
	"github.com/gravitational/roundtrip"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// MissingNamespaceError indicates that the client failed to
// provide the namespace in the request.
const MissingNamespaceError = authclient.MissingNamespaceError

type APIClient = authclient.Client
type Client = authclient.Client
type ClientI = authclient.ClientI

// NewClient creates a new API client with a connection to a Teleport server.
//
// The client will use the first credentials and the given dialer. If
// no dialer is given, the first address will be used. This address must
// be an auth server address.
//
// NOTE: This client is being deprecated in favor of the gRPC Client in
// teleport/api/client. This Client should only be used internally, or for
// functionality that hasn't been ported to the new client yet.
func NewClient(cfg client.Config, params ...roundtrip.ClientParam) (*Client, error) {
	return authclient.NewClient(cfg, params...)
}
