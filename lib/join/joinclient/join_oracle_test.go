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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/join/internal/messages"
)

type mockClientStream struct {
	received []messages.Request
}

func (s *mockClientStream) Send(msg messages.Request) error {
	s.received = append(s.received, msg)
	return nil
}

func (s *mockClientStream) Recv() (messages.Response, error) {
	return nil, trace.NotImplemented("mockClientStream.Recv()")
}

func (s *mockClientStream) CloseSend() error {
	return trace.NotImplemented("mockClientStream.CloseSend()")
}

func requireBadArg(t require.TestingT, err error, msgAndArgs ...any) {
	require.ErrorAs(t, err, new(*trace.BadParameterError), msgAndArgs...)
}

type mockHTTPDoer struct{}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return nil, trace.NotImplemented("mockHTTPDoer.Do")
}

func TestOracleJoin(t *testing.T) {
	t.Run("JoinParams Validation", func(t *testing.T) {
		testCases := []struct {
			name         string
			mutateParams func(*JoinParams, *messages.ClientParams)
			expect       require.ErrorAssertionFunc
		}{
			{
				name:         "missing OracleIMDSClient",
				mutateParams: func(j *JoinParams, _ *messages.ClientParams) { j.OracleIMDSClient = nil },
				expect:       requireBadArg,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				var stream mockClientStream
				joinParams := JoinParams{
					Token:            "some-token",
					TokenSecret:      "some-secret",
					OracleIMDSClient: &mockHTTPDoer{},
				}
				clientParams := makeClientParams(joinParams, &messages.PublicKeys{})

				testCase.mutateParams(&joinParams, &clientParams)

				_, err := oracleJoin(t.Context(), &stream, joinParams, clientParams)
				testCase.expect(t, err)
				require.Empty(t, stream.received)
			})
		}
	})
}
