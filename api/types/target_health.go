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
	"iter"
	"time"
)

// TargetHealthProtocol is the network protocol for a health checker.
type TargetHealthProtocol string

const (
	// TargetHealthProtocolTCP is a target health check protocol.
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
	// TargetHealthStatusMixed indicates the resource has a mix of health
	// statuses. This can happen when multiple agents proxy the same resource.
	TargetHealthStatusMixed TargetHealthStatus = "mixed"
)

// Canonical converts a status into its canonical form.
// An empty or unknown status is converted to [TargetHealthStatusUnknown].
func (s TargetHealthStatus) Canonical() TargetHealthStatus {
	switch s {
	case TargetHealthStatusHealthy, TargetHealthStatusUnhealthy:
		return s
	default:
		return TargetHealthStatusUnknown
	}
}

// AggregateHealthStatus health statuses into a single status. If there are a
// mix of different statuses then the aggregate status is "mixed".
func AggregateHealthStatus(statuses iter.Seq[TargetHealthStatus]) TargetHealthStatus {
	first := true
	out := TargetHealthStatusUnknown
	for s := range statuses {
		if first {
			out = s.Canonical()
			first = false
		} else if out != s.Canonical() {
			return TargetHealthStatusMixed
		}
	}
	return out
}

// TargetHealthTransitionReason is the reason for the target health status
// transition.
type TargetHealthTransitionReason string

const (
	// TargetHealthTransitionReasonInit means that initial health checks are in
	// progress.
	TargetHealthTransitionReasonInit TargetHealthTransitionReason = "initialized"
	// TargetHealthStatusDisabled indicates that health checks are disabled.
	TargetHealthTransitionReasonDisabled TargetHealthTransitionReason = "disabled"
	// TargetHealthTransitionReasonThreshold means that the health status
	// changed because the healthy or unhealthy threshold was reached.
	TargetHealthTransitionReasonThreshold TargetHealthTransitionReason = "threshold_reached"
	// TargetHealthTransitionReasonInternalError indicates that health checks
	// encountered an internal error (this is a bug).
	TargetHealthTransitionReasonInternalError TargetHealthTransitionReason = "internal_error"
)

// GetTransitionTimestamp returns transition timestamp
func (t *TargetHealth) GetTransitionTimestamp() time.Time {
	if t.TransitionTimestamp == nil {
		return time.Time{}
	}
	return *t.TransitionTimestamp
}

// TargetHealthStatusGetter is a type that can return [TargetHealthStatus].
type TargetHealthStatusGetter interface {
	// GetTargetHealthStatus returns the target health status.
	GetTargetHealthStatus() TargetHealthStatus
}

// GroupByTargetHealthStatus groups resources by target health and returns [TargetHealthGroups].
func GroupByTargetHealthStatus[T TargetHealthStatusGetter](resources []T) TargetHealthGroups[T] {
	var groups TargetHealthGroups[T]
	for _, r := range resources {
		switch r.GetTargetHealthStatus() {
		case TargetHealthStatusHealthy:
			groups.Healthy = append(groups.Healthy, r)
		case TargetHealthStatusUnhealthy:
			groups.Unhealthy = append(groups.Unhealthy, r)
		default:
			// all other statuses are equivalent to unknown
			groups.Unknown = append(groups.Unknown, r)
		}
	}
	return groups
}

// TargetHealthGroups holds resources grouped by target health status.
type TargetHealthGroups[T TargetHealthStatusGetter] struct {
	// Healthy is the resources with [TargetHealthStatusHealthy].
	Healthy []T
	// Unhealthy is the resources with [TargetHealthStatusUnhealthy].
	Unhealthy []T
	// Unknown is the resources with any status that isn't healthy or unhealthy.
	// Namely [TargetHealthStatusUnknown], [TargetHealthStatusMixed], and the
	// empty string are grouped together.
	// Agents running with a version prior to health checks will always report
	// an empty health status.
	// A mixed status should only be set if health status for multiple servers
	// are aggregated. An aggregated mixed status is equivalent to "unknown"
	// because the underlying statuses that compose the mix are not known,
	// although it really doesn't make sense to aggregate the health status
	// before grouping it (please don't do that).
	Unknown []T
}
