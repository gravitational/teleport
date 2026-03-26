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

package pinning

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
)

// TestEncodeDecode tests that encoding and then decoding a Scope Pin returns the original Pin.
func TestEncodeDecode(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name string
		pin  *scopesv1.Pin
	}{
		{
			name: "single role assignment",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/foo": {
						"/foo": {"role1"},
					},
				}),
			},
		},
		{
			name: "multiple roles at same scope",
			pin: &scopesv1.Pin{
				Scope: "/staging",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/staging": {
						"/staging": {"admin", "developer", "viewer"},
					},
				}),
			},
		},
		{
			name: "hierarchical assignments",
			pin: &scopesv1.Pin{
				Scope: "/foo",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/":        {"root-global"},
						"/foo":     {"root-foo"},
						"/foo/bar": {"root-bar"},
					},
					"/foo": {
						"/foo":     {"foo-foo"},
						"/foo/bar": {"foo-bar"},
					},
					"/foo/bar": {
						"/foo/bar": {"bar-bar"},
					},
				}),
			},
		},
		{
			name: "complex multi-branch tree",
			pin: &scopesv1.Pin{
				Scope: "/staging/west",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/":             {"global"},
						"/staging":      {"root-staging"},
						"/staging/west": {"root-west"},
						"/staging/east": {"root-east"},
					},
					"/staging": {
						"/staging":      {"staging-base"},
						"/staging/west": {"staging-west-1", "staging-west-2"},
						"/staging/east": {"staging-east"},
					},
					"/staging/west": {
						"/staging/west": {"west-local"},
					},
				}),
			},
		},
		{
			name: "empty assignment tree",
			pin: &scopesv1.Pin{
				Scope:          "/foo",
				AssignmentTree: &scopesv1.AssignmentNode{},
			},
		},
		{
			name: "root scope",
			pin: &scopesv1.Pin{
				Scope: "/",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/": {"global-admin"},
					},
				}),
			},
		},
		{
			name: "deep hierarchy",
			pin: &scopesv1.Pin{
				Scope: "/a/b/c/d",
				AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
					"/": {
						"/a/b/c/d": {"role-from-root"},
					},
					"/a": {
						"/a/b/c/d": {"role-from-a"},
					},
					"/a/b": {
						"/a/b/c/d": {"role-from-ab"},
					},
					"/a/b/c": {
						"/a/b/c/d": {"role-from-abc"},
					},
					"/a/b/c/d": {
						"/a/b/c/d": {"role-from-abcd"},
					},
				}),
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := Encode(tt.pin)
			require.NoError(t, err)
			require.NotEmpty(t, encoded)

			decoded, err := Decode(encoded)
			require.NoError(t, err)
			require.NotNil(t, decoded)

			require.Empty(t, cmp.Diff(tt.pin, decoded, protocmp.Transform()))
		})
	}
}

// TestEncodeDecodeErrors verifies some basic failure scenarios.
func TestEncodeDecodeErrors(t *testing.T) {
	t.Parallel()

	t.Run("decode invalid base64", func(t *testing.T) {
		_, err := Decode("not valid base64!!!")
		require.Error(t, err)
	})

	t.Run("decode invalid protobuf", func(t *testing.T) {
		_, err := Decode("YWJjZGVm")
		require.Error(t, err)
	})

	t.Run("decode empty string", func(t *testing.T) {
		_, err := Decode("")
		require.Error(t, err)
	})

	t.Run("encode empty pin", func(t *testing.T) {
		_, err := Encode(&scopesv1.Pin{})
		require.Error(t, err)
	})

	t.Run("encode nil pin", func(t *testing.T) {
		_, err := Encode(nil)
		require.Error(t, err)
	})
}

// TestDecodeKnown tests that decoding a known encoding returns the expected Pin. This includes
// verification that Decode correctly handles json input.  As a rule we don't *need* decode to
// continue to handle json input as the scopes feature was still highly unstable at the time we
// broke compatibility with the json encoding, but some confusing errors are avoided by having
// this fallback.
func TestDecodeKnown(t *testing.T) {
	t.Parallel()

	encoded := "CgUvdGVzdBodChsKBHRlc3QSExIRCg8KBHRlc3QSBxIFcm9sZTE"

	encodedJSON := `{"scope":"/test", "assignmentTree":{"children":{"test":{"roleTree":{"children":{"test":{"roles":["role1"]}}}}}}}`

	expect := &scopesv1.Pin{
		Scope: "/test",
		AssignmentTree: AssignmentTreeFromMap(map[string]map[string][]string{
			"/test": {
				"/test": {"role1"},
			},
		}),
	}

	decoded, err := Decode(encoded)
	require.NoError(t, err)
	require.NotNil(t, decoded)
	require.Empty(t, cmp.Diff(expect, decoded, protocmp.Transform()))

	decodedJSON, err := Decode(encodedJSON)
	require.NoError(t, err)
	require.NotNil(t, decodedJSON)
	require.Empty(t, cmp.Diff(expect, decodedJSON, protocmp.Transform()))
}
