/*
 * Copyright 2026 Gravitational, Inc.
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

package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestValidatedMFAChallengeFilterIntoMap(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		filter types.ValidatedMFAChallengeFilter
		want   map[string]string
	}{
		{
			name:   "empty filter",
			filter: types.ValidatedMFAChallengeFilter{},
			want:   map[string]string{},
		},
		{
			name: "target cluster",
			filter: types.ValidatedMFAChallengeFilter{
				TargetCluster: "root",
			},
			want: map[string]string{
				"target_cluster": "root",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.filter.IntoMap())
		})
	}
}

func TestValidatedMFAChallengeFilterFromMap(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		filter types.ValidatedMFAChallengeFilter
		input  map[string]string
		want   types.ValidatedMFAChallengeFilter
	}{
		{
			name:   "empty map leaves filter unchanged",
			filter: types.ValidatedMFAChallengeFilter{},
			input:  map[string]string{},
			want:   types.ValidatedMFAChallengeFilter{},
		},
		{
			name:   "target cluster is copied from map",
			filter: types.ValidatedMFAChallengeFilter{},
			input: map[string]string{
				"target_cluster": "leaf",
			},
			want: types.ValidatedMFAChallengeFilter{
				TargetCluster: "leaf",
			},
		},
		{
			name: "missing target cluster preserves existing value",
			filter: types.ValidatedMFAChallengeFilter{
				TargetCluster: "root",
			},
			input: map[string]string{
				"unrelated": "value",
			},
			want: types.ValidatedMFAChallengeFilter{
				TargetCluster: "root",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filter := tc.filter
			filter.FromMap(tc.input)
			require.Equal(t, tc.want, filter)
		})
	}
}

func TestValidatedMFAChallengeFilterMatch(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		filter        types.ValidatedMFAChallengeFilter
		targetCluster string
		want          bool
	}{
		{
			name:          "empty filter matches nothing",
			filter:        types.ValidatedMFAChallengeFilter{},
			targetCluster: "root",
			want:          false,
		},
		{
			name: "matching target cluster",
			filter: types.ValidatedMFAChallengeFilter{
				TargetCluster: "root",
			},
			targetCluster: "root",
			want:          true,
		},
		{
			name: "different target cluster",
			filter: types.ValidatedMFAChallengeFilter{
				TargetCluster: "root",
			},
			targetCluster: "leaf",
			want:          false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.filter.Match(tc.targetCluster))
		})
	}
}
