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

package llmv1

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	llmv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/llm/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

var _ (llmv1.UsageServiceServer) = (*UsageService)(nil)

// UsageServiceConfig holds configuration options for [UsageService].
type UsageServiceConfig struct {
	// Authorizer used by the service.
	Authorizer authz.Authorizer
	// Cache is the cache used to retrieve LLM endpoint quotas.
	Cache services.LLMEndpointQuotaGetter
	// Logger is the slog logger.
	Logger *slog.Logger
}

// UsageService implements [llmv1.UsageServiceServer] gRPC server.
type UsageService struct {
	llmv1.UnimplementedUsageServiceServer

	logger     *slog.Logger
	authorizer authz.Authorizer
	cache      services.LLMEndpointQuotaGetter
}

// NewUsageService creates a new [UsageService].
func NewUsageService(cfg UsageServiceConfig) (*UsageService, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required for llm usage service")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required for llm usage service")
	}

	logger := cmp.Or(cfg.Logger, slog.Default())
	return &UsageService{
		logger:     logger.With(teleport.ComponentKey, teleport.ComponentLLM),
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
	}, nil
}

// GetLLMEndpointQuota implements [llmv1.UsageServiceServer].
func (u *UsageService) GetLLMEndpointQuota(ctx context.Context, req *llmv1.GetLLMEndpointQuotaRequest) (*llmv1.GetLLMEndpointQuotaResponse, error) {
	if err := u.authorizeView(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	quota, err := u.cache.GetLLMEndpointQuota(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return llmv1.GetLLMEndpointQuotaResponse_builder{
		Quota: quota,
	}.Build(), trace.Wrap(err)
}

// ListLLMEndpointQuotas implements [llmv1.UsageServiceServer].
func (u *UsageService) ListLLMEndpointQuotas(ctx context.Context, req *llmv1.ListLLMEndpointQuotasRequest) (*llmv1.ListLLMEndpointQuotasResponse, error) {
	if err := u.authorizeView(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	pageSize := int(req.GetPageSize())
	if pageSize <= 0 || pageSize > apidefaults.DefaultChunkSize {
		pageSize = apidefaults.DefaultChunkSize
	}

	quotas, nextKey, err := u.cache.ListLLMEndpointQuotas(ctx, pageSize, req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return llmv1.ListLLMEndpointQuotasResponse_builder{
		Quotas:        quotas,
		NextPageToken: nextKey,
	}.Build(), nil
}

// ReportUsage implements [llmv1.UsageServiceServer].
func (u *UsageService) ReportUsage(ctx context.Context, _ *llmv1.ReportUsageRequest) (*llmv1.ReportUsageResponse, error) {
	if _, err := u.authorizeReport(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(gabrielcorado): finish implementing the report handling.
	return nil, trace.NotImplemented("not implemented")
}

func (u *UsageService) authorizeReport(ctx context.Context) (string, error) {
	authCtx, err := u.authorizer.Authorize(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	builtin, ok := authCtx.Identity.(authz.BuiltinRole)
	if !ok {
		return "", trace.AccessDenied("usage can only be reported by app services")
	}
	if !authCtx.Checker.HasRole(string(types.RoleApp)) {
		return "", trace.AccessDenied("usage can only be reported by app services")
	}
	return builtin.GetServerID(), nil
}

// authorizeView authorizes read operations on the resource, effectively meaning
// who can view current quota consumption.
func (u *UsageService) authorizeView(ctx context.Context) error {
	authCtx, err := u.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, ok := authCtx.Identity.(authz.BuiltinRole); ok {
		return trace.Wrap(authCtx.CheckAccessToKind(types.KindLLMEndpointQuota, types.VerbRead))
	}
	// TODO(gabrielcorado): implement specific role permissions to LLM quotas for users.
	if err := authCtx.CheckAccessToKind(types.KindAppServer, types.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
