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
	modelService       *generic.ServiceWrapper[*summarizerv1.InferenceModel]
	secretService      *generic.ServiceWrapper[*summarizerv1.InferenceSecret]
	policyService      *generic.ServiceWrapper[*summarizerv1.InferencePolicy]
	searchModelService *generic.ServiceWrapper[*summarizerv1.SearchModel]
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

// DeleteAllInferenceModels deletes all session summary inference models from
// the backend. This should only be used by the cache.
func (s *SummarizerService) DeleteAllInferenceModels(ctx context.Context) error {
	return trace.Wrap(s.modelService.DeleteAllResources(ctx))
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
	ctx context.Context, name string,
) error {
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

// DeleteAllInferenceSecrets deletes all session summary inference secrets from
// the backend. This should only be used by the cache.
func (s *SummarizerService) DeleteAllInferenceSecrets(ctx context.Context) error {
	return trace.Wrap(s.secretService.DeleteAllResources(ctx))
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

// DeleteAllInferencePolicies deletes all session summary inference policies
// from the backend. This should only be used by the cache.
func (s *SummarizerService) DeleteAllInferencePolicies(ctx context.Context) error {
	return trace.Wrap(s.policyService.DeleteAllResources(ctx))
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

// CreateSearchModel creates the search model in the backend.
// Only one SearchModel can exist per cluster.
func (s *SummarizerService) CreateSearchModel(
	ctx context.Context, model *summarizerv1.SearchModel,
) (*summarizerv1.SearchModel, error) {
	res, err := s.searchModelService.CreateResource(ctx, model)
	return res, trace.Wrap(err)
}

// GetSearchModel retrieves the search model from the backend.
// Since only one SearchModel can exist per cluster, no name is required.
func (s *SummarizerService) GetSearchModel(
	ctx context.Context,
) (*summarizerv1.SearchModel, error) {
	res, err := s.searchModelService.GetResource(ctx, types.MetaNameSearchModel)
	return res, trace.Wrap(err)
}

// UpdateSearchModel updates the existing search model in the backend.
func (s *SummarizerService) UpdateSearchModel(
	ctx context.Context, model *summarizerv1.SearchModel,
) (*summarizerv1.SearchModel, error) {
	res, err := s.searchModelService.ConditionalUpdateResource(ctx, model)
	return res, trace.Wrap(err)
}

// UpsertSearchModel creates or updates the search model in the backend.
// If the model already exists, it will be updated.
func (s *SummarizerService) UpsertSearchModel(
	ctx context.Context, model *summarizerv1.SearchModel,
) (*summarizerv1.SearchModel, error) {
	res, err := s.searchModelService.UpsertResource(ctx, model)
	return res, trace.Wrap(err)
}

// DeleteSearchModel deletes the search model from the backend.
// Since only one SearchModel can exist per cluster, no name is required.
func (s *SummarizerService) DeleteSearchModel(ctx context.Context) error {
	return trace.Wrap(s.searchModelService.DeleteResource(ctx, types.MetaNameSearchModel))
}

const (
	inferenceModelPrefix  = "inference_models"
	inferenceSecretPrefix = "inference_secrets"
	inferencePolicyPrefix = "inference_policies"
	searchModelPrefix     = "search_model"
)

// SummarizerServiceConfig provides data necessary to initialize a
// [SummarizerService].
type SummarizerServiceConfig struct {
	// Backend is the resource storage backend.
	Backend backend.Backend
	// EnableBedrockWithoutRestrictions enables access to Amazon Bedrock models
	// without any restrictions. This should only be turned on outside Teleport
	// Cloud. Setting it to true allows creating inference_model resources that
	// use the Bedrock inference provider without going through OIDC. Setting it
	// to false means that only teleport-cloud-default model is authorized to use
	// Bedrock this way.
	EnableBedrockWithoutRestrictions bool
}

// NewSummarizerService returns a service that manages summarization
// configuration resources in the backend.
func NewSummarizerService(cfg SummarizerServiceConfig) (*SummarizerService, error) {
	validateInferenceModel := func(m *summarizerv1.InferenceModel) error {
		err := summarizer.ValidateInferenceModel(m)
		if err != nil {
			return trace.Wrap(err)
		}
		// If access to Bedrock is restricted, only models available via OIDC and
		// the special default cloud model are considered valid.
		if !cfg.EnableBedrockWithoutRestrictions &&
			m.GetSpec().GetBedrock() != nil &&
			m.GetSpec().GetBedrock().GetIntegration() == "" &&
			m.GetMetadata().GetName() != summarizer.CloudDefaultInferenceModelName {
			return trace.BadParameter("only the default model is allowed to use Amazon Bedrock without OIDC in Teleport Cloud")
		}
		return nil
	}

	modelService, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*summarizerv1.InferenceModel]{
			Backend:       cfg.Backend,
			ResourceKind:  types.KindInferenceModel,
			BackendPrefix: backend.NewKey(inferenceModelPrefix),
			MarshalFunc:   services.MarshalProtoResource[*summarizerv1.InferenceModel],
			UnmarshalFunc: services.UnmarshalProtoResource[*summarizerv1.InferenceModel],
			ValidateFunc:  validateInferenceModel,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secretService, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*summarizerv1.InferenceSecret]{
			Backend:       cfg.Backend,
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
			Backend:       cfg.Backend,
			ResourceKind:  types.KindInferencePolicy,
			BackendPrefix: backend.NewKey(inferencePolicyPrefix),
			MarshalFunc:   services.MarshalProtoResource[*summarizerv1.InferencePolicy],
			UnmarshalFunc: services.UnmarshalProtoResource[*summarizerv1.InferencePolicy],
			ValidateFunc:  services.ValidateInferencePolicy,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateSearchModel := func(m *summarizerv1.SearchModel) error {
		err := summarizer.ValidateSearchModel(m)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	searchModelService, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*summarizerv1.SearchModel]{
			Backend:       cfg.Backend,
			ResourceKind:  types.KindSearchModel,
			BackendPrefix: backend.NewKey(searchModelPrefix),
			MarshalFunc:   services.MarshalProtoResource[*summarizerv1.SearchModel],
			UnmarshalFunc: services.UnmarshalProtoResource[*summarizerv1.SearchModel],
			ValidateFunc:  validateSearchModel,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SummarizerService{
		modelService:       modelService,
		secretService:      secretService,
		policyService:      policyService,
		searchModelService: searchModelService,
	}, nil
}

// Parser implementations for event watching

func newInferenceModelParser() *inferenceModelParser {
	return &inferenceModelParser{
		baseParser: newBaseParser(backend.NewKey(inferenceModelPrefix)),
	}
}

type inferenceModelParser struct {
	baseParser
}

func (p *inferenceModelParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(inferenceModelPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}
		return &types.ResourceHeader{
			Kind:    types.KindInferenceModel,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: name,
			},
		}, nil
	case types.OpPut:
		model, err := services.UnmarshalProtoResource[*summarizerv1.InferenceModel](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(model), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newInferencePolicyParser() *inferencePolicyParser {
	return &inferencePolicyParser{
		baseParser: newBaseParser(backend.NewKey(inferencePolicyPrefix)),
	}
}

type inferencePolicyParser struct {
	baseParser
}

func (p *inferencePolicyParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(inferencePolicyPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}
		return &types.ResourceHeader{
			Kind:    types.KindInferencePolicy,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: name,
			},
		}, nil
	case types.OpPut:
		policy, err := services.UnmarshalProtoResource[*summarizerv1.InferencePolicy](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(policy), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newInferenceSecretParser() *inferenceSecretParser {
	return &inferenceSecretParser{
		baseParser: newBaseParser(backend.NewKey(inferenceSecretPrefix)),
	}
}

type inferenceSecretParser struct {
	baseParser
}

func (p *inferenceSecretParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(inferenceSecretPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}
		return &types.ResourceHeader{
			Kind:    types.KindInferenceSecret,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: name,
			},
		}, nil
	case types.OpPut:
		secret, err := services.UnmarshalProtoResource[*summarizerv1.InferenceSecret](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(secret), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newSearchModelParser() *searchModelParser {
	return &searchModelParser{
		baseParser: newBaseParser(backend.NewKey(searchModelPrefix)),
	}
}

type searchModelParser struct {
	baseParser
}

func (p *searchModelParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(searchModelPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}
		return &types.ResourceHeader{
			Kind:    types.KindSearchModel,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: name,
			},
		}, nil
	case types.OpPut:
		model, err := services.UnmarshalProtoResource[*summarizerv1.SearchModel](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(model), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
