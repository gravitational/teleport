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
	}{
		{
			name:    "Query exceeds max limit size",
			maxSize: 6000,
			in: &DatabaseSessionQuery{
				DatabaseQuery: strings.Repeat("A", 7000),
			},
			want: &DatabaseSessionQuery{
				DatabaseQuery: strings.Repeat("A", 5377),
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
				DatabaseQuery: strings.Repeat("A", 592),
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
				DatabaseQuery: strings.Repeat("A", 223),
				DatabaseQueryParameters: []string{
					strings.Repeat("A", 89),
					strings.Repeat("A", 89),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sr, ok := tc.in.(messageSizeTrimmer)
			require.True(t, ok)

			got := sr.TrimToMaxSize(tc.maxSize)

			require.Empty(t, cmp.Diff(got, tc.want))
			require.Less(t, got.Size(), tc.maxSize)
		})
	}
}
