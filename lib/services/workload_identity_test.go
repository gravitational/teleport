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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

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
		return func(t require.TestingT, err error, msgAndArgs ...any) {
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
						X509: &workloadidentityv1pb.WorkloadIdentitySPIFFEX509{
							MaximumTtl: durationpb.New(time.Hour * 24 * 14),
						},
						Jwt: &workloadidentityv1pb.WorkloadIdentitySPIFFEJWT{
							MaximumTtl: durationpb.New(time.Hour * 24),
						},
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
		{
			name: "maximum x509 ttl too large",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
						X509: &workloadidentityv1pb.WorkloadIdentitySPIFFEX509{
							MaximumTtl: durationpb.New(time.Hour * 24 * 365),
						},
					},
				},
			},
			requireErr: errContains(`spec.spiffe.x509.maximum_ttl: must be less than 336h0m0s`),
		},
		{
			name: "maximum jwt ttl too large",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
						Jwt: &workloadidentityv1pb.WorkloadIdentitySPIFFEJWT{
							MaximumTtl: durationpb.New(time.Hour * 24 * 365),
						},
					},
				},
			},
			requireErr: errContains(`spec.spiffe.jwt.maximum_ttl: must be less than 24h0m0s`),
		},
		{
			name: "scoped success",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Scope: "/security/eu",
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/security/eu/_/k8s/cluster-a",
					},
				},
			},
			requireErr: require.NoError,
		},
		{
			name: "scoped success with templated spiffe id",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Scope: "/security/eu",
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/security/eu/_/k8s/{{ workload.kubernetes.namespace }}/{{ workload.kubernetes.pod_name }}",
					},
				},
			},
			requireErr: require.NoError,
		},
		{
			name: "scoped templated spiffe id cannot escape scope prefix",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Scope: "/security",
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/{{ user.name }}/_/svc",
					},
				},
			},
			requireErr: errContains("must be prefixed with the scope"),
		},
		{
			name: "scoped spiffe id not prefixed with scope",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Scope: "/security",
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/other/_/svc",
					},
				},
			},
			requireErr: errContains("must be prefixed with the scope"),
		},
		{
			name: "scoped root scope rejected",
			in: &workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Scope: "/",
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/_/svc",
					},
				},
			},
			requireErr: errContains("must not be the root scope"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWorkloadIdentity(tc.in)
			tc.requireErr(t, err)
		})
	}
}

func TestValidateScopedSPIFFEID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		scope     string
		id        string
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "valid single segment scope",
			scope:     "/security",
			id:        "/security/_/foo-svc",
			assertErr: require.NoError,
		},
		{
			name:      "valid multi segment scope",
			scope:     "/security/eu",
			id:        "/security/eu/_/k8s/cluster-a/default/default",
			assertErr: require.NoError,
		},
		{
			name:      "valid multi segment admin section",
			scope:     "/security",
			id:        "/security/_/k8s/cluster-a",
			assertErr: require.NoError,
		},
		{
			name:  "missing leading slash",
			scope: "/security",
			id:    "security/_/foo",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "must begin with a forward slash")
			},
		},
		{
			name:  "id not prefixed with scope",
			scope: "/security",
			id:    "/other/_/foo",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "must be prefixed with the scope")
			},
		},
		{
			name:  "segment-aware prefix - not a string prefix match",
			scope: "/foo",
			id:    "/foo-buzz/_/svc",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "must be prefixed with the scope")
			},
		},
		{
			name:  "scope section is ancestor of scope",
			scope: "/foo/bar",
			id:    "/foo/_/svc",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "must be prefixed with the scope")
			},
		},
		{
			name:  "scope section is descendant of scope",
			scope: "/foo/bar",
			id:    "/foo/bar/baz/_/svc",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "separator segment immediately after the scope")
			},
		},
		{
			name:  "missing separator - too short",
			scope: "/security",
			id:    "/security/foo",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "followed by the")
			},
		},
		{
			name:  "missing separator - long enough",
			scope: "/security",
			id:    "/security/foo/bar",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "separator segment immediately after the scope")
			},
		},
		{
			name:  "separator is last segment - no admin section",
			scope: "/security/eu",
			id:    "/security/eu/_",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "followed by the")
			},
		},
		{
			name:  "separator in admin section",
			scope: "/security/eu",
			id:    "/security/eu/_/k8s/_/probes/liveness",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "administratively-defined section")
			},
		},
		// Templated SPIFFE IDs are validated in their unrendered form at
		// create/update time; the rendered form is re-validated at issuance.
		// Templates are only permitted in the administratively-defined section.
		{
			name:      "template in admin section",
			scope:     "/security/eu",
			id:        "/security/eu/_/k8s/{{ workload.kubernetes.namespace }}/{{ workload.kubernetes.pod_name }}",
			assertErr: require.NoError,
		},
		{
			name:      "template as sole admin segment",
			scope:     "/security",
			id:        "/security/_/{{ workload.kubernetes.pod_name }}",
			assertErr: require.NoError,
		},
		{
			name:  "template cannot substitute for a scope segment",
			scope: "/security",
			id:    "/{{ user.name }}/_/svc",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "must be prefixed with the scope")
			},
		},
		{
			name:  "template cannot substitute for the separator",
			scope: "/security",
			id:    "/security/{{ user.name }}/svc",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "separator segment immediately after the scope")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertErr(t, validateScopedSPIFFEID(tt.scope, tt.id))
		})
	}
}
