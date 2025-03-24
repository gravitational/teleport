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

package services

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestWorkloadIdentityMarshaling(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		in   *workloadidentityv1pb.WorkloadIdentity
	}{
		{
			name: "normal",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotBytes, err := MarshalWorkloadIdentity(tc.in)
			require.NoError(t, err)
			// Test that unmarshaling gives us the same object
			got, err := UnmarshalWorkloadIdentity(gotBytes)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tc.in, got, protocmp.Transform()))
		})
	}
}

func TestValidateWorkloadIdentity(t *testing.T) {
	t.Parallel()

	var errContains = func(contains string) require.ErrorAssertionFunc {
		return func(t require.TestingT, err error, msgAndArgs ...interface{}) {
			require.ErrorContains(t, err, contains, msgAndArgs...)
		}
	}

	testCases := []struct {
		name       string
		in         *workloadidentityv1pb.WorkloadIdentity
		requireErr require.ErrorAssertionFunc
	}{
		{
			name: "success - full",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Rules: &workloadidentityv1pb.WorkloadIdentityRules{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									{
										Attribute: "example",
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
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			},
			requireErr: require.NoError,
		},
		{
			name: "success - minimal",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			},
			requireErr: require.NoError,
		},
		{
			name: "missing name",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:     types.KindWorkloadIdentity,
				Version:  types.V1,
				Metadata: &headerv1.Metadata{},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			},
			requireErr: errContains("metadata.name: is required"),
		},
		{
			name: "missing spiffe id",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{},
				},
			},
			requireErr: errContains("spec.spiffe.id: is required"),
		},
		{
			name: "spiffe id must have leading /",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "example",
					},
				},
			},
			requireErr: errContains("spec.spiffe.id: must start with a /"),
		},
		{
			name: "missing attribute",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Rules: &workloadidentityv1pb.WorkloadIdentityRules{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									{
										Attribute: "",
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
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			},
			requireErr: errContains("spec.rules.allow[0].conditions[0].attribute: must be non-empty"),
		},
		{
			name: "missing operator",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Rules: &workloadidentityv1pb.WorkloadIdentityRules{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									{
										Attribute: "example",
									},
								},
							},
						},
					},
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			},
			requireErr: errContains("spec.rules.allow[0].conditions[0]: operator must be specified"),
		},
		{
			name: "expression and conditions",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Rules: &workloadidentityv1pb.WorkloadIdentityRules{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							{
								Expression: `user.name == "Alan Partridge"`,
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									{
										Attribute: "example",
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
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			},
			requireErr: errContains("spec.rules.allow[0].conditions: is mutually exclusive with expression"),
		},
		{
			name: "neither expression or conditions",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Rules: &workloadidentityv1pb.WorkloadIdentityRules{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							{}, // Empty rule.
						},
					},
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			},
			requireErr: errContains("spec.rules.allow[0].conditions: must be non-empty"),
		},
		{
			name: "invalid expression",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Rules: &workloadidentityv1pb.WorkloadIdentityRules{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							{
								Expression: `does_not_exist`,
							},
						},
					},
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			},
			requireErr: errContains(`unknown identifier: "does_not_exist"`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWorkloadIdentity(tc.in)
			tc.requireErr(t, err)
		})
	}
}
