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

package local

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// SummarizerResourcesService implements the [services.SummarizerResources]
// interface and manages summarization configuration resources in the backend.
type SummarizerResourcesService struct {
	modelService  *generic.ServiceWrapper[*summarizerv1.InferenceModel]
	secretService *generic.ServiceWrapper[*summarizerv1.InferenceSecret]
	policyService *generic.ServiceWrapper[*summarizerv1.InferencePolicy]
}

var _ services.SummarizerResources = (*SummarizerResourcesService)(nil)

func (s *SummarizerResourcesService) CreateInferenceModel(
	ctx context.Context, model *summarizerv1.InferenceModel,
) (*summarizerv1.InferenceModel, error) {
	res, err := s.modelService.CreateResource(ctx, model)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) DeleteInferenceModel(
	ctx context.Context, name string,
) error {
	return trace.Wrap(s.modelService.DeleteResource(ctx, name))
}

func (s *SummarizerResourcesService) GetInferenceModel(
	ctx context.Context, name string,
) (*summarizerv1.InferenceModel, error) {
	res, err := s.modelService.GetResource(ctx, name)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) ListInferenceModels(
	ctx context.Context, size int, pageToken string,
) ([]*summarizerv1.InferenceModel, string, error) {
	res, nextToken, err := s.modelService.ListResources(ctx, size, pageToken)
	return res, nextToken, trace.Wrap(err)
}

func (s *SummarizerResourcesService) UpdateInferenceModel(
	ctx context.Context, model *summarizerv1.InferenceModel,
) (*summarizerv1.InferenceModel, error) {
	res, err := s.modelService.ConditionalUpdateResource(ctx, model)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) UpsertInferenceModel(
	ctx context.Context, model *summarizerv1.InferenceModel,
) (*summarizerv1.InferenceModel, error) {
	res, err := s.modelService.UpsertResource(ctx, model)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) CreateInferenceSecret(
	ctx context.Context, secret *summarizerv1.InferenceSecret,
) (*summarizerv1.InferenceSecret, error) {
	res, err := s.secretService.CreateResource(ctx, secret)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) DeleteInferenceSecret(
	ctx context.Context, name string) error {
	return trace.Wrap(s.secretService.DeleteResource(ctx, name))
}

func (s *SummarizerResourcesService) GetInferenceSecret(
	ctx context.Context, name string,
) (*summarizerv1.InferenceSecret, error) {
	res, err := s.secretService.GetResource(ctx, name)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) ListInferenceSecrets(
	ctx context.Context, size int, pageToken string,
) ([]*summarizerv1.InferenceSecret, string, error) {
	res, nextToken, err := s.secretService.ListResources(ctx, size, pageToken)
	return res, nextToken, trace.Wrap(err)
}

func (s *SummarizerResourcesService) UpdateInferenceSecret(
	ctx context.Context, secret *summarizerv1.InferenceSecret,
) (*summarizerv1.InferenceSecret, error) {
	res, err := s.secretService.ConditionalUpdateResource(ctx, secret)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) UpsertInferenceSecret(
	ctx context.Context, secret *summarizerv1.InferenceSecret,
) (*summarizerv1.InferenceSecret, error) {
	res, err := s.secretService.UpsertResource(ctx, secret)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) CreateInferencePolicy(
	ctx context.Context, policy *summarizerv1.InferencePolicy,
) (*summarizerv1.InferencePolicy, error) {
	res, err := s.policyService.CreateResource(ctx, policy)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) DeleteInferencePolicy(
	ctx context.Context, name string,
) error {
	return trace.Wrap(s.policyService.DeleteResource(ctx, name))
}

func (s *SummarizerResourcesService) GetInferencePolicy(
	ctx context.Context, name string,
) (*summarizerv1.InferencePolicy, error) {
	res, err := s.policyService.GetResource(ctx, name)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) ListInferencePolicies(
	ctx context.Context, size int, pageToken string,
) ([]*summarizerv1.InferencePolicy, string, error) {
	res, nextToken, err := s.policyService.ListResources(ctx, size, pageToken)
	return res, nextToken, trace.Wrap(err)
}

func (s *SummarizerResourcesService) UpdateInferencePolicy(
	ctx context.Context, policy *summarizerv1.InferencePolicy,
) (*summarizerv1.InferencePolicy, error) {
	res, err := s.policyService.ConditionalUpdateResource(ctx, policy)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) UpsertInferencePolicy(
	ctx context.Context, policy *summarizerv1.InferencePolicy,
) (*summarizerv1.InferencePolicy, error) {
	res, err := s.policyService.UpsertResource(ctx, policy)
	return res, trace.Wrap(err)
}

func (s *SummarizerResourcesService) AllInferencePolicies(
	ctx context.Context,
) iter.Seq2[*summarizerv1.InferencePolicy, error] {
	return s.policyService.Resources(ctx, "", "")
}

const (
	inferenceModelPrefix  = "inference_models"
	inferenceSecretPrefix = "inference_secrets"
	inferencePolicyPrefix = "inference_policies"
)

// NewSummarizerResourcesService returns a service that manages summarization
// configuration resources in the backend.
func NewSummarizerResourcesService(b backend.Backend) (*SummarizerResourcesService, error) {
	modelService, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*summarizerv1.InferenceModel]{
			Backend:       b,
			ResourceKind:  types.KindInferenceModel,
			BackendPrefix: backend.NewKey(inferenceModelPrefix),
			MarshalFunc:   services.MarshalProtoResource[*summarizerv1.InferenceModel],
			UnmarshalFunc: services.UnmarshalProtoResource[*summarizerv1.InferenceModel],
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secretService, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*summarizerv1.InferenceSecret]{
			Backend:       b,
			ResourceKind:  types.KindInferenceSecret,
			BackendPrefix: backend.NewKey(inferenceSecretPrefix),
			MarshalFunc:   services.MarshalProtoResource[*summarizerv1.InferenceSecret],
			UnmarshalFunc: services.UnmarshalProtoResource[*summarizerv1.InferenceSecret],
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	policyService, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*summarizerv1.InferencePolicy]{
			Backend:       b,
			ResourceKind:  types.KindInferencePolicy,
			BackendPrefix: backend.NewKey(inferencePolicyPrefix),
			MarshalFunc:   services.MarshalProtoResource[*summarizerv1.InferencePolicy],
			UnmarshalFunc: services.UnmarshalProtoResource[*summarizerv1.InferencePolicy],
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SummarizerResourcesService{
		modelService:  modelService,
		secretService: secretService,
		policyService: policyService,
	}, nil
}
