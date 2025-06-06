/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package healthcheck

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport/api/defaults"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/healthcheckconfig"
	"github.com/gravitational/teleport/api/utils"
)

func Test_newHealthCheckConfig(t *testing.T) {
	t.Parallel()
	fullCfg, err := healthcheckconfig.NewHealthCheckConfig(
		"full",
		&healthcheckconfigv1.HealthCheckConfigSpec{
			Match: &healthcheckconfigv1.Matcher{
				DbLabels: []*labelv1.Label{{
					Name:   "foo",
					Values: []string{"bar", "baz"},
				}},
				DbLabelsExpression: `labels["qux"] == "*"`,
			},
			Timeout:            durationpb.New(time.Second * 42),
			Interval:           durationpb.New(time.Second * 43),
			HealthyThreshold:   7,
			UnhealthyThreshold: 8,
		},
	)
	require.NoError(t, err)

	minimalCfg, err := healthcheckconfig.NewHealthCheckConfig(
		"minimal",
		&healthcheckconfigv1.HealthCheckConfigSpec{
			Match: &healthcheckconfigv1.Matcher{
				DbLabelsExpression: `labels["*"] == "*"`,
			},
		},
	)
	require.NoError(t, err)

	tests := []struct {
		desc string
		cfg  *healthcheckconfigv1.HealthCheckConfig
		want *healthCheckConfig
	}{
		{
			desc: "copies all settings",
			cfg:  fullCfg,
			want: &healthCheckConfig{
				name:               "full",
				timeout:            time.Second * 42,
				interval:           time.Second * 43,
				healthyThreshold:   7,
				unhealthyThreshold: 8,
				protocol:           types.TargetHealthProtocolTCP,
				databaseLabelMatchers: types.LabelMatchers{
					Labels: types.Labels{
						"foo": utils.Strings{"bar", "baz"},
					},
					Expression: `labels["qux"] == "*"`,
				},
			},
		},
		{
			desc: "applies defaults",
			cfg:  minimalCfg,
			want: &healthCheckConfig{
				name:               "minimal",
				timeout:            defaults.HealthCheckTimeout,
				interval:           defaults.HealthCheckInterval,
				healthyThreshold:   defaults.HealthCheckHealthyThreshold,
				unhealthyThreshold: defaults.HealthCheckUnhealthyThreshold,
				protocol:           types.TargetHealthProtocolTCP,
				databaseLabelMatchers: types.LabelMatchers{
					Labels:     types.Labels{},
					Expression: `labels["*"] == "*"`,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got := newHealthCheckConfig(test.cfg)
			require.Equal(t, test.want, got)
		})
	}
}
func TestHealthCheckConfig_equivalent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc string
		a, b *healthCheckConfig
		want bool
	}{
		{
			desc: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			desc: "one nil, one non-nil",
			a:    &healthCheckConfig{},
			b:    nil,
			want: false,
		},
		{
			desc: "all fields equal",
			a: &healthCheckConfig{
				name:               "test",
				protocol:           "http",
				interval:           time.Second,
				healthyThreshold:   3,
				unhealthyThreshold: 5,
			},
			b: &healthCheckConfig{
				name:               "test",
				protocol:           "http",
				interval:           time.Second,
				healthyThreshold:   3,
				unhealthyThreshold: 5,
			},
			want: true,
		},
		{
			desc: "all fields equal ignoring labels",
			a: &healthCheckConfig{
				name:                  "test",
				protocol:              "http",
				interval:              time.Second,
				healthyThreshold:      3,
				unhealthyThreshold:    5,
				databaseLabelMatchers: types.LabelMatchers{Expression: "a", Labels: types.Labels{"a": {"a"}}},
			},
			b: &healthCheckConfig{
				name:                  "test",
				protocol:              "http",
				interval:              time.Second,
				healthyThreshold:      3,
				unhealthyThreshold:    5,
				databaseLabelMatchers: types.LabelMatchers{Expression: "b", Labels: types.Labels{"b": {"b"}}},
			},
			want: true,
		},
		{
			desc: "different name",
			a: &healthCheckConfig{
				name: "test1",
			},
			b: &healthCheckConfig{
				name: "test2",
			},
			want: false,
		},
		{
			desc: "different protocol",
			a: &healthCheckConfig{
				protocol: "http",
			},
			b: &healthCheckConfig{
				protocol: "tcp",
			},
			want: false,
		},
		{
			desc: "different interval",
			a: &healthCheckConfig{
				interval: time.Second,
			},
			b: &healthCheckConfig{
				interval: 2 * time.Second,
			},
			want: false,
		},
		{
			desc: "different healthyThreshold",
			a: &healthCheckConfig{
				healthyThreshold: 2,
			},
			b: &healthCheckConfig{
				healthyThreshold: 3,
			},
			want: false,
		},
		{
			desc: "different unhealthyThreshold",
			a: &healthCheckConfig{
				unhealthyThreshold: 2,
			},
			b: &healthCheckConfig{
				unhealthyThreshold: 3,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.want, tt.a.equivalent(tt.b))
		})
	}
}
