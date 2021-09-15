/*
Copyright 2021 Gravitational, Inc.

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

package client

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/stretchr/testify/require"
)

func TestHostCredentialsFallback(t *testing.T) {
	ctx := context.Background()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI != "/v1/webapi/host/credentials" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(proto.Certs{})
	})
	httpSvr := httptest.NewServer(handler)
	defer httpSvr.Close()

	t.Run("Allowed on insecure & loopback", func(t *testing.T) {
		_, err := HostCredentials(ctx, httpSvr.Listener.Addr().String(), true, auth.RegisterUsingTokenRequest{})
		require.NoError(t, err)
	})

	t.Run("Denied on secure", func(t *testing.T) {
		_, err := HostCredentials(ctx, httpSvr.Listener.Addr().String(), false, auth.RegisterUsingTokenRequest{})
		require.Error(t, err)
	})

	t.Run("Denied on non-loopback", func(t *testing.T) {
		nonLoopbackSvr := httptest.NewUnstartedServer(handler)

		// replace the test-supplied loopback listener with the first available
		// non-loopback address
		nonLoopbackSvr.Listener.Close()
		l, err := net.Listen("tcp", "0.0.0.0:0")
		require.NoError(t, err)
		nonLoopbackSvr.Listener = l
		nonLoopbackSvr.Start()
		defer nonLoopbackSvr.Close()

		_, err = HostCredentials(ctx, nonLoopbackSvr.Listener.Addr().String(), false, auth.RegisterUsingTokenRequest{})
		require.Error(t, err)
	})
}
