/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package mcp

import (
	"encoding/json"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

// Test_sanitizeParamsNameRaw_preserves_number_precision verifies that sanitizing raw MCP requests
// does not round or otherwise rewrite JSON number tokens.
func Test_sanitizeParamsNameRaw_preserves_number_precision(t *testing.T) {
	t.Parallel()

	const preciseFloat = `0.123456789012345678901234567890123456789`
	const bigNumber = `12345678901234567890123456789012345678901234567890`

	request := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test-tool","arguments":{"preciseFloat":` + preciseFloat + `,"bigNumber":` + bigNumber + `}}}`)

	// Verify those values cause errors with naive unmarshaling.
	var m map[string]any
	err := json.Unmarshal(request, &m)
	require.NoError(t, err)

	corrupted, err := json.Marshal(m)
	require.NoError(t, err)
	require.NotContains(t, string(corrupted), preciseFloat)
	require.NotContains(t, string(corrupted), bigNumber)

	// Now verify that sanitization does not corrupt.
	sanitized, err := sanitizeRawRequest(request)
	require.NoError(t, err)

	require.Contains(t, string(sanitized), preciseFloat)
	require.Contains(t, string(sanitized), bigNumber)
}

func Test_sanitizeRawRequest_returns_same_slice_when_unmodified(t *testing.T) {
	t.Parallel()

	request := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)

	sanitized, err := sanitizeRawRequest(request)
	require.NoError(t, err)

	require.Equal(t, request, sanitized)
	require.True(t, unsafe.SliceData(request) == unsafe.SliceData(sanitized))
}
