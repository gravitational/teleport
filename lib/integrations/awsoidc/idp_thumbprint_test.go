/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
