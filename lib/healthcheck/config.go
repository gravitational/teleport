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
	"cmp"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	apiutils "github.com/gravitational/teleport/api/utils"
)

// healthCheckConfig is an internal health check config type converted from a
// [*healthcheckconfigv1.HealthCheckConfig] with defaults set.
type healthCheckConfig struct {
	name                  string
	protocol              types.TargetHealthProtocol
	interval              time.Duration
	timeout               time.Duration
	healthyThreshold      uint32
	unhealthyThreshold    uint32
	databaseLabelMatchers types.LabelMatchers
}

// newHealthCheckConfig converts a health check config protobuf message into a
// [healthCheckConfig] and sets defaults.
func newHealthCheckConfig(cfg *healthcheckconfigv1.HealthCheckConfig) *healthCheckConfig {
	spec := cfg.GetSpec()
	match := spec.GetMatch()
	return &healthCheckConfig{
		name:               cfg.GetMetadata().GetName(),
		timeout:            cmp.Or(spec.GetTimeout().AsDuration(), defaults.HealthCheckTimeout),
		interval:           cmp.Or(spec.GetInterval().AsDuration(), defaults.HealthCheckInterval),
		healthyThreshold:   cmp.Or(spec.GetHealthyThreshold(), defaults.HealthCheckHealthyThreshold),
		unhealthyThreshold: cmp.Or(spec.GetUnhealthyThreshold(), defaults.HealthCheckUnhealthyThreshold),
		// we only support plain TCP health checks currently, but eventually we
		// may add support for other protocols such as TLS or HTTP
		protocol:              types.TargetHealthProtocolTCP,
		databaseLabelMatchers: newLabelMatchers(match.GetDbLabelsExpression(), match.GetDbLabels()),
	}
}

// equivalent returns whether the config is equivalent to another, where
// equivalence is defined as equality in all fields except the matchers.
func (h *healthCheckConfig) equivalent(other *healthCheckConfig) bool {
	return (h == nil && other == nil) ||
		h != nil && other != nil &&
			h.name == other.name &&
			h.protocol == other.protocol &&
			h.interval == other.interval &&
			h.healthyThreshold == other.healthyThreshold &&
			h.unhealthyThreshold == other.unhealthyThreshold
}

// getLabelMatchers returns the label matchers to use for the given resource
// kind.
func (h *healthCheckConfig) getLabelMatchers(kind string) types.LabelMatchers {
	switch kind {
	case types.KindDatabase:
		return h.databaseLabelMatchers
	}
	// unreachable since we enforce a list of supported target resource kinds,
	// but empty matchers do the right thing anyway: don't match anything.
	return types.LabelMatchers{}
}

// newLabelMatchers creates a new [types.LabelMatchers] from an expression and
// r153 labels.
func newLabelMatchers(expr string, labels []*labelv1.Label) types.LabelMatchers {
	out := types.LabelMatchers{
		Expression: expr,
		Labels:     make(types.Labels),
	}
	for k, vs := range label.ToMap(labels) {
		out.Labels[k] = apiutils.Strings(vs)
	}
	return out
}
