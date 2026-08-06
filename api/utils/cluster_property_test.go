// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/api/constants"
)

func TestProperty_ClusterNameRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := string(rapid.SliceOf(rapid.Byte()).Draw(t, "clusterName"))
		decoded, err := DecodeClusterName(EncodeClusterName(name))
		require.NoError(t, err)
		require.Equal(t, name, decoded)
	})
}

func TestProperty_DecodeClusterName_NeverPanics(t *testing.T) {
	const suffix = "." + constants.APIDomain

	serverNameGen := rapid.OneOf(
		rapid.String(),
		rapid.Map(rapid.String(), func(s string) string { return s + suffix }),
		rapid.StringMatching(`[0-9a-fA-F]{0,65}`+regexp.QuoteMeta(suffix)),
		rapid.Map(rapid.String(), EncodeClusterName),
		rapid.Just(constants.APIDomain),
	)

	rapid.Check(t, func(t *rapid.T) {
		_, _ = DecodeClusterName(serverNameGen.Draw(t, "serverName"))
	})
}

func TestProperty_EncodeClusterName_OutputIsHexPlusSuffix(t *testing.T) {
	const suffix = "." + constants.APIDomain
	hexRegex := regexp.MustCompile(`^[0-9a-f]*$`)

	rapid.Check(t, func(t *rapid.T) {
		name := string(rapid.SliceOf(rapid.Byte()).Draw(t, "clusterName"))
		encoded := EncodeClusterName(name)
		subdomain, ok := strings.CutSuffix(encoded, suffix)
		require.True(t, ok, "expected %q to end with %q", encoded, suffix)
		require.Regexp(t, hexRegex, subdomain)
	})
}

func TestProperty_EncodeClusterName_OutputIsCorrectLength(t *testing.T) {
	const suffix = "." + constants.APIDomain

	rapid.Check(t, func(t *rapid.T) {
		name := string(rapid.SliceOf(rapid.Byte()).Draw(t, "clusterName"))
		encoded := EncodeClusterName(name)
		require.Len(t, encoded, 2*len(name)+len(suffix))
	})
}
