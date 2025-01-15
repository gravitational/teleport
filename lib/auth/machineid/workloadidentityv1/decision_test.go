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
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

func Test_decide(t *testing.T) {
	standardAttrs := &workloadidentityv1pb.Attrs{
		User: &workloadidentityv1pb.UserAttrs{
			Name: "jeff",
		},
		Workload: &workloadidentityv1pb.WorkloadAttrs{
			Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
				PodName:   "pod1",
				Namespace: "default",
			},
		},
	}
	tests := []struct {
		name         string
		wid          *workloadidentityv1pb.WorkloadIdentity
		attrs        *workloadidentityv1pb.Attrs
		wantIssue    bool
		assertReason require.ErrorAssertionFunc
	}{
		{
			name: "invalid dns name",
			wid: &workloadidentityv1pb.WorkloadIdentity{
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/valid",
						X509: &workloadidentityv1pb.WorkloadIdentitySPIFFEX509{
							DnsSans: []string{
								"//imvalid;;",
							},
						},
					},
				},
			},
			attrs:     standardAttrs,
			wantIssue: false,
			assertReason: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "templating spec.spiffe.x509.dns_sans[0] resulted in an invalid DNS name")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := decide(context.Background(), tt.wid, tt.attrs)
			require.Equal(t, tt.wantIssue, d.shouldIssue)
			tt.assertReason(t, d.reason)
		})
	}
}

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
			// This test ensures that the proto name (e.g service_account) is
			// used instead of the Go name (e.g serviceAccount).
			name: "underscored",
			in: &workloadidentityv1pb.Attrs{
				Join: &workloadidentityv1pb.JoinAttrs{
					Kubernetes: &workloadidentityv1pb.JoinAttrsKubernetes{
						ServiceAccount: &workloadidentityv1pb.JoinAttrsKubernetesServiceAccount{
							Namespace: "default",
						},
					},
				},
			},
			attr:       "join.kubernetes.service_account.namespace",
			want:       "default",
			requireErr: require.NoError,
		},
		{
			name: "bool",
			in: &workloadidentityv1pb.Attrs{
				User: &workloadidentityv1pb.UserAttrs{
					Name: "jeff",
				},
				Workload: &workloadidentityv1pb.WorkloadAttrs{
					Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
						Attested: true,
					},
				},
			},
			attr:       "workload.unix.attested",
			want:       "true",
			requireErr: require.NoError,
		},
		{
			name: "int32",
			in: &workloadidentityv1pb.Attrs{
				User: &workloadidentityv1pb.UserAttrs{
					Name: "jeff",
				},
				Workload: &workloadidentityv1pb.WorkloadAttrs{
					Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
						Pid: 123,
					},
				},
			},
			attr:       "workload.unix.pid",
			want:       "123",
			requireErr: require.NoError,
		},
		{
			name: "uint32",
			in: &workloadidentityv1pb.Attrs{
				User: &workloadidentityv1pb.UserAttrs{
					Name: "jeff",
				},
				Workload: &workloadidentityv1pb.WorkloadAttrs{
					Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
						Gid: 123,
					},
				},
			},
			attr:       "workload.unix.gid",
			want:       "123",
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
				require.ErrorContains(t, err, "attribute \"user\" of type \"message\" cannot be converted to string")
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
			tt.requireErr(t, gotErr)
			require.Equal(t, tt.want, got)
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
		Workload: &workloadidentityv1pb.WorkloadAttrs{
			Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
				PodName:   "pod1",
				Namespace: "default",
			},
		},
	}

	var noMatchRule require.ErrorAssertionFunc = func(t require.TestingT, err error, i ...interface{}) {
		require.Error(t, err)
		require.Contains(t, err.Error(), "no matching rule found")
	}

	tests := []struct {
		name       string
		wid        *workloadidentityv1pb.WorkloadIdentity
		attrs      *workloadidentityv1pb.Attrs
		requireErr require.ErrorAssertionFunc
	}{
		{
			name: "no rules: pass",
			wid: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Rules: &workloadidentityv1pb.WorkloadIdentityRules{},
				},
			},
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "eq: pass",
			wid: &workloadidentityv1pb.WorkloadIdentity{
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
										Operator: &workloadidentityv1pb.WorkloadIdentityCondition_Eq{
											Eq: &workloadidentityv1pb.WorkloadIdentityConditionEq{
												Value: "foo",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "eq: fail",
			wid: &workloadidentityv1pb.WorkloadIdentity{
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
										Operator: &workloadidentityv1pb.WorkloadIdentityCondition_Eq{
											Eq: &workloadidentityv1pb.WorkloadIdentityConditionEq{
												Value: "not-foo",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			attrs:      attrs,
			requireErr: noMatchRule,
		},
		{
			name: "not_eq: pass",
			wid: &workloadidentityv1pb.WorkloadIdentity{
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
										Operator: &workloadidentityv1pb.WorkloadIdentityCondition_NotEq{
											NotEq: &workloadidentityv1pb.WorkloadIdentityConditionNotEq{
												Value: "bar",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "not_eq: fail",
			wid: &workloadidentityv1pb.WorkloadIdentity{
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
										Operator: &workloadidentityv1pb.WorkloadIdentityCondition_NotEq{
											NotEq: &workloadidentityv1pb.WorkloadIdentityConditionNotEq{
												Value: "foo",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			attrs:      attrs,
			requireErr: noMatchRule,
		},
		{
			name: "in: pass",
			wid: &workloadidentityv1pb.WorkloadIdentity{
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
										Operator: &workloadidentityv1pb.WorkloadIdentityCondition_In{
											In: &workloadidentityv1pb.WorkloadIdentityConditionIn{
												Values: []string{"bar", "foo"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "in: fail",
			wid: &workloadidentityv1pb.WorkloadIdentity{
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
										Operator: &workloadidentityv1pb.WorkloadIdentityCondition_In{
											In: &workloadidentityv1pb.WorkloadIdentityConditionIn{
												Values: []string{"bar", "fizz"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			attrs:      attrs,
			requireErr: noMatchRule,
		},
		{
			name: "not_in: pass",
			wid: &workloadidentityv1pb.WorkloadIdentity{
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
										Operator: &workloadidentityv1pb.WorkloadIdentityCondition_NotIn{
											NotIn: &workloadidentityv1pb.WorkloadIdentityConditionNotIn{
												Values: []string{"bar", "fizz"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "in: fail",
			wid: &workloadidentityv1pb.WorkloadIdentity{
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
										Operator: &workloadidentityv1pb.WorkloadIdentityCondition_NotIn{
											NotIn: &workloadidentityv1pb.WorkloadIdentityConditionNotIn{
												Values: []string{"bar", "foo"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			attrs:      attrs,
			requireErr: noMatchRule,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := evaluateRules(tt.wid, tt.attrs)
			tt.requireErr(t, err)
		})
	}
}
