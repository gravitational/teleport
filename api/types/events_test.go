/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWatchKindContains tests that the WatchKind.Contains method correctly detects whether its receiver contains its
// argument.
func TestWatchKindContains(t *testing.T) {
	allCAFilter := make(CertAuthorityFilter)
	for _, caType := range CertAuthTypes {
		allCAFilter[caType] = Wildcard
	}
	testCases := []struct {
		name      string
		kind      WatchKind
		other     WatchKind
		assertion require.BoolAssertionFunc
	}{
		{
			name: "yes: kind and subkind match",
			kind: WatchKind{
				Kind:    "a",
				SubKind: "b",
			},
			other: WatchKind{
				Kind:    "a",
				SubKind: "b",
			},
			assertion: require.True,
		},
		{
			name: "no: kind and subkind don't match",
			kind: WatchKind{
				Kind:    "a",
				SubKind: "b",
			},
			other: WatchKind{
				Kind:    "a",
				SubKind: "c",
			},
			assertion: require.False,
		},
		{
			name: "yes: only subset specifies name",
			kind: WatchKind{
				Kind:    "a",
				SubKind: "b",
			},
			other: WatchKind{
				Kind:    "a",
				SubKind: "b",
				Name:    "c",
			},
			assertion: require.True,
		},
		{
			name: "no: subset is missing name when superset has one",
			kind: WatchKind{
				Kind:    "a",
				SubKind: "b",
				Name:    "c",
			},
			other: WatchKind{
				Kind:    "a",
				SubKind: "b",
			},
			assertion: require.False,
		},
		{
			name: "no: different names",
			kind: WatchKind{
				Kind:    "a",
				SubKind: "b",
				Name:    "c",
			},
			other: WatchKind{
				Kind:    "a",
				SubKind: "b",
				Name:    "d",
			},
			assertion: require.False,
		},
		{
			name: "yes: subset has narrower filter",
			kind: WatchKind{
				Kind:    "a",
				SubKind: "b",
				Filter: map[string]string{
					"c": "d",
				},
			},
			other: WatchKind{
				Kind:    "a",
				SubKind: "b",
				Filter: map[string]string{
					"c": "d",
					"e": "f",
				},
			},
			assertion: require.True,
		},
		{
			name: "no: subset has no filter",
			kind: WatchKind{
				Kind:    "a",
				SubKind: "b",
				Filter: map[string]string{
					"c": "d",
				},
			},
			other: WatchKind{
				Kind:    "a",
				SubKind: "b",
			},
			assertion: require.False,
		},
		{
			name: "no: subset has wider filter",
			kind: WatchKind{
				Kind:    "a",
				SubKind: "b",
				Filter: map[string]string{
					"c": "d",
					"e": "f",
				},
			},
			other: WatchKind{
				Kind:    "a",
				SubKind: "b",
				Filter: map[string]string{
					"e": "f",
				},
			},
			assertion: require.False,
		},
		{
			name: "yes: superset and subset have no CA filter",
			kind: WatchKind{
				Kind: "cert_authority",
			},
			other: WatchKind{
				Kind: "cert_authority",
			},
			assertion: require.True,
		},
		{
			name: "yes: superset has no CA filter",
			kind: WatchKind{
				Kind: "cert_authority",
			},
			other: WatchKind{
				Kind: "cert_authority",
				Filter: map[string]string{
					"a": "b",
					"c": "d",
				},
			},
			assertion: require.True,
		},
		{
			name: "yes: superset filter matches all, subset has no CA filter",
			kind: WatchKind{
				Kind:   "cert_authority",
				Filter: allCAFilter.IntoMap(),
			},
			other: WatchKind{
				Kind: "cert_authority",
			},
			assertion: require.True,
		},
		{
			name: "yes: subset has narrower CA filter",
			kind: WatchKind{
				Kind: "cert_authority",
				Filter: map[string]string{
					"a": "b",
					"c": Wildcard,
					"e": "f",
				},
			},
			other: WatchKind{
				Kind: "cert_authority",
				Filter: map[string]string{
					"a": "b",
					"c": "d",
				},
			},
			assertion: require.True,
		},
		{
			name: "no: superset filter does not match all, subset has no CA filter",
			kind: WatchKind{
				Kind: "cert_authority",
				Filter: map[string]string{
					"a": "b",
					"c": "d",
				},
			},
			other: WatchKind{
				Kind: "cert_authority",
			},
			assertion: require.False,
		},
		{
			name: "no: subset has wider CA filter",
			kind: WatchKind{
				Kind: "cert_authority",
				Filter: map[string]string{
					"a": "b",
					"c": "d",
				},
			},
			other: WatchKind{
				Kind: "cert_authority",
				Filter: map[string]string{
					"a": "b",
					"c": "d",
					"e": "",
				},
			},
			assertion: require.False,
		},
		{
			name: "no: subset filter does not match",
			kind: WatchKind{
				Kind: "cert_authority",
				Filter: map[string]string{
					"a": "b",
				},
			},
			other: WatchKind{
				Kind: "cert_authority",
				Filter: map[string]string{
					"a": "",
				},
			},
			assertion: require.False,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.assertion(t, tc.kind.Contains(tc.other))
		})
	}
}
