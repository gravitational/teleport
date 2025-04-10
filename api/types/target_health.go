/*
Copyright 2025 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"github.com/gravitational/teleport/api/types/compare"
	"github.com/gravitational/trace"
)

var _ compare.IsEqual[*TargetHealth] = (*TargetHealth)(nil)

// TargetHealthProtocol is the network protocol for a health checker.
type TargetHealthProtocol string

const (
	// TargetHealthProtocolTCP is a [TargetHealth] network protocol.
	TargetHealthProtocolTCP TargetHealthProtocol = "TCP"
)

// TargetHealthStatus is a target resource's health status.
type TargetHealthStatus string

const (
	// TargetHealthStatusHealthy indicates that a health check target is healthy.
	TargetHealthStatusHealthy TargetHealthStatus = "healthy"
	// TargetHealthStatusUnhealthy indicates that a health check target is unhealthy.
	TargetHealthStatusUnhealthy TargetHealthStatus = "unhealthy"
	// TargetHealthStatusUnknown indicates that an unknown health check target health status.
	TargetHealthStatusUnknown TargetHealthStatus = "unknown"
)

type TargetHealthTransitionReason string

const (
	// TargetHealthTransitionReasonInit means that initial health checks are in
	// progress.
	TargetHealthTransitionReasonInit TargetHealthTransitionReason = "initialized"
	// TargetHealthStatusDisabled indicates that health checks are disabled.
	TargetHealthTransitionReasonDisabled TargetHealthTransitionReason = "disabled"
	// TargetHealthTransitionReasonThreshold means that the health status
	// changed because the healthy or unhealthy threshold was reached.
	TargetHealthTransitionReasonThreshold TargetHealthTransitionReason = "threshold"
)

// IsEqual determines if two target health resources are equivalent to one another.
func (t *TargetHealth) IsEqual(other *TargetHealth) bool {
	return deriveTeleportEqualTargetHealth(t, other)
}

type targetHealthGetter interface {
	GetTargetHealth() TargetHealth
}

// GroupByTargetHealth groups the given resources by target health and returns
// the groups ordered by connection priority.
func GroupByTargetHealth[T targetHealthGetter](resources []T) [][]T {
	var (
		healthy, unhealthy, unknown []T
	)
	for _, r := range resources {
		switch TargetHealthStatus(r.GetTargetHealth().Status) {
		case TargetHealthStatusHealthy:
			healthy = append(healthy, r)
		case TargetHealthStatusUnhealthy:
			unhealthy = append(unhealthy, r)
		default:
			unknown = append(unknown, r)
		}
	}
	return [][]T{healthy, unknown, unhealthy}
}

func validHealthStatus(status string) bool {
	return status == string(TargetHealthStatusHealthy) ||
		status == string(TargetHealthStatusUnhealthy) ||
		status == string(TargetHealthStatusUnknown)
}

// ValidateHealthStatuses ensures given status string values
// are known/supported string values, else return error.
func ValidateHealthStatuses(statuses []string) error {
	for _, status := range statuses {
		if !validHealthStatus(status) {
			return trace.BadParameter("resource health status value %q is invalid", status)
		}
	}

	return nil
}

// MatchByUnknownStatus returns true if unknown has been requested and status
// equals unknown or equals empty string (which is also another form of unknown).
func MatchByUnknownStatus(gotStatus string, wantHealthStatusMap map[string]string) bool {
	if _, requestedUnknown := wantHealthStatusMap[string(TargetHealthStatusUnknown)]; !requestedUnknown {
		return false
	}
	return gotStatus == "" || gotStatus == string(TargetHealthStatusUnknown)
}
