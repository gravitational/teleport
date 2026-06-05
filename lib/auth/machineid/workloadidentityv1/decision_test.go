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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

func Test_decide(t *testing.T) {
	standardAttrs := workloadidentityv1pb.Attrs_builder{
		User: workloadidentityv1pb.UserAttrs_builder{
			Name: "jeff",
		}.Build(),
		Workload: workloadidentityv1pb.WorkloadAttrs_builder{
			Kubernetes: workloadidentityv1pb.WorkloadAttrsKubernetes_builder{
				PodName:   "pod1",
				Namespace: "default",
			}.Build(),
		}.Build(),
	}.Build()
	tests := []struct {
		name         string
		wid          *workloadidentityv1pb.WorkloadIdentity
		attrs        *workloadidentityv1pb.Attrs
		wantIssue    bool
		assertReason require.ErrorAssertionFunc
	}{
		{
			name: "invalid dns name",
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/valid",
						X509: workloadidentityv1pb.WorkloadIdentitySPIFFEX509_builder{
							DnsSans: []string{
								"//imvalid;;",
							},
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			attrs:     standardAttrs,
			wantIssue: false,
			assertReason: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "templating spec.spiffe.x509.dns_sans[0] resulted in an invalid DNS SAN")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := decide(context.Background(), tt.wid, tt.attrs, OSSSigstorePolicyEvaluator{})
			require.Equal(t, tt.wantIssue, d.shouldIssue)
			tt.assertReason(t, d.reason)
		})
	}
}

func Test_evaluateRules(t *testing.T) {
	attrs := workloadidentityv1pb.Attrs_builder{
		User: workloadidentityv1pb.UserAttrs_builder{
			Name: "foo",
		}.Build(),
		Workload: workloadidentityv1pb.WorkloadAttrs_builder{
			Kubernetes: workloadidentityv1pb.WorkloadAttrsKubernetes_builder{
				PodName:   "pod1",
				Namespace: "default",
			}.Build(),
		}.Build(),
	}.Build()

	var noMatchRule require.ErrorAssertionFunc = func(t require.TestingT, err error, i ...any) {
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
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: &workloadidentityv1pb.WorkloadIdentityRules{},
				}.Build(),
			}.Build(),
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "eq: pass",
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "user.name",
										Eq: workloadidentityv1pb.WorkloadIdentityConditionEq_builder{
											Value: "foo",
										}.Build(),
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "eq: fail",
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "user.name",
										Eq: workloadidentityv1pb.WorkloadIdentityConditionEq_builder{
											Value: "not-foo",
										}.Build(),
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			attrs:      attrs,
			requireErr: noMatchRule,
		},
		{
			name: "not_eq: pass",
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "user.name",
										NotEq: workloadidentityv1pb.WorkloadIdentityConditionNotEq_builder{
											Value: "bar",
										}.Build(),
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "not_eq: fail",
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "user.name",
										NotEq: workloadidentityv1pb.WorkloadIdentityConditionNotEq_builder{
											Value: "foo",
										}.Build(),
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			attrs:      attrs,
			requireErr: noMatchRule,
		},
		{
			name: "in: pass",
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "user.name",
										In: workloadidentityv1pb.WorkloadIdentityConditionIn_builder{
											Values: []string{"bar", "foo"},
										}.Build(),
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "in: fail",
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "user.name",
										In: workloadidentityv1pb.WorkloadIdentityConditionIn_builder{
											Values: []string{"bar", "fizz"},
										}.Build(),
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			attrs:      attrs,
			requireErr: noMatchRule,
		},
		{
			name: "not_in: pass",
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "user.name",
										NotIn: workloadidentityv1pb.WorkloadIdentityConditionNotIn_builder{
											Values: []string{"bar", "fizz"},
										}.Build(),
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "in: fail",
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "user.name",
										NotIn: workloadidentityv1pb.WorkloadIdentityConditionNotIn_builder{
											Values: []string{"bar", "foo"},
										}.Build(),
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			attrs:      attrs,
			requireErr: noMatchRule,
		},
		{
			name: "expression: pass",
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{Expression: `user.name == "foo"`}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			attrs:      attrs,
			requireErr: require.NoError,
		},
		{
			name: "expression: fail",
			wid: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{Expression: `user.name == "not-foo"`}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			attrs:      attrs,
			requireErr: noMatchRule,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := evaluateRules(context.Background(), tt.wid, tt.attrs,
				OSSSigstorePolicyEvaluator{}, make(map[string]error))
			tt.requireErr(t, err)
		})
	}
}

