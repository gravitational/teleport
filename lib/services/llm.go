// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"context"
	"slices"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	llmv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/llm/v1"
	"github.com/gravitational/teleport/api/types"
)

// LLMEndpointQuotaGetter defines interface for getting LLM endpoint quotas.
type LLMEndpointQuotaGetter interface {
	// GetLLMEndpointQuota returns the specified LLMEndpointQuota.
	GetLLMEndpointQuota(ctx context.Context, name string) (*llmv1.LLMEndpointQuota, error)
	// ListLLMEndpointQuotas lists LLMEndpointQuota resources.
	ListLLMEndpointQuotas(ctx context.Context, limit int, startKey string) ([]*llmv1.LLMEndpointQuota, string, error)
}

// LLMEndpointQuotaWriter defines interface for writing LLM endpoint quotas.
type LLMEndpointQuotaWriter interface {
	// CreateLLMEndpointQuota creates a new LLMEndpointQuota.
	CreateLLMEndpointQuota(ctx context.Context, quota *llmv1.LLMEndpointQuota) (*llmv1.LLMEndpointQuota, error)
	// UpdateLLMEndpointQuota updates an existing LLMEndpointQuota.
	UpdateLLMEndpointQuota(ctx context.Context, quota *llmv1.LLMEndpointQuota) (*llmv1.LLMEndpointQuota, error)
	// UpsertLLMEndpointQuota creates or replaces a LLMEndpointQuota.
	UpsertLLMEndpointQuota(ctx context.Context, quota *llmv1.LLMEndpointQuota) (*llmv1.LLMEndpointQuota, error)
	// DeleteLLMEndpointQuota deletes the specified LLMEndpointQuota.
	DeleteLLMEndpointQuota(ctx context.Context, name string) error
	// CreateOrUpdateLLMEndpointQuota creates or updates a LLMEndpointQuota.
	CreateOrUpdateLLMEndpointQuota(ctx context.Context, quota *llmv1.LLMEndpointQuota) (*llmv1.LLMEndpointQuota, error)
}

// LLMUsageReporter defines interface used to report LLM usage.
type LLMUsageReporter interface {
	// ReportUsage reports an amount of LLM tokens consumed.
	ReportUsage(ctx context.Context, usages []*llmv1.EndpointUsage) (*llmv1.ReportUsageResponse, error)
}

// LLMEndpointQuotaService defines interface to manage LLM endpoint quotas.
type LLMEndpointQuotaService interface {
	LLMEndpointQuotaGetter
	LLMEndpointQuotaWriter
}

// ValidateLLMEndpointQuota validates the given LLM endpoint quota.
func ValidateLLMEndpointQuota(quota *llmv1.LLMEndpointQuota) error {
	switch {
	case quota.GetKind() != types.KindLLMEndpointQuota:
		return trace.BadParameter("kind must be %q", types.KindLLMEndpointQuota)
	case quota.GetVersion() != types.V1:
		return trace.BadParameter("version must be %q", types.V1)
	case quota.GetMetadata() == nil:
		return trace.BadParameter("llm_endpoint_quota must have metadata")
	case quota.GetMetadata().GetName() == "":
		return trace.BadParameter("llm_endpoint_quota must set a non-empty metadata.name")
	case quota.GetMetadata().GetExpires() != nil:
		return trace.BadParameter("llm_endpoint_quota must not have an expiry")
	}

	for _, window := range quota.GetStatus().GetActiveWindows() {
		if !slices.Contains(types.SupportedAppLLMQuotaIntervals(), window.GetIntervalName()) {
			return trace.BadParameter("llm_endpoint_quota active window has unsupported %q interval name", window.GetIntervalName())
		}

		if window.GetIntervalName() == types.AppLLMQuotaIntervalLifetime {
			if window.GetStartsAt() != nil || window.GetEndsAt() != nil {
				return trace.BadParameter("llm_endpoint_quota lifetime active window must not define starts_at and ends_at fields")
			}

			continue
		}

		if window.GetStartsAt() == nil || window.GetEndsAt() == nil {
			return trace.BadParameter("llm_endpoint_quota active window doesn't define starts_at and ends_at")
		}

		if window.GetStartsAt().AsTime().After(window.GetEndsAt().AsTime()) {
			return trace.BadParameter("llm_endpoint_quota active window starts_at must be be before ends_at")
		}
	}

	return nil
}

// NewLLMEndpointQuota creates a [LLMEndpointQuota] resource with defined status.
func NewLLMEndpointQuota(name string, status *llmv1.LLMEndpointQuotaStatus) *llmv1.LLMEndpointQuota {
	return llmv1.LLMEndpointQuota_builder{
		Kind:    types.KindLLMEndpointQuota,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: name,
		}.Build(),
		Status: status,
	}.Build()
}
