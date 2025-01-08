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

package web

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestJWKSOktaPublicEndpoint ensures the public endpoint for the Okta API Service App integration
// is available.
func TestJWKSOktaPublicEndpoint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	publicClt := proxy.newClient(t)

	resp, err := publicClt.Get(ctx, publicClt.Endpoint(".well-known/jwks-okta"), nil)
	require.NoError(t, err)

	var gotKeys JWKSResponse
	err = json.Unmarshal(resp.Bytes(), &gotKeys)
	require.NoError(t, err)

	require.Len(t, gotKeys.Keys, 1)
	require.NotEmpty(t, gotKeys.Keys[0].KeyID)
}
