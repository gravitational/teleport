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
	"github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// SummarizerService implements the [services.Summarizer]
// interface and manages summarization configuration resources in the backend.
type SummarizerService struct {
	modelService  *generic.ServiceWrapper[*summarizerv1.InferenceModel]
	secretService *generic.ServiceWrapper[*summarizerv1.InferenceSecret]
	policyService *generic.ServiceWrapper[*summarizerv1.InferencePolicy]
}

var _ services.Summarizer = (*SummarizerService)(nil)

// CreateInferenceModel creates a new session summary inference model in the
// backend.
func (s *SummarizerService) CreateInferenceModel(
	ctx context.Context, model *summarizerv1.InferenceModel,
) (*summarizerv1.InferenceModel, error) {
	res, err := s.modelService.CreateResource(ctx, model)
	return res, trace.Wrap(err)
}

// DeleteInferenceModel deletes a session summary inference model from the
// backend by name.
func (s *SummarizerService) DeleteInferenceModel(
	ctx context.Context, name string,
) error {
	return trace.Wrap(s.modelService.DeleteResource(ctx, name))
}

// GetInferenceModel retrieves a session summary inference model from the
// backend by name.
func (s *SummarizerService) GetInferenceModel(
	ctx context.Context, name string,
) (*summarizerv1.InferenceModel, error) {
	res, err := s.modelService.GetResource(ctx, name)
	return res, trace.Wrap(err)
}

// ListInferenceModels lists session summary inference models in the backend
// with pagination support. Returns a slice of models and a next page token.
func (s *SummarizerService) ListInferenceModels(
	ctx context.Context, size int, pageToken string,
) ([]*summarizerv1.InferenceModel, string, error) {
	res, nextToken, err := s.modelService.ListResources(ctx, size, pageToken)
	return res, nextToken, trace.Wrap(err)
}

// UpdateInferenceModel updates an existing session summary inference model in
// the backend.
func (s *SummarizerService) UpdateInferenceModel(
	ctx context.Context, model *summarizerv1.InferenceModel,
) (*summarizerv1.InferenceModel, error) {
	res, err := s.modelService.ConditionalUpdateResource(ctx, model)
	return res, trace.Wrap(err)
}

// UpsertInferenceModel creates or updates a session summary inference model in
// the backend. If the model already exists, it will be updated.
func (s *SummarizerService) UpsertInferenceModel(
	ctx context.Context, model *summarizerv1.InferenceModel,
) (*summarizerv1.InferenceModel, error) {
	res, err := s.modelService.UpsertResource(ctx, model)
	return res, trace.Wrap(err)
}

// CreateInferenceSecret creates a new session summary inference secret in the
// backend. The returned object contains the secret value and should be handled
// with care.
func (s *SummarizerService) CreateInferenceSecret(
	ctx context.Context, secret *summarizerv1.InferenceSecret,
) (*summarizerv1.InferenceSecret, error) {
	res, err := s.secretService.CreateResource(ctx, secret)
	return res, trace.Wrap(err)
}

// DeleteInferenceSecret deletes a session summary inference secret from the
// backend by name.
func (s *SummarizerService) DeleteInferenceSecret(
	ctx context.Context, name string) error {
	return trace.Wrap(s.secretService.DeleteResource(ctx, name))
}

// GetInferenceSecret retrieves a session summary inference secret from the
// backend by name. The returned object contains the secret value and should be
// handled with care.
func (s *SummarizerService) GetInferenceSecret(
	ctx context.Context, name string,
) (*summarizerv1.InferenceSecret, error) {
	res, err := s.secretService.GetResource(ctx, name)
	return res, trace.Wrap(err)
}

// ListInferenceSecrets lists session summary inference secrets in the backend
// with pagination support. Returns a slice of secrets and a next page token.
// The returned objects contain the secret values and should be handled with
// care.
func (s *SummarizerService) ListInferenceSecrets(
	ctx context.Context, size int, pageToken string,
) ([]*summarizerv1.InferenceSecret, string, error) {
	res, nextToken, err := s.secretService.ListResources(ctx, size, pageToken)
	return res, nextToken, trace.Wrap(err)
}

