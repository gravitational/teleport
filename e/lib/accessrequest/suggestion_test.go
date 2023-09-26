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

package accessrequest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
)

func TestScoreRelevance(t *testing.T) {
	t.Parallel()

	makeAccessList := func(roles ...string) *accesslist.AccessList {
		return &accesslist.AccessList{
			Spec: accesslist.Spec{
				Grants: accesslist.Grants{Roles: []string{"role1"}},
			},
		}
	}

	type args struct {
		request types.AccessRequest
		lists   []*accesslist.AccessList
	}
	tests := []struct {
		name string
		args args
		want []*accesslist.AccessList
	}{
		{
			name: "no lists",
			args: args{
				request: &types.AccessRequestV3{},
				lists:   []*accesslist.AccessList{},
			},
			want: []*accesslist.AccessList{},
		},
		{
			name: "one role",
			args: args{
				request: &types.AccessRequestV3{
					Spec: types.AccessRequestSpecV3{Roles: []string{"role1"}},
				},
				lists: []*accesslist.AccessList{
					makeAccessList("role1"),
				},
			},
			want: []*accesslist.AccessList{
				makeAccessList("role1"),
			},
		},
		{
			name: "irrelevant roles are penalized",
			args: args{
				request: &types.AccessRequestV3{
					Spec: types.AccessRequestSpecV3{Roles: []string{"role1"}},
				},
				lists: []*accesslist.AccessList{
					makeAccessList("role1", "role2"),
					makeAccessList("role1"),
				},
			},
			want: []*accesslist.AccessList{
				makeAccessList("role1"),
				makeAccessList("role1", "role2"),
			},
		},
		{
			name: "relevant roles are prioritized",
			args: args{
				request: &types.AccessRequestV3{
					Spec: types.AccessRequestSpecV3{Roles: []string{"role1", "role3"}},
				},
				lists: []*accesslist.AccessList{
					makeAccessList("role1", "role2"),
					makeAccessList("role1", "role3"),
				},
			},
			want: []*accesslist.AccessList{
				makeAccessList("role1", "role3"),
				makeAccessList("role1", "role2"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreRelevance(tt.args.request, tt.args.lists)
			require.Equal(t, tt.want, got)
		})
	}
}
