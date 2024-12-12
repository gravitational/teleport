// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package workloadidentityv1

import (
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

func Test_getFieldStringValue(t *testing.T) {
	tests := []struct {
		name       string
		in         *workloadidentityv1pb.Attrs
		attr       string
		want       string
		requireErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			in: &workloadidentityv1pb.Attrs{
				User: &workloadidentityv1pb.UserAttrs{
					Name: "jeff",
				},
			},
			attr:       "user.name",
			want:       "jeff",
			requireErr: require.NoError,
		},
		{
			name: "non-string final field",
			in: &workloadidentityv1pb.Attrs{
				User: &workloadidentityv1pb.UserAttrs{
					Name: "user",
				},
			},
			attr: "user",
			requireErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "attribute \"user\" is not a string")
			},
		},
		{
			// We mostly just want this to not panic.
			name:       "nil root",
			in:         nil,
			attr:       "user.name",
			want:       "",
			requireErr: require.NoError,
		},
		{
			// We mostly just want this to not panic.
			name: "nil submessage",
			in: &workloadidentityv1pb.Attrs{
				User: nil,
			},
			attr:       "user.name",
			want:       "",
			requireErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := getFieldStringValue(tt.in, tt.attr)
			require.Equal(t, tt.want, got)
			tt.requireErr(t, gotErr)
		})
	}
}

func Test_templateString(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		want       string
		attrs      *workloadidentityv1pb.Attrs
		requireErr require.ErrorAssertionFunc
	}{
		{
			name: "success mixed",
			in:   "hello{{user.name}}.{{user.name}} {{ workload.kubernetes.pod_name }}//{{ workload.kubernetes.namespace}}",
			want: "hellojeff.jeff pod1//default",
			attrs: &workloadidentityv1pb.Attrs{
				User: &workloadidentityv1pb.UserAttrs{
					Name: "jeff",
				},
				Workload: &workloadidentityv1pb.WorkloadAttrs{
					Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
						PodName:   "pod1",
						Namespace: "default",
					},
				},
			},
			requireErr: require.NoError,
		},
		{
			name: "success with spaces",
			in:   "hello {{user.name}}",
			want: "hello jeff",
			attrs: &workloadidentityv1pb.Attrs{
				User: &workloadidentityv1pb.UserAttrs{
					Name: "jeff",
				},
			},
			requireErr: require.NoError,
		},
		{
			name: "fail due to unset",
			in:   "hello {{workload.kubernetes.pod_name}}",
			attrs: &workloadidentityv1pb.Attrs{
				User: &workloadidentityv1pb.UserAttrs{
					Name: "jeff",
				},
			},
			requireErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "attribute \"workload.kubernetes.pod_name\" unset")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := templateString(tt.in, tt.attrs)
			tt.requireErr(t, gotErr)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_evaluateRules(t *testing.T) {
	attrs := &workloadidentityv1pb.Attrs{
		User: &workloadidentityv1pb.UserAttrs{
			Name: "foo",
		},
	}
	wi := &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "test",
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Rules: &workloadidentityv1pb.WorkloadIdentityRules{
				Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
					{
						Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
							{
								Attribute: "user.name",
								Equals:    "foo",
							},
						},
					},
				},
			},
		},
	}
	err := evaluateRules(wi, attrs)
	require.NoError(t, err)
}
