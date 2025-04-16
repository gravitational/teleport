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
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

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
		{
			name: "expression: pass",
			wid: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Rules: &workloadidentityv1pb.WorkloadIdentityRules{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							{Expression: `user.name == "foo"`},
						},
					},
				},
			},
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "expression: fail",
			wid: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Rules: &workloadidentityv1pb.WorkloadIdentityRules{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							{Expression: `user.name == "not-foo"`},
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

func TestTemplateExtraClaims_Success(t *testing.T) {
	const inputJSON = `
		{
			"simple-string": "hello world",
			"simple-number": 1234,
			"simple-bool": true,
			"null": null,
			"object": {
				"message": "hello, {{user.name}}",
				"workload": {
					"podman": {
						"pod_name": "{{workload.podman.pod.name}}",
						"labels": ["{{workload.podman.pod.labels[\"a\"]}}", "{{workload.podman.pod.labels[\"b\"]}}", "c"]
					}
				}
			}
		}
	`

	const expectedOutputJSON = `
	{
		"simple-string": "hello world",
		"simple-number": 1234,
		"simple-bool": true,
		"null": null,
		"object": {
			"message": "hello, Bobby",
			"workload": {
				"podman": {
					"pod_name": "webserver",
					"labels": ["a", "b", "c"]
				}
			}
		}
	}
	`

	var input, expectedOutput *structpb.Struct
	err := json.Unmarshal([]byte(inputJSON), &input)
	require.NoError(t, err)

	err = json.Unmarshal([]byte(expectedOutputJSON), &expectedOutput)
	require.NoError(t, err)

	output, err := templateExtraClaims(input, &workloadidentityv1pb.Attrs{
		User: &workloadidentityv1pb.UserAttrs{
			Name: "Bobby",
		},
		Workload: &workloadidentityv1pb.WorkloadAttrs{
			Podman: &workloadidentityv1pb.WorkloadAttrsPodman{
				Pod: &workloadidentityv1pb.WorkloadAttrsPodmanPod{
					Name:   "webserver",
					Labels: map[string]string{"a": "a", "b": "b"},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expectedOutput, output, protocmp.Transform()))
}

func TestTemplateExtraClaims_Failure(t *testing.T) {
	const claimsJSON = `
		{
			"foo": {
				"bar": {
					"baz": ["a", {"b":"{{blah}}"}, "c"]
				}
			}
		}
	`

	var rawClaims *structpb.Struct
	err := json.Unmarshal([]byte(claimsJSON), &rawClaims)
	require.NoError(t, err)

	_, err = templateExtraClaims(rawClaims, &workloadidentityv1pb.Attrs{})
	require.ErrorContains(t, err, "templating claim: foo.bar.baz[1].b")
	require.ErrorContains(t, err, `unknown identifier: "blah"`)
}

func TestTemplateExtraClaims_TooDeeplyNested(t *testing.T) {
	const claimsJSON = `
		{
			"1": {
				"2": {
					"3": {
						"4": {
							"5": {
								"6": {
									"7": {
										"8": {
											"9": {
												"10": "very deep"
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	`

	var rawClaims *structpb.Struct
	err := json.Unmarshal([]byte(claimsJSON), &rawClaims)
	require.NoError(t, err)

	_, err = templateExtraClaims(rawClaims, &workloadidentityv1pb.Attrs{})
	require.ErrorContains(t, err, "cannot contain more than 10 levels of nesting")
}