func Test_decision_sigstore(t *testing.T) {
	identity := workloadidentityv1pb.WorkloadIdentity_builder{
		Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
			Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
				Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
					workloadidentityv1pb.WorkloadIdentityRule_builder{Expression: `sigstore.policy_satisfied("foo") && sigstore.policy_satisfied("bar")`}.Build(),
				},
			}.Build(),
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{},
		}.Build(),
	}.Build()
	attrs := &workloadidentityv1pb.Attrs{}

	t.Run("success", func(t *testing.T) {
		evaluator := newMockSigstorePolicyEvaluator(t)

		for policy, result := range map[string]error{
			"foo": nil,
			"bar": nil,
		} {
			evaluator.On("Evaluate", mock.Anything, []string{policy}, attrs).
				Return(map[string]error{policy: result}, nil)
		}

		decision := decide(
			context.Background(),
			identity,
			attrs,
			evaluator,
		)
		require.True(t, decision.shouldIssue)
		require.NoError(t, decision.reason)
	})

	t.Run("failure", func(t *testing.T) {
		evaluator := newMockSigstorePolicyEvaluator(t)

		results := map[string]error{
			"foo": nil,
			"bar": errors.New("missing artifact signature"),
		}
		for policy, result := range results {
			evaluator.On("Evaluate", mock.Anything, []string{policy}, attrs).
				Return(map[string]error{policy: result}, nil)
		}

		decision := decide(
			context.Background(),
			identity,
			attrs,
			evaluator,
		)
		require.False(t, decision.shouldIssue)
		require.Equal(t, results, decision.sigstorePolicyResults)
	})
}

func newMockSigstorePolicyEvaluator(t *testing.T) *mockSigstorePolicyEvaluator {
	t.Helper()

	eval := new(mockSigstorePolicyEvaluator)
	t.Cleanup(func() { _ = eval.AssertExpectations(t) })

	return eval
}

type mockSigstorePolicyEvaluator struct {
	mock.Mock
}

func (m *mockSigstorePolicyEvaluator) Evaluate(ctx context.Context, policyNames []string, attrs *workloadidentityv1pb.Attrs) (map[string]error, error) {
	result := m.Called(ctx, policyNames, attrs)
	return result.Get(0).(map[string]error), result.Error(1)
}

var _ SigstorePolicyEvaluator = (*mockSigstorePolicyEvaluator)(nil)

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

	output, err := templateExtraClaims(input, workloadidentityv1pb.Attrs_builder{
		User: workloadidentityv1pb.UserAttrs_builder{
			Name: "Bobby",
		}.Build(),
		Workload: workloadidentityv1pb.WorkloadAttrs_builder{
			Podman: workloadidentityv1pb.WorkloadAttrsPodman_builder{
				Pod: workloadidentityv1pb.WorkloadAttrsPodmanPod_builder{
					Name:   "webserver",
					Labels: map[string]string{"a": "a", "b": "b"},
				}.Build(),
			}.Build(),
		}.Build(),
	}.Build())
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

func Test_validDNSSAN(t *testing.T) {
	tests := []struct {
		str  string
		want bool
	}{
		{
			str:  "example.com",
			want: true,
		},
		{
			// Single-label domain name. An unusual case but important since we
			// may be issuing certs for hostnames in a local network (which will
			// not abide by usual public internet dns semantics).
			str:  "example",
			want: true,
		},
		{
			// DNS 1123 permits numeric characters at start (unlike RFC 1035)
			str:  "123-example.com",
			want: true,
		},
		{
			str:  "*.example.com",
			want: true,
		},
		{
			str:  "*example.com",
			want: false,
		},
		{
			str:  ".example.com",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			require.Equal(t, tt.want, validDNSSAN(tt.str))
		})
	}
}
