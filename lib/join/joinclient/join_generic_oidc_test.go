// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package joinclient

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/genericoidc"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

func TestJoinWithMethod_GenericOIDCHTTP(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		status  int
		wantErr string
	}{
		{name: "success", status: http.StatusOK},
		{name: "HTTP failure", status: http.StatusInternalServerError, wantErr: "fetching generic_oidc token from an HTTP endpoint"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte("header.payload.signature"))
			}))
			t.Cleanup(server.Close)

			stream := &genericOIDCClientStream{result: &messages.HostResult{}}
			params := JoinParams{
				GenericOIDCParams: GenericOIDCParams{
					HTTPRequest: genericoidc.JWTFromHTTPEndpointParams{
						Request: genericoidc.HTTPRequestParams{URL: server.URL},
					},
				},
			}
			clientParams := makeClientParams(params, &messages.PublicKeys{})
			result, err := joinWithMethod(t.Context(), stream, params, clientParams, string(types.JoinMethodGenericOIDC))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Nil(t, result)
				require.Empty(t, stream.received)
				return
			}
			require.NoError(t, err)
			require.Same(t, stream.result, result)
			require.Equal(t, []messages.Request{&messages.OIDCInit{
				ClientParams: clientParams,
				IDToken:      []byte("header.payload.signature"),
			}}, stream.received)
		})
	}
}

type genericOIDCClientStream struct {
	mockClientStream
	result messages.Response
}

func (s *genericOIDCClientStream) Recv() (messages.Response, error) {
	return s.result, nil
}
