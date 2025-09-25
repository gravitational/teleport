// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestValidateHealthCheckConfig(t *testing.T) {
	t.Parallel()

	var errContains = func(substr string) require.ErrorAssertionFunc {
		return func(t require.TestingT, err error, _ ...any) {
			t.(*testing.T).Helper()
			require.ErrorContains(t, err, substr)
		}
	}

	testCases := []struct {
		name       string
		in         *healthcheckconfigv1.HealthCheckConfig
		requireErr require.ErrorAssertionFunc
	}{
		{
			name: "default is valid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "default",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "*",
							Values: []string{"*"},
						}},
					},
				},
			},
			requireErr: require.NoError,
		},
		{
			name: "valid custom",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
						DbLabelsExpression: "labels.env == `prod`",
					},
					Timeout:  durationpb.New(1 * time.Second),
					Interval: durationpb.New(30 * time.Second),
				},
			},
			requireErr: require.NoError,
		},
		{
			name:       "nil object is invalid",
			in:         nil,
			requireErr: errContains("object must not be nil"),
		},
		{
			name: "unknown version is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: "v999",
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
					},
					Timeout:  durationpb.New(time.Second),
					Interval: durationpb.New(2 * time.Second),
				},
			},
			requireErr: errContains(`only version "v1" is supported`),
		},
		{
			name: "mismatched kind is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindUser,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
					},
					Timeout:  durationpb.New(time.Second),
					Interval: durationpb.New(2 * time.Second),
				},
			},
			requireErr: errContains(`kind must be "health_check_config"`),
		},
		{
			name: "missing metadata is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
					},
					Timeout:  durationpb.New(time.Second),
					Interval: durationpb.New(2 * time.Second),
				},
			},
			requireErr: errContains("metadata is missing"),
		},
		{
			name: "missing name is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
					},
					Timeout:  durationpb.New(time.Second),
					Interval: durationpb.New(2 * time.Second),
				},
			},
			requireErr: errContains("metadata.name is missing"),
		},
		{
			name: "missing spec is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: nil,
			},
			requireErr: errContains("spec is missing"),
		},
		{
			name: "missing match section is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match:    nil,
					Timeout:  durationpb.New(time.Second),
					Interval: durationpb.New(2 * time.Second),
				},
			},
			requireErr: errContains("spec.match is missing"),
		},
		{
			name: "invalid label matcher",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   types.Wildcard,
							Values: []string{"asdf"},
						}},
					},
					Timeout:  durationpb.New(time.Second),
					Interval: durationpb.New(2 * time.Second),
				},
			},
			requireErr: errContains("invalid spec.db_labels: selector *:asdf is not supported, a wildcard label key may only be used with a wildcard label value"),
		},
		{
			name: "invalid label expression matcher",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabelsExpression: "abc",
					},
					Timeout:  durationpb.New(time.Second),
					Interval: durationpb.New(2 * time.Second),
				},
			},
			requireErr: errContains("invalid spec.db_labels_expression: parsing label expression"),
		},
		{
			name: "timeout less than minimum is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
					},
					Timeout:  durationpb.New(500 * time.Millisecond),
					Interval: durationpb.New(5 * time.Second),
				},
			},
			requireErr: errContains("spec.timeout must be at least"),
		},
		{
			name: "interval less than minimum is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
					},
					Timeout:  durationpb.New(time.Second),
					Interval: durationpb.New(250 * time.Millisecond),
				},
			},
			requireErr: errContains("spec.interval must be at least"),
		},
		{
			name: "timeout greater than interval is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
					},
					Timeout:  durationpb.New(31 * time.Second),
					Interval: durationpb.New(30 * time.Second),
				},
			},
			requireErr: errContains("spec.timeout (31s) must not be greater than spec.interval (30s)"),
		},
		{
			name: "timeout greater than default interval is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
					},
					Timeout:  durationpb.New(31 * time.Second),
					Interval: nil,
				},
			},
			requireErr: errContains("spec.timeout (31s) must not be greater than the default interval (30s)"),
		},
		{
			name: "interval greater than maximum is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
					},
					Timeout:  durationpb.New(time.Second),
					Interval: durationpb.New(601 * time.Second),
				},
			},
			requireErr: errContains("spec.interval must not be greater than 10m0s"),
		},
		{
			name: "healthy threshold greater than maximum is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
					},
					Timeout:          durationpb.New(time.Second),
					Interval:         durationpb.New(30 * time.Second),
					HealthyThreshold: 11,
				},
			},
			requireErr: errContains("spec.healthy_threshold (11) must not be greater than 10"),
		},
		{
			name: "unhealthy threshold greater than maximum is invalid",
			in: &healthcheckconfigv1.HealthCheckConfig{
				Version: types.V1,
				Kind:    types.KindHealthCheckConfig,
				Metadata: &headerv1.Metadata{
					Name: "example",
				},
				Spec: &healthcheckconfigv1.HealthCheckConfigSpec{
					Match: &healthcheckconfigv1.Matcher{
						DbLabels: []*labelv1.Label{{
							Name:   "env",
							Values: []string{"prod"},
						}},
					},
					Timeout:            durationpb.New(time.Second),
					Interval:           durationpb.New(30 * time.Second),
					UnhealthyThreshold: 11,
				},
			},
			requireErr: errContains("spec.unhealthy_threshold (11) must not be greater than 10"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateHealthCheckConfig(tc.in)
			tc.requireErr(t, err)
		})
	}
}
