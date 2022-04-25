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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestSanitizeMessageSize(t *testing.T) {
	type sanitizer interface {
		Sanitize(n int) AuditEvent
	}

	testCases := []struct {
		name    string
		maxSize int
		in      AuditEvent
		want    AuditEvent
	}{
		{
			name:    "Query exceeds max limit size",
			maxSize: 6000,
			in: &DatabaseSessionQuery{
				DatabaseQuery: stringN(7000),
			},
			want: &DatabaseSessionQuery{
				DatabaseQuery: stringN(5377),
			},
		},
		{
			name:    "Query with query params exceeds max size",
			maxSize: 2000,
			in: &DatabaseSessionQuery{
				DatabaseQuery: stringN(2000),
				DatabaseQueryParameters: []string{
					stringN(89),
					stringN(89),
				},
			},
			want: &DatabaseSessionQuery{
				DatabaseQuery: stringN(592),
				DatabaseQueryParameters: []string{
					stringN(89),
					stringN(89),
				},
			},
		},
		{
			name:    "with metadata",
			maxSize: 3000,
			in: &DatabaseSessionQuery{
				Metadata: Metadata{
					ClusterName: stringN(2000),
					Index:       1,
				},
				DatabaseQuery: stringN(2000),
				DatabaseQueryParameters: []string{
					stringN(89),
					stringN(89),
				},
			},
			want: &DatabaseSessionQuery{
				Metadata: Metadata{
					ClusterName: stringN(2000),
					Index:       1,
				},
				DatabaseQuery: stringN(223),
				DatabaseQueryParameters: []string{
					stringN(89),
					stringN(89),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sr, ok := tc.in.(sanitizer)
			require.True(t, ok)

			got := sr.Sanitize(tc.maxSize)

			require.Empty(t, cmp.Diff(got, tc.want))
			require.Less(t, got.Size(), tc.maxSize)
		})
	}
}

func stringN(n int) string {
	s := string(make([]byte, n))
	return s
}
