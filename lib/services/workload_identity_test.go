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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
)

func TestWorkloadIdentityMarshaling(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		in   *workloadidentityv1pb.WorkloadIdentity
	}{
		{
			name: "normal",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/example",
					}.Build(),
				}.Build(),
			}.Build(),
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

func TestWorkloadIdentityKey(t *testing.T) {
	t.Parallel()

	newWI := func(scope, name string) *workloadidentityv1pb.WorkloadIdentity {
		return workloadidentityv1pb.WorkloadIdentity_builder{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Scope:   scope,
			Metadata: headerv1.Metadata_builder{
				Name: name,
			}.Build(),
			Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
				Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
					Id: "/example",
				}.Build(),
			}.Build(),
		}.Build()
	}

	// A scope that cannot be encoded safely, as may be read from invalid
	// stored data. The key funcs degrade to the invalid-scope cursor for it.
	badScope := "/foo bar"

	testCases := []struct {
		name      string
		sortField WorkloadIdentitySortField
		in        *workloadidentityv1pb.WorkloadIdentity
		want      string
	}{
		{
			name:      "name - unscoped",
			sortField: WorkloadIdentitySortFieldName,
			in:        newWI("", "example"),
			want:      "example",
		},
		{
			name:      "name - scoped",
			sortField: WorkloadIdentitySortFieldName,
			in:        newWI("/aa/bb", "example"),
			want:      scopes.MakeResourceCursor("/aa/bb", "example"),
		},
		{
			name:      "name - invalid scope",
			sortField: WorkloadIdentitySortFieldName,
			in:        newWI(badScope, "example"),
			want:      scopes.MakeResourceCursor(badScope, "example"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keyFn, err := WorkloadIdentityKey(tc.sortField)
			require.NoError(t, err)
			require.Equal(t, tc.want, keyFn(tc.in))
		})
	}

	t.Run("spiffe_id - invalid scope", func(t *testing.T) {
		keyFn, err := WorkloadIdentityKey(WorkloadIdentitySortFieldSPIFFEID)
		require.NoError(t, err)
		// The spiffe_id key appends the resource cursor, so it degrades to the
		// invalid-scope cursor too.
		key := keyFn(newWI(badScope, "example"))
		require.True(t, strings.HasSuffix(key, "/"+scopes.MakeResourceCursor(badScope, "example")), "key: %q", key)
	})

	t.Run("unsupported sort field", func(t *testing.T) {
		_, err := WorkloadIdentityKey("bogus")
		require.Error(t, err)
	})
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
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "example",
										Eq: workloadidentityv1pb.WorkloadIdentityConditionEq_builder{
											Value: "foo",
										}.Build(),
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/example",
						X509: workloadidentityv1pb.WorkloadIdentitySPIFFEX509_builder{
							MaximumTtl: durationpb.New(time.Hour * 24 * 14),
						}.Build(),
						Jwt: workloadidentityv1pb.WorkloadIdentitySPIFFEJWT_builder{
							MaximumTtl: durationpb.New(time.Hour * 24),
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: require.NoError,
		},
		{
			name: "success - minimal",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/example",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: require.NoError,
		},
		{
			name: "missing name",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:     types.KindWorkloadIdentity,
				Version:  types.V1,
				Metadata: &headerv1.Metadata{},
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/example",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains("metadata.name: is required"),
		},
		{
			name: "missing spiffe id",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{},
				}.Build(),
			}.Build(),
			requireErr: errContains("spec.spiffe.id: is required"),
		},
		{
			name: "spiffe id must have leading /",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "example",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains("spec.spiffe.id: must start with a /"),
		},
		{
			name: "missing attribute",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "",
										Eq: workloadidentityv1pb.WorkloadIdentityConditionEq_builder{
											Value: "foo",
										}.Build(),
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/example",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains("spec.rules.allow[0].conditions[0].attribute: must be non-empty"),
		},
		{
			name: "missing operator",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "example",
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/example",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains("spec.rules.allow[0].conditions[0]: operator must be specified"),
		},
		{
			name: "expression and conditions",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Expression: `user.name == "Alan Partridge"`,
								Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
									workloadidentityv1pb.WorkloadIdentityCondition_builder{
										Attribute: "example",
										Eq: workloadidentityv1pb.WorkloadIdentityConditionEq_builder{
											Value: "foo",
										}.Build(),
									}.Build(),
								},
							}.Build(),
						},
					}.Build(),
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/example",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains("spec.rules.allow[0].conditions: is mutually exclusive with expression"),
		},
		{
			name: "neither expression or conditions",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							{}, // Empty rule.
						},
					}.Build(),
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/example",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains("spec.rules.allow[0].conditions: must be non-empty"),
		},
		{
			name: "invalid expression",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
						Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
							workloadidentityv1pb.WorkloadIdentityRule_builder{
								Expression: `does_not_exist`,
							}.Build(),
						},
					}.Build(),
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/example",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains(`unknown identifier: "does_not_exist"`),
		},
		{
			name: "maximum x509 ttl too large",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/example",
						X509: workloadidentityv1pb.WorkloadIdentitySPIFFEX509_builder{
							MaximumTtl: durationpb.New(time.Hour * 24 * 365),
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains(`spec.spiffe.x509.maximum_ttl: must be less than 336h0m0s`),
		},
		{
			name: "maximum jwt ttl too large",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/example",
						Jwt: workloadidentityv1pb.WorkloadIdentitySPIFFEJWT_builder{
							MaximumTtl: durationpb.New(time.Hour * 24 * 365),
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains(`spec.spiffe.jwt.maximum_ttl: must be less than 24h0m0s`),
		},
		{
			name: "scoped success",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Scope: "/security/eu",
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/security/eu/_/k8s/cluster-a",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: require.NoError,
		},
		{
			name: "scoped name must be valid segment",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example::",
				}.Build(),
				Scope: "/security/eu",
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/security/eu/_/k8s/cluster-a",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains("segment \"example::\" is malformed"),
		},
		{
			name: "scoped success with templated spiffe id",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Scope: "/security/eu",
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/security/eu/_/k8s/{{ workload.kubernetes.namespace }}/{{ workload.kubernetes.pod_name }}",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: require.NoError,
		},
		{
			name: "scoped templated spiffe id cannot escape scope prefix",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Scope: "/security",
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/{{ user.name }}/_/svc",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains("must be prefixed with the scope"),
		},
		{
			name: "scoped spiffe id not prefixed with scope",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Scope: "/security",
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/other/_/svc",
					}.Build(),
				}.Build(),
			}.Build(),
			requireErr: errContains("must be prefixed with the scope"),
		},
		{
			name: "scoped root scope rejected",
			in: workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example",
				}.Build(),
				Scope: "/",
				Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
					Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
						Id: "/_/svc",
					}.Build(),
				}.Build(),
			}.Build(),
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

	var errContains = func(contains string) require.ErrorAssertionFunc {
		return func(t require.TestingT, err error, msgAndArgs ...any) {
			require.ErrorContains(t, err, contains, msgAndArgs...)
		}
	}

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
			name:      "missing leading slash",
			scope:     "/security",
			id:        "security/_/foo",
			assertErr: errContains("must begin with a forward slash"),
		},
		{
			name:      "id not prefixed with scope",
			scope:     "/security",
			id:        "/other/_/foo",
			assertErr: errContains("must be prefixed with the scope"),
		},
		{
			name:      "segment-aware prefix - not a string prefix match",
			scope:     "/foo",
			id:        "/foo-buzz/_/svc",
			assertErr: errContains("must be prefixed with the scope"),
		},
		{
			name:      "scope section is ancestor of scope",
			scope:     "/foo/bar",
			id:        "/foo/_/svc",
			assertErr: errContains("must be prefixed with the scope"),
		},
		{
			name:      "scope section is descendant of scope",
			scope:     "/foo/bar",
			id:        "/foo/bar/baz/_/svc",
			assertErr: errContains("must be prefixed with the scope"),
		},
		{
			name:      "missing separator - too short",
			scope:     "/security",
			id:        "/security/foo",
			assertErr: errContains("is missing the"),
		},
		{
			name:      "missing separator - long enough",
			scope:     "/security",
			id:        "/security/foo/bar",
			assertErr: errContains("is missing the"),
		},
		{
			name:      "separator is last segment - no admin section",
			scope:     "/security/eu",
			id:        "/security/eu/_",
			assertErr: errContains("at least one segment after"),
		},
		{
			name:      "separator in admin section",
			scope:     "/security/eu",
			id:        "/security/eu/_/k8s/_/probes/liveness",
			assertErr: errContains("administratively-defined section"),
		},
		{
			name:      "trailing slash",
			scope:     "/security",
			id:        "/security/_/foo/",
			assertErr: errContains("empty path segments"),
		},
		{
			name:      "empty segment in admin section",
			scope:     "/security",
			id:        "/security/_/foo//bar",
			assertErr: errContains("empty path segments"),
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
			name:      "template cannot substitute for a scope segment",
			scope:     "/security",
			id:        "/{{ user.name }}/_/svc",
			assertErr: errContains("must be prefixed with the scope"),
		},
		{
			name:      "template cannot substitute for the separator",
			scope:     "/security",
			id:        "/security/{{ user.name }}/svc",
			assertErr: errContains("is missing the"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertErr(t, ValidateScopedSPIFFEID(tt.scope, tt.id))
		})
	}
}
