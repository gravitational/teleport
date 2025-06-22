package local

import (
	"context"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/trace"
)

type summarizerService struct {
	modelService  *generic.ServiceWrapper[*summarizerv1.SummarizationInferenceModel]
	secretService *generic.ServiceWrapper[*summarizerv1.SummarizationInferenceSecret]
	policyService *generic.ServiceWrapper[*summarizerv1.SummarizationInferencePolicy]
}

var _ services.Summarizer = (*summarizerService)(nil)

func (s *summarizerService) CreateSummarizationInferenceModel(ctx context.Context, model *summarizerv1.SummarizationInferenceModel) (*summarizerv1.SummarizationInferenceModel, error) {
	res, err := s.modelService.CreateResource(ctx, model)
	return res, trace.Wrap(err)
}

func (s *summarizerService) DeleteSummarizationInferenceModel(ctx context.Context, name string) error {
	return trace.Wrap(s.modelService.DeleteResource(ctx, name))
}

func (s *summarizerService) GetSummarizationInferenceModel(ctx context.Context, name string) (*summarizerv1.SummarizationInferenceModel, error) {
	res, err := s.modelService.GetResource(ctx, name)
	return res, trace.Wrap(err)
}

func (s *summarizerService) ListSummarizationInferenceModels(ctx context.Context, size int, pageToken string) ([]*summarizerv1.SummarizationInferenceModel, string, error) {
	res, nextToken, err := s.modelService.ListResources(ctx, size, pageToken)
	return res, nextToken, trace.Wrap(err)
}

func (s *summarizerService) UpdateSummarizationInferenceModel(ctx context.Context, model *summarizerv1.SummarizationInferenceModel) (*summarizerv1.SummarizationInferenceModel, error) {
	res, err := s.modelService.ConditionalUpdateResource(ctx, model)
	return res, trace.Wrap(err)
}

func (s *summarizerService) CreateSummarizationInferenceSecret(ctx context.Context, secret *summarizerv1.SummarizationInferenceSecret) (*summarizerv1.SummarizationInferenceSecret, error) {
	res, err := s.secretService.CreateResource(ctx, secret)
	return res, trace.Wrap(err)
}

func (s *summarizerService) DeleteSummarizationInferenceSecret(ctx context.Context, name string) error {
	return trace.Wrap(s.secretService.DeleteResource(ctx, name))
}

func (s *summarizerService) GetSummarizationInferenceSecret(ctx context.Context, name string) (*summarizerv1.SummarizationInferenceSecret, error) {
	res, err := s.secretService.GetResource(ctx, name)
	return res, trace.Wrap(err)
}

func (s *summarizerService) ListSummarizationInferenceSecrets(ctx context.Context, size int, pageToken string) ([]*summarizerv1.SummarizationInferenceSecret, string, error) {
	res, nextToken, err := s.secretService.ListResources(ctx, size, pageToken)
	return res, nextToken, trace.Wrap(err)
}

func (s *summarizerService) UpdateSummarizationInferenceSecret(ctx context.Context, secret *summarizerv1.SummarizationInferenceSecret) (*summarizerv1.SummarizationInferenceSecret, error) {
	res, err := s.secretService.ConditionalUpdateResource(ctx, secret)
	return res, trace.Wrap(err)
}

func (s *summarizerService) CreateSummarizationInferencePolicy(ctx context.Context, policy *summarizerv1.SummarizationInferencePolicy) (*summarizerv1.SummarizationInferencePolicy, error) {
	res, err := s.policyService.CreateResource(ctx, policy)
	return res, trace.Wrap(err)
}

func (s *summarizerService) DeleteSummarizationInferencePolicy(ctx context.Context, name string) error {
	return trace.Wrap(s.policyService.DeleteResource(ctx, name))
}

func (s *summarizerService) GetSummarizationInferencePolicy(ctx context.Context, name string) (*summarizerv1.SummarizationInferencePolicy, error) {
	res, err := s.policyService.GetResource(ctx, name)
	return res, trace.Wrap(err)
}

func (s *summarizerService) ListSummarizationInferencePolicies(ctx context.Context, size int, pageToken string) ([]*summarizerv1.SummarizationInferencePolicy, string, error) {
	res, nextToken, err := s.policyService.ListResources(ctx, size, pageToken)
	return res, nextToken, trace.Wrap(err)
}

func (s *summarizerService) UpdateSummarizationInferencePolicy(ctx context.Context, policy *summarizerv1.SummarizationInferencePolicy) (*summarizerv1.SummarizationInferencePolicy, error) {
	res, err := s.policyService.ConditionalUpdateResource(ctx, policy)
	return res, trace.Wrap(err)
}

const (
	summarizationInferenceModelPrefix  = "summarization_inference_models"
	summarizationInferenceSecretPrefix = "summarization_inference_secrets"
	summarizationInferencePolicyPrefix = "summarization_inference_policies"
)

func NewSummarizerService(b backend.Backend) (services.Summarizer, error) {
	modelService, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*summarizerv1.SummarizationInferenceModel]{
			Backend:       b,
			ResourceKind:  types.KindSummarizationInferenceModel,
			BackendPrefix: backend.NewKey(summarizationInferenceModelPrefix),
			MarshalFunc:   services.MarshalProtoResource[*summarizerv1.SummarizationInferenceModel],
			UnmarshalFunc: services.UnmarshalProtoResource[*summarizerv1.SummarizationInferenceModel],
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secretService, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*summarizerv1.SummarizationInferenceSecret]{
			Backend:       b,
			ResourceKind:  types.KindSummarizationInferenceSecret,
			BackendPrefix: backend.NewKey(summarizationInferenceSecretPrefix),
			MarshalFunc:   services.MarshalProtoResource[*summarizerv1.SummarizationInferenceSecret],
			UnmarshalFunc: services.UnmarshalProtoResource[*summarizerv1.SummarizationInferenceSecret],
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	policyService, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*summarizerv1.SummarizationInferencePolicy]{
			Backend:       b,
			ResourceKind:  types.KindSummarizationInferencePolicy,
			BackendPrefix: backend.NewKey(summarizationInferencePolicyPrefix),
			MarshalFunc:   services.MarshalProtoResource[*summarizerv1.SummarizationInferencePolicy],
			UnmarshalFunc: services.UnmarshalProtoResource[*summarizerv1.SummarizationInferencePolicy],
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &summarizerService{
		modelService:  modelService,
		secretService: secretService,
		policyService: policyService,
	}, nil
}
