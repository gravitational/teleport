/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mcputils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockTransportReader struct {
	message string
}

func (m mockTransportReader) ReadMessage(context.Context) (string, error) {
	return m.message, nil
}
func (m mockTransportReader) Type() string {
	return "mock"
}
func (m mockTransportReader) Close() error {
	return nil
}

func TestReadOneResponse(t *testing.T) {
	tests := []struct {
		name          string
		rawMessage    string
		checkError    require.ErrorAssertionFunc
		checkResponse func(*testing.T, *JSONRPCResponse)
	}{
		{
			name:       "bad json",
			rawMessage: "not JSON RPC message",
			checkError: require.Error,
		},
		{
			name:       "notification",
			rawMessage: string(sampleNotificationJSON),
			checkError: require.Error,
		},
		{
			name:       "request",
			rawMessage: string(sampleRequestJSON),
			checkError: require.Error,
		},
		{
			name:       "response",
			rawMessage: string(sampleResponseJSON),
			checkError: require.NoError,
			checkResponse: func(t *testing.T, response *JSONRPCResponse) {
				require.NotNil(t, response)
				_, err := response.GetListToolResult()
				require.NoError(t, err)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := ReadOneResponse(t.Context(), mockTransportReader{test.rawMessage})
			test.checkError(t, err)
			if test.checkResponse != nil {
				test.checkResponse(t, resp)
			}
		})
	}
}