// UpdateInferenceSecret updates an existing session summary inference secret
// in the backend. The returned object contains the secret value and should be
// handled with care.
func (s *SummarizerService) UpdateInferenceSecret(
	ctx context.Context, secret *summarizerv1.InferenceSecret,
) (*summarizerv1.InferenceSecret, error) {
	res, err := s.secretService.ConditionalUpdateResource(ctx, secret)
	return res, trace.Wrap(err)
}

// UpsertInferenceSecret creates or updates a session summary inference
// secretin the backend. If the secret already exists, it will be updated. The
// returned object contains the secret value and should be handled with care.
func (s *SummarizerService) UpsertInferenceSecret(
	ctx context.Context, secret *summarizerv1.InferenceSecret,
) (*summarizerv1.InferenceSecret, error) {
	res, err := s.secretService.UpsertResource(ctx, secret)
	return res, trace.Wrap(err)
}

// CreateInferencePolicy creates a new session summary inference policy in the
// backend.
func (s *SummarizerService) CreateInferencePolicy(
	ctx context.Context, policy *summarizerv1.InferencePolicy,
) (*summarizerv1.InferencePolicy, error) {
	res, err := s.policyService.CreateResource(ctx, policy)
	return res, trace.Wrap(err)
}

// DeleteInferencePolicy deletes a session summary inference policy from the
// backend by name.
func (s *SummarizerService) DeleteInferencePolicy(
	ctx context.Context, name string,
) error {
	return trace.Wrap(s.policyService.DeleteResource(ctx, name))
}

// GetInferencePolicy retrieves a session summary inference policy from the
// backend by name.
func (s *SummarizerService) GetInferencePolicy(
	ctx context.Context, name string,
) (*summarizerv1.InferencePolicy, error) {
	res, err := s.policyService.GetResource(ctx, name)
	return res, trace.Wrap(err)
}

// ListInferencePolicies lists session summary inference policies in the
// backend with pagination support. Returns a slice of policies and a next page
// token.
func (s *SummarizerService) ListInferencePolicies(
	ctx context.Context, size int, pageToken string,
) ([]*summarizerv1.InferencePolicy, string, error) {
	res, nextToken, err := s.policyService.ListResources(ctx, size, pageToken)
	return res, nextToken, trace.Wrap(err)
}

// UpdateInferencePolicy updates an existing session summary inference policy
// in the backend.
func (s *SummarizerService) UpdateInferencePolicy(
	ctx context.Context, policy *summarizerv1.InferencePolicy,
) (*summarizerv1.InferencePolicy, error) {
	res, err := s.policyService.ConditionalUpdateResource(ctx, policy)
	return res, trace.Wrap(err)
}

// UpsertInferencePolicy creates or updates a session summary inference policy
// in the backend. If the policy already exists, it will be updated.
func (s *SummarizerService) UpsertInferencePolicy(
	ctx context.Context, policy *summarizerv1.InferencePolicy,
) (*summarizerv1.InferencePolicy, error) {
	res, err := s.policyService.UpsertResource(ctx, policy)
	return res, trace.Wrap(err)
}

// AllInferencePolicies returns an iterator that retrieves all session summary
// inference policies from the backend, without pagination.
func (s *SummarizerService) AllInferencePolicies(
	ctx context.Context,
) iter.Seq2[*summarizerv1.InferencePolicy, error] {
	return s.policyService.Resources(ctx, "", "")
}

const (
	inferenceModelPrefix  = "inference_models"
	inferenceSecretPrefix = "inference_secrets"
	inferencePolicyPrefix = "inference_policies"
)

// NewSummarizerService returns a service that manages summarization
// configuration resources in the backend.
func NewSummarizerService(b backend.Backend) (*SummarizerService, error) {
	modelService, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*summarizerv1.InferenceModel]{
			Backend:       b,
			ResourceKind:  types.KindInferenceModel,
			BackendPrefix: backend.NewKey(inferenceModelPrefix),
			MarshalFunc:   services.MarshalProtoResource[*summarizerv1.InferenceModel],
			UnmarshalFunc: services.UnmarshalProtoResource[*summarizerv1.InferenceModel],
			ValidateFunc:  summarizer.ValidateInferenceModel,
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
			ValidateFunc:  summarizer.ValidateInferenceSecret,
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
			ValidateFunc:  services.ValidateInferencePolicy,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SummarizerService{
		modelService:  modelService,
		secretService: secretService,
		policyService: policyService,
	}, nil
}
