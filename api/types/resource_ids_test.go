/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestResourceIDs tests that ResourceIDs are correctly marshaled to and from
// their string representation.
func TestResourceIDs(t *testing.T) {
	testCases := []struct {
		desc             string
		in               []ResourceID
		expected         string
		expectParseError bool
	}{
		{
			desc: "single id",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindNode,
				Name:        "uuid",
			}},
			expected: `["/one/node/uuid"]`,
		},
		{
			desc: "multiple ids",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindNode,
				Name:        "uuid-1",
			}, {
				ClusterName: "two",
				Kind:        KindDatabase,
				Name:        "uuid-2",
			}},
			expected: `["/one/node/uuid-1","/two/db/uuid-2"]`,
		},
		{
			desc: "no cluster name",
			in: []ResourceID{{
				ClusterName: "",
				Kind:        KindNode,
				Name:        "uuid",
			}},
			expected:         `["//node/uuid"]`,
			expectParseError: true,
		},
		{
			desc: "bad cluster name",
			in: []ResourceID{{
				ClusterName: "/,",
				Kind:        KindNode,
				Name:        "uuid",
			}},
			expected:         `["//,/node/uuid"]`,
			expectParseError: true,
		},
		{
			desc: "bad resource kind",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "not,/a,/kind",
				Name:        "uuid",
			}},
			expected:         `["/one/not,/a,/kind/uuid"]`,
			expectParseError: true,
		},
		{
			// Any resource name is actually fine, test that the list parsing
			// doesn't break.
			desc: "bad resource name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindNode,
				Name:        `really"--,bad resource\"\\"name`,
			}},
			expected: `["/one/node/really\"--,bad resource\\\"\\\\\"name"]`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			out, err := ResourceIDsToString(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.expected, out)

			// Parse the ids from the string and make sure they match the
			// original.
			parsed, err := ResourceIDsFromString(out)
			if tc.expectParseError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.in, parsed)
		})
	}
}
