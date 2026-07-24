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

package scopes

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
)

// TestEncodeDecode tests that encoded scopes are valid backend keys and than
// an encode-decode round-trip preserves with scope.
func TestEncodeDecode(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"",
		"/",
		"/example/basic",
		"/ops/west/vancouver",
		"/some/really/long/scope/with/many/segments",
		"/a/b/c",
		"/a/b/",
		"/a",
	}

	seenEncodings := make(map[string]struct{}, len(testCases))

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			encoded, err := EncodeForKey(tc)
			require.NoError(t, err)

			require.NotEmpty(t, encoded)

			require.NotContains(t, seenEncodings, encoded)
			seenEncodings[encoded] = struct{}{}

			require.True(t, backend.IsKeySafe(backend.NewKey(encoded)))

			decoded, err := DecodeFromKey(encoded)
			require.NoError(t, err)

			require.Equal(t, NormalizeForEquality(tc), decoded)
		})
	}
}

// TestEncodeForKeyExamples runs through some known examples.
func TestEncodeForKeyExamples(t *testing.T) {
	t.Parallel()

	tts := []struct {
		scope string
		enc   string
	}{
		{scope: "", enc: "00"},  // unscoped
		{scope: "/", enc: "04"}, // root
		{scope: "/staging", enc: "04076x31cxmpwsr0"},
		{scope: "/staging/west", enc: "04076x31cxmpwsr001vpawvm00"},
		{scope: "/staging/west/testbed", enc: "04076x31cxmpwsr001vpawvm00078sbkehh6as00"},
		{scope: "/prod", enc: "04070wkfcg00"},
		{scope: "/prod/west", enc: "04070wkfcg000xv5edt00"},
	}

	for _, tt := range tts {
		t.Run(tt.scope, func(t *testing.T) {
			encoded, err := EncodeForKey(tt.scope)
			require.NoError(t, err)
			require.Equal(t, tt.enc, encoded, "encoding of %q mismatch", tt.scope)

			decoded, err := DecodeFromKey(tt.enc)
			require.NoError(t, err)
			require.Equal(t, tt.scope, decoded)
		})
	}
}

// TestEncodeForKeyErrors asserts that EncodeForKeys returns an error in expected cases.
func TestEncodeForKeyErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range []string{
		// empty segments break the ordering/prefix guarantees and are rejected.
		"//",
		"/aa//",
		"/a//b",
		"///",
		// non-printable bytes are rejected by weak validation.
		string([]byte{0, 1, 2, 3}),
	} {
		_, err := EncodeForKey(tc)
		require.Error(t, err, "expected an error encoding %s", tc)
	}
}

// TestEncodeForKeyWeaklyValid asserts that scopes which are weakly valid but not
// strongly valid (e.g. segments with a leading dot or uppercase characters)
// still encode and round-trip. The scope key encoding intentionally relies only
// on weak validation so that it is forward-compatible with future expansions of
// the valid scope character set.
func TestEncodeForKeyWeaklyValid(t *testing.T) {
	t.Parallel()

	for _, tc := range []string{
		"/.a",
		"/aa/.b",
		"/UPPER",
		"/a/.hidden/b",
	} {
		encoded, err := EncodeForKey(tc)
		require.NoError(t, err, "expected %q to encode", tc)
		decoded, err := DecodeFromKey(encoded)
		require.NoError(t, err)
		require.Equal(t, NormalizeForEquality(tc), decoded)
	}
}

// TestEncodedSort asserts that a basic string sort over encoded scopes
// produces the same ordering as sorting real scopes with [Sort].
func TestEncodedSort(t *testing.T) {
	t.Parallel()

	const numScopes = 1000
	var generatedScopes [numScopes]string
	for i := range numScopes {
		generatedScopes[i] = generateScope()
	}

	unencoded := generatedScopes[:]
	encoded, err := encodeScopes(unencoded)
	require.NoError(t, err)

	slices.SortFunc(unencoded, Sort)
	slices.Sort(encoded)

	decoded, err := decodeScopes(encoded)
	require.NoError(t, err)

	require.Equal(t, unencoded, decoded)

	// Encoded scopes are not only compared as standalone values. They are also
	// commonly used as a key component before a resource name, e.g.
	//
	//	<encoded-scope>/<resource-name>
	//
	// Verify that adding this suffix doesn't change the relative ordering of the
	// encoded scopes.
	composed := make([]string, 0, len(encoded))
	for _, encodedScope := range encoded {
		composed = append(composed, encodedScope+backend.SeparatorString+"resource")
	}
	slices.Sort(composed)
	for i, key := range composed {
		// Split the composed key back into the encoded scope component so we can
		// compare the resulting scope order with the canonical scope sort order.
		encodedScope, _, ok := strings.Cut(key, backend.SeparatorString)
		require.True(t, ok)
		decodedScope, err := DecodeFromKey(encodedScope)
		require.NoError(t, err)
		require.Equal(t, NormalizeForEquality(unencoded[i]), decodedScope)
	}

	// [Sort] does not support empty scopes, so generatedScope does not
	// generate any empty scopes. Manually test that empty encoded scopes sort
	// to the beginning.
	const numEmptyScopes = 10
	encodedEmptyScope, err := EncodeForKey("")
	require.NoError(t, err)
	// Add some encoded empty scopes at the end.
	for range numEmptyScopes {
		encoded = append(encoded, encodedEmptyScope)
	}
	// Make sure they sort to the beginning.
	slices.Sort(encoded)
	for i := range numEmptyScopes {
		require.Equal(t, encodedEmptyScope, encoded[i])
	}
}

func generateScope() string {
	targetLen := 1 + rand.IntN(maxScopeSize)
	var scope strings.Builder
	for scope.Len() <= targetLen-2 {
		scope.WriteString(separator)
		segmentLen := 1 + rand.IntN(targetLen-scope.Len())
		scope.WriteString(generateSegment(segmentLen))
	}
	if scope.Len() == 0 {
		return Root
	}
	return scope.String()
}

// generateSegment generates a scope segment considered valid by EncodeForKey,
// meaning it must be weakly valid. Weak validation accepts any non-space
// printable ASCII byte (i.e. [33, 126]) that is not a breaking character, at any
// position within the segment.
func generateSegment(segmentLen int) string {
	const (
		minByte = 33
		maxByte = 126
	)
	b := make([]byte, segmentLen)
	for i := range segmentLen {
		b[i] = randomValidByteInRange(minByte, maxByte)
	}
	return string(b)
}

func randomValidByteInRange(min, max int) byte {
	for {
		candidate := byte(min + rand.IntN(max-min+1))
		if !strings.ContainsRune(breakingChars, rune(candidate)) {
			return candidate
		}
	}
}

func encodeScopes(scopes []string) ([]string, error) {
	encoded := make([]string, len(scopes))
	for i, scope := range scopes {
		var err error
		encoded[i], err = EncodeForKey(scope)
		if err != nil {
			return nil, err
		}
	}
	return encoded, nil
}

func decodeScopes(encodedScopes []string) ([]string, error) {
	scopes := make([]string, len(encodedScopes))
	for i, encodedScope := range encodedScopes {
		var err error
		scopes[i], err = DecodeFromKey(encodedScope)
		if err != nil {
			return nil, err
		}
	}
	return scopes, nil
}
