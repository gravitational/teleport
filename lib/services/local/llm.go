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

package local

import (
	"context"
	"strings"

	"github.com/gravitational/trace"

	llmv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/llm/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const llmEndpointQuotaPrefix = "llm_endpoint_quota"

var _ (services.LLMEndpointQuotaService) = (*LLMEndpointQuotaService)(nil)

// LLMEndpointQuotaService implements the storage layer for the LLMEndpointQuota
// resource.
type LLMEndpointQuotaService struct {
	svc *generic.ServiceWrapper[*llmv1.LLMEndpointQuota]
}

// NewLLMEndpointQuotaService returns a new instance of [LLMEndpointQuotaService].
func NewLLMEndpointQuotaService(b backend.Backend) (*LLMEndpointQuotaService, error) {
	svc, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*llmv1.LLMEndpointQuota]{
			Backend:       b,
			ResourceKind:  types.KindLLMEndpointQuota,
			BackendPrefix: backend.NewKey(llmEndpointQuotaPrefix),
			MarshalFunc:   services.MarshalProtoResource[*llmv1.LLMEndpointQuota],
			UnmarshalFunc: services.UnmarshalProtoResource[*llmv1.LLMEndpointQuota],
			ValidateFunc:  services.ValidateLLMEndpointQuota,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &LLMEndpointQuotaService{
		svc: svc,
	}, nil
}

// CreateLLMEndpointQuota implements [services.LLMEndpointQuotaService].
func (l *LLMEndpointQuotaService) CreateLLMEndpointQuota(ctx context.Context, quota *llmv1.LLMEndpointQuota) (*llmv1.LLMEndpointQuota, error) {
	created, err := l.svc.CreateResource(ctx, quota)
	return created, trace.Wrap(err)
}

// DeleteLLMEndpointQuota implements [services.LLMEndpointQuotaService].
func (l *LLMEndpointQuotaService) DeleteLLMEndpointQuota(ctx context.Context, name string) error {
	return trace.Wrap(l.svc.DeleteResource(ctx, name))
}

// GetLLMEndpointQuota implements [services.LLMEndpointQuotaService].
func (l *LLMEndpointQuotaService) GetLLMEndpointQuota(ctx context.Context, name string) (*llmv1.LLMEndpointQuota, error) {
	quota, err := l.svc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return quota, nil
}

// ListLLMEndpointQuotas implements [services.LLMEndpointQuotaService].
func (l *LLMEndpointQuotaService) ListLLMEndpointQuotas(ctx context.Context, limit int, startKey string) ([]*llmv1.LLMEndpointQuota, string, error) {
	quotas, nextKey, err := l.svc.ListResources(ctx, limit, startKey)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return quotas, nextKey, nil
}

// UpdateLLMEndpointQuota implements [services.LLMEndpointQuotaService].
func (l *LLMEndpointQuotaService) UpdateLLMEndpointQuota(ctx context.Context, quota *llmv1.LLMEndpointQuota) (*llmv1.LLMEndpointQuota, error) {
	updated, err := l.svc.ConditionalUpdateResource(ctx, quota)
	return updated, trace.Wrap(err)
}

// UpsertLLMEndpointQuota implements [services.LLMEndpointQuotaService].
func (l *LLMEndpointQuotaService) UpsertLLMEndpointQuota(ctx context.Context, quota *llmv1.LLMEndpointQuota) (*llmv1.LLMEndpointQuota, error) {
	upserted, err := l.svc.UpsertResource(ctx, quota)
	return upserted, trace.Wrap(err)
}

// DeleteAllLLMEndpointQuotas removes all LLMEndpointQuotas resources.
func (s *LLMEndpointQuotaService) DeleteAllLLMEndpointQuotas(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}

func newLLMEndpointQuotaParser() *llmEndpointQuotaParser {
	return &llmEndpointQuotaParser{
		baseParser: newBaseParser(backend.NewKey(llmEndpointQuotaPrefix).ExactKey()),
	}
}

type llmEndpointQuotaParser struct {
	baseParser
}

func (p *llmEndpointQuotaParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(llmEndpointQuotaPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}

		return &types.ResourceHeader{
			Kind:    types.KindLLMEndpointQuota,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: strings.TrimPrefix(name, backend.SeparatorString),
			},
		}, nil
	case types.OpPut:
		resource, err := services.UnmarshalProtoResource[*llmv1.LLMEndpointQuota](
			event.Item.Value,
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshalling resource from event")
		}
		return types.Resource153ToLegacy(resource), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
