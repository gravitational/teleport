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

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/cli"
)

func dummyGetSuite(ctx context.Context) (types.SignatureAlgorithmSuite, error) {
	return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1, nil
}

// TestGenerateStaticKeypairAtPath
func TestGenerateStaticKeypairAtPath(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "test.key")

	// Try creating a new key
	firstBuf := bytes.Buffer{}
	require.NoError(t, generateStaticKeypair(t.Context(), &cli.GlobalArgs{}, &cli.KeypairCreateCommand{
		GetSuite:      dummyGetSuite,
		Format:        teleport.JSON,
		StaticKeyPath: dest,
		Overwrite:     false,
		Writer:        &firstBuf,
	}))

	// It should be readable and nonempty
	require.FileExists(t, dest)
	firstFileBytes, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.NotEmpty(t, firstFileBytes)

	firstParseResult := KeypairDocument{}
	require.NoError(t, json.Unmarshal(firstBuf.Bytes(), &firstParseResult))

	decodedFirstKey, err := base64.StdEncoding.DecodeString(firstParseResult.PrivateKey)
	require.NoError(t, err)

	// Defer to bytes.Equal instead of require.Equal(), since otherwise it'll
	// print very unhelpful debugging info if the check fails.
	require.True(t, bytes.Equal(firstFileBytes, decodedFirstKey))

	// Try again, but overwrite
	secondBuf := bytes.Buffer{}
	require.NoError(t, generateStaticKeypair(t.Context(), &cli.GlobalArgs{}, &cli.KeypairCreateCommand{
		GetSuite:      dummyGetSuite,
		Format:        teleport.JSON,
		StaticKeyPath: dest,
		Overwrite:     true,
		Writer:        &secondBuf,
	}))

	secondFileBytes, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.NotEmpty(t, secondFileBytes)

	// Content should be different
	require.False(t, bytes.Equal(firstFileBytes, secondFileBytes))

	// Try once more, but don't overwrite
	thirdBuf := bytes.Buffer{}
	require.NoError(t, generateStaticKeypair(t.Context(), &cli.GlobalArgs{}, &cli.KeypairCreateCommand{
		GetSuite:      dummyGetSuite,
		Format:        teleport.JSON,
		StaticKeyPath: dest,
		Overwrite:     false,
		Writer:        &thirdBuf,
	}))

	thirdFileBytes, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.NotEmpty(t, thirdFileBytes)

	thirdParseResult := KeypairDocument{}
	require.NoError(t, json.Unmarshal(thirdBuf.Bytes(), &thirdParseResult))

	decodedThirdKey, err := base64.StdEncoding.DecodeString(thirdParseResult.PrivateKey)
	require.NoError(t, err)

	// These values should all match the file from the 2nd try
	require.True(t, bytes.Equal(secondFileBytes, thirdFileBytes))
	require.True(t, bytes.Equal(secondFileBytes, decodedThirdKey))
}
