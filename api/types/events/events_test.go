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

package events

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

func TestTrimToMaxSize(t *testing.T) {
	type messageSizeTrimmer interface {
		TrimToMaxSize(int) AuditEvent
	}

	testCases := []struct {
		name    string
		maxSize int
		in      AuditEvent
		want    AuditEvent
		cmpOpts []cmp.Option
	}{
		// DatabaseSessionQuery
		{
			name:    "Query exceeds max limit size",
			maxSize: 6000,
			in: &DatabaseSessionQuery{
				DatabaseQuery: strings.Repeat("A", 7000),
			},
			want: &DatabaseSessionQuery{
				DatabaseQuery: strings.Repeat("A", 5375),
			},
		},
		{
			name:    "Query with query params exceeds max size",
			maxSize: 2000,
			in: &DatabaseSessionQuery{
				DatabaseQuery: strings.Repeat("A", 2000),
				DatabaseQueryParameters: []string{
					strings.Repeat("A", 89),
					strings.Repeat("A", 89),
				},
			},
			want: &DatabaseSessionQuery{
				DatabaseQuery: strings.Repeat("A", 590),
				DatabaseQueryParameters: []string{
					strings.Repeat("A", 89),
					strings.Repeat("A", 89),
				},
			},
		},
		{
			name:    "with metadata",
			maxSize: 3000,
			in: &DatabaseSessionQuery{
				Metadata: Metadata{
					ClusterName: strings.Repeat("A", 2000),
					Index:       1,
				},
				DatabaseQuery: strings.Repeat("A", 2000),
				DatabaseQueryParameters: []string{
					strings.Repeat("A", 89),
					strings.Repeat("A", 89),
				},
			},
			want: &DatabaseSessionQuery{
				Metadata: Metadata{
					ClusterName: strings.Repeat("A", 2000),
					Index:       1,
				},
				DatabaseQuery: strings.Repeat("A", 221),
				DatabaseQueryParameters: []string{
					strings.Repeat("A", 89),
					strings.Repeat("A", 89),
				},
			},
		},
		{
			name:    "Query requires heavy escaping",
			maxSize: 50,
			in: &DatabaseSessionQuery{
				DatabaseQuery: `{` + strings.Repeat(`"a": "b",`, 100) + "}",
			},
			want: &DatabaseSessionQuery{
				DatabaseQuery: `{"a": "b","a":`,
			},
		},
		// UserLogin
		{
			name:    "UserLogin event with error",
			maxSize: 3000,
			in: &UserLogin{
				Status: Status{
					Error:       strings.Repeat("A", 2000),
					UserMessage: strings.Repeat("A", 2000),
				},
			},
			want: &UserLogin{
				Status: Status{
					Error:       strings.Repeat("A", 1336),
					UserMessage: strings.Repeat("A", 1336),
				},
			},
			cmpOpts: []cmp.Option{
				// UserLogin.IdentityAttributes has an Equal method which gets used
				// by cmp.Diff but fails whether nil or an empty struct is supplied.
				cmpopts.IgnoreFields(UserLogin{}, "IdentityAttributes"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sr, ok := tc.in.(messageSizeTrimmer)
			require.True(t, ok)

			got := sr.TrimToMaxSize(tc.maxSize)

			require.Empty(t, cmp.Diff(got, tc.want, tc.cmpOpts...))
			require.Less(t, got.Size(), tc.maxSize)
		})
	}
}

func TestTrimStr(t *testing.T) {
	tests := []struct {
		have string
		want string
	}{
		{strings.Repeat("A", 17) + `\n`, strings.Repeat("A", 17) + `\`},
		{strings.Repeat(`A\n`, 200), `A\nA\nA\nA\nA\`},
		{strings.Repeat(`A\a`, 200), `A\aA\aA\aA\aA\`},
		{strings.Repeat(`A\t`, 200), `A\tA\tA\tA\tA\`},
		{`{` + strings.Repeat(`"a": "b",`, 100) + "}", `{"a": "b","a"`},
	}

	const maxLen = 20
	for _, test := range tests {
		require.Equal(t, test.want, trimStr(test.have, maxLen))
	}
}
