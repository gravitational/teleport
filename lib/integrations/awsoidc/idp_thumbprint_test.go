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

package awsoidc

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib"
)

func TestThumbprint(t *testing.T) {
	ctx := context.Background()

	tlsServer := httptest.NewTLSServer(nil)

	// Proxy starts with self-signed certificates.
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	thumbprint, err := ThumbprintIdP(ctx, tlsServer.URL)
	require.NoError(t, err)

	serverCertificateSHA1 := sha1.Sum(tlsServer.Certificate().Raw)
	expectedThumbprint := hex.EncodeToString(serverCertificateSHA1[:])

	require.Equal(t, expectedThumbprint, thumbprint)
}
