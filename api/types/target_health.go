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
)

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
