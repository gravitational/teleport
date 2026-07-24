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

package mfa_test

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/lib/mfa"
)

func TestAuthPromptRoundTrip(t *testing.T) {
	t.Parallel()

	prompt := mfa.NewAuthPromptWithMFA()
	require.Len(t, prompt.GetPrompts(), 1)
	require.NotNil(t, prompt.GetPrompts()[0].GetMfa())

	encoded, err := mfa.MarshalAuthPrompt(prompt)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	got, err := mfa.UnmarshalAuthPrompt(encoded)
	require.NoError(t, err)
	require.Len(t, got.GetPrompts(), 1)
	require.NotNil(t, got.GetPrompts()[0].GetMfa())
}

func TestAuthPromptResponseRoundTrip(t *testing.T) {
	t.Parallel()

	const jwtToken = "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.test.signature"

	encoded, err := mfa.MarshalAuthPromptResponse(jwtToken)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	tokens, err := mfa.UnmarshalAuthPromptResponse(encoded)
	require.NoError(t, err)
	require.Equal(t, []string{jwtToken}, tokens)
}

func TestUnmarshalAuthPrompt_InvalidInput(t *testing.T) {
	t.Parallel()

	_, err := mfa.UnmarshalAuthPrompt("not-base64!!!")
	require.Error(t, err)
}

func TestUnmarshalAuthPromptResponse_InvalidInput(t *testing.T) {
	t.Parallel()

	// Valid base64 but invalid proto.
	_, err := mfa.UnmarshalAuthPromptResponse("dGVzdA==")
	require.Error(t, err)
}

func TestUnmarshalAuthPromptResponse_MissingToken(t *testing.T) {
	t.Parallel()

	// Marshal an empty response with no responses field set.
	resp := &mfav2.AuthPromptResponse{}
	data, err := protojson.Marshal(resp)
	require.NoError(t, err)

	encoded := base64.StdEncoding.EncodeToString(data)

	tokens, err := mfa.UnmarshalAuthPromptResponse(encoded)
	require.NoError(t, err)
	require.Empty(t, tokens)
}
