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

	// The Proxy is started using httptest.NewTLSServer, which uses a hard-coded cert
	// located at go/src/net/http/internal/testcert/testcert.go
	// The following value is the sha1 fingerprint of that certificate.
	expectedThumbprint := "15dbd260c7465ecca6de2c0b2181187f66ee0d1a"

	require.Equal(t, expectedThumbprint, thumbprint)
}
