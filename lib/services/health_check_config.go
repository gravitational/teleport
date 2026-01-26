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

package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
)

// HealthCheckConfigReader defines methods for reading health check config
// resources.
type HealthCheckConfigReader interface {
	// GetHealthCheckConfig fetches a health check config by name.
	GetHealthCheckConfig(ctx context.Context, name string) (*healthcheckconfigv1.HealthCheckConfig, error)
	// ListHealthCheckConfigs lists health check configs with pagination.
	ListHealthCheckConfigs(ctx context.Context, limit int, startKey string) ([]*healthcheckconfigv1.HealthCheckConfig, string, error)
}

// HealthCheckConfig is a service that manages
// [healthcheckconfigv1.HealthCheckConfig] resources.
type HealthCheckConfig interface {
	HealthCheckConfigReader

	// CreateHealthCheckConfig creates a new health check config.
	CreateHealthCheckConfig(ctx context.Context, in *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error)
	// UpdateHealthCheckConfig updates an existing health check config.
	UpdateHealthCheckConfig(ctx context.Context, in *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error)
	// UpsertHealthCheckConfig creates or updates a health check config.
	UpsertHealthCheckConfig(ctx context.Context, in *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error)
	// DeleteHealthCheckConfig deletes a health check config.
	DeleteHealthCheckConfig(ctx context.Context, name string) error
}

// ValidateHealthCheckConfig validates the given health check config.
func ValidateHealthCheckConfig(s *healthcheckconfigv1.HealthCheckConfig) error {
	switch {
	case s == nil:
		return trace.BadParameter("object must not be nil")
	case s.Version != types.V1:
		return trace.BadParameter("only version %q is supported, got %q", types.V1, s.Version)
	case s.Kind != types.KindHealthCheckConfig:
		return trace.BadParameter("kind must be %q, got %q", types.KindHealthCheckConfig, s.Kind)
	case s.Metadata == nil:
		return trace.BadParameter("metadata is missing")
	case s.Metadata.Name == "":
		return trace.BadParameter("metadata.name is missing")
	case s.Spec == nil:
		return trace.BadParameter("spec is missing")
	case s.Spec.Match == nil:
		return trace.BadParameter("spec.match is missing")
	}

	for _, label := range s.Spec.Match.DbLabels {
		if err := validateLabel(label); err != nil {
			return trace.BadParameter("invalid spec.db_labels: %v", err)
		}
	}
	if expr := s.Spec.Match.DbLabelsExpression; len(expr) > 0 {
		if _, err := parseLabelExpression(expr); err != nil {
			return trace.BadParameter("invalid spec.db_labels_expression: %v", err)
		}
	}

	for _, label := range s.Spec.Match.KubernetesLabels {
		if err := validateLabel(label); err != nil {
			return trace.BadParameter("invalid spec.kubernetes_labels: %v", err)
		}
	}
	if expr := s.Spec.Match.KubernetesLabelsExpression; len(expr) > 0 {
		if _, err := parseLabelExpression(expr); err != nil {
			return trace.BadParameter("invalid spec.kubernetes_labels_expression: %v", err)
		}
	}

	timeout := s.Spec.Timeout.AsDuration()
	switch {
	case timeout == 0:
		timeout = defaults.HealthCheckTimeout
	case timeout < constants.MinHealthCheckTimeout:
		return trace.BadParameter("spec.timeout must be at least %s", constants.MinHealthCheckTimeout)
	}

	interval := s.Spec.Interval.AsDuration()
	switch {
	case interval == 0:
		interval = defaults.HealthCheckInterval
	case interval < constants.MinHealthCheckInterval:
		return trace.BadParameter("spec.interval must be at least %s", constants.MinHealthCheckInterval)
	case interval > constants.MaxHealthCheckInterval:
		return trace.BadParameter("spec.interval must not be greater than %s", constants.MaxHealthCheckInterval)
	}

	if timeout > interval {
		if s.Spec.Timeout.AsDuration() == 0 {
			return trace.BadParameter("spec.interval (%s) must not be less than the default timeout (%s)", interval, defaults.HealthCheckTimeout)
		}
		if s.Spec.Interval.AsDuration() == 0 {
			return trace.BadParameter("spec.timeout (%s) must not be greater than the default interval (%s)", timeout, defaults.HealthCheckInterval)
		}
		return trace.BadParameter("spec.timeout (%s) must not be greater than spec.interval (%s)", timeout, interval)
	}

	if s.Spec.HealthyThreshold > constants.MaxHealthCheckHealthyThreshold {
		return trace.BadParameter(
			"spec.healthy_threshold (%v) must not be greater than %v",
			s.Spec.HealthyThreshold,
			constants.MaxHealthCheckHealthyThreshold,
		)
	}
	if s.Spec.UnhealthyThreshold > constants.MaxHealthCheckUnhealthyThreshold {
		return trace.BadParameter(
			"spec.unhealthy_threshold (%v) must not be greater than %v",
			s.Spec.UnhealthyThreshold,
			constants.MaxHealthCheckUnhealthyThreshold,
		)
	}
	return nil
}

func validateLabel(label *labelv1.Label) error {
	if label.Name == types.Wildcard {
		if len(label.Values) != 1 || label.Values[0] != types.Wildcard {
			return trace.BadParameter("selector *:%s is not supported, a wildcard label key may only be used with a wildcard label value", label.Values[0])
		}
	}
	return nil
}

// MarshalHealthCheckConfig marshals HealthCheckConfig resource to JSON.
func MarshalHealthCheckConfig(cfg *healthcheckconfigv1.HealthCheckConfig, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(cfg, opts...)
}

// UnmarshalHealthCheckConfig unmarshals the HealthCheckConfig resource.
func UnmarshalHealthCheckConfig(data []byte, opts ...MarshalOption) (*healthcheckconfigv1.HealthCheckConfig, error) {
	return UnmarshalProtoResource[*healthcheckconfigv1.HealthCheckConfig](data, opts...)
}
