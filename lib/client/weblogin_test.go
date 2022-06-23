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
	"github.com/gravitational/teleport/api/types"

	"github.com/stretchr/testify/require"
)

func TestPlainHttpFallback(t *testing.T) {
	testCases := []struct {
		desc            string
		path            string
		handler         http.HandlerFunc
		actionUnderTest func(ctx context.Context, addr string, insecure bool) error
	}{
		{
			desc: "HostCredentials",
			path: "/v1/webapi/host/credentials",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.RequestURI != "/v1/webapi/host/credentials" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(proto.Certs{})
			},
			actionUnderTest: func(ctx context.Context, addr string, insecure bool) error {
				_, err := HostCredentials(ctx, addr, insecure, types.RegisterUsingTokenRequest{})
				return err
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			ctx := context.Background()

			t.Run("Allowed on insecure & loopback", func(t *testing.T) {
				httpSvr := httptest.NewServer(testCase.handler)
				defer httpSvr.Close()

				err := testCase.actionUnderTest(ctx, httpSvr.Listener.Addr().String(), true /* insecure */)
				require.NoError(t, err)
			})

			t.Run("Denied on secure", func(t *testing.T) {
				httpSvr := httptest.NewServer(testCase.handler)
				defer httpSvr.Close()

				err := testCase.actionUnderTest(ctx, httpSvr.Listener.Addr().String(), false /* secure */)
				require.Error(t, err)
			})

			t.Run("Denied on non-loopback", func(t *testing.T) {
				nonLoopbackSvr := httptest.NewUnstartedServer(testCase.handler)

				// replace the test-supplied loopback listener with the first available
				// non-loopback address
				nonLoopbackSvr.Listener.Close()
				l, err := net.Listen("tcp", "0.0.0.0:0")
				require.NoError(t, err)
				nonLoopbackSvr.Listener = l
				nonLoopbackSvr.Start()
				defer nonLoopbackSvr.Close()

				err = testCase.actionUnderTest(ctx, nonLoopbackSvr.Listener.Addr().String(), true /* insecure */)
				require.Error(t, err)
			})
		})
	}
}
