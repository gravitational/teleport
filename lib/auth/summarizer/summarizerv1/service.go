package summarizerv1

import (
	"context"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"
)

// NewSummarizerService creates a new OSS version of the SummarizerService. It
// returns a licensing error from every RPC.
func NewSummarizerService() *ossSummarizerService {
	return &ossSummarizerService{}
}

// ossSummarizerService is an OSS version of the SummarizerService. It returns
// a licensing error from every RPC.
type ossSummarizerService struct {
	pb.UnimplementedSummarizerServiceServer
}

// CRUD operations for models

func (s *ossSummarizerService) CreateSummarizationInferenceModel(
	ctx context.Context, req *pb.CreateSummarizationInferenceModelRequest,
) (*pb.SummarizationInferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) GetSummarizationInferenceModel(
	ctx context.Context, req *pb.GetSummarizationInferenceModelRequest,
) (*pb.SummarizationInferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) UpdateSummarizationInferenceModel(
	ctx context.Context, req *pb.UpdateSummarizationInferenceModelRequest,
) (*pb.SummarizationInferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) UpsertSummarizationInferenceModel(
	ctx context.Context, req *pb.UpsertSummarizationInferenceModelRequest,
) (*pb.SummarizationInferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) DeleteSummarizationInferenceModel(
	ctx context.Context, req *pb.DeleteSummarizationInferenceModelRequest,
) (*emptypb.Empty, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) ListSummarizationInferenceModels(
	ctx context.Context, req *pb.ListSummarizationInferenceModelsRequest,
) (*pb.ListSummarizationInferenceModelsResponse, error) {
	return nil, s.requireEnterprise()
}

// CRUD operations for secrets

func (s *ossSummarizerService) CreateSummarizationInferenceSecret(
	ctx context.Context, req *pb.CreateSummarizationInferenceSecretRequest,
) (*pb.SummarizationInferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) GetSummarizationInferenceSecret(
	ctx context.Context, req *pb.GetSummarizationInferenceSecretRequest,
) (*pb.SummarizationInferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) UpdateSummarizationInferenceSecret(
	ctx context.Context, req *pb.UpdateSummarizationInferenceSecretRequest,
) (*pb.SummarizationInferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) UpsertSummarizationInferenceSecret(
	ctx context.Context, req *pb.UpsertSummarizationInferenceSecretRequest,
) (*pb.SummarizationInferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) DeleteSummarizationInferenceSecret(
	ctx context.Context, req *pb.DeleteSummarizationInferenceSecretRequest,
) (*emptypb.Empty, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) ListSummarizationInferenceSecrets(
	ctx context.Context, req *pb.ListSummarizationInferenceSecretsRequest,
) (*pb.ListSummarizationInferenceSecretsResponse, error) {
	return nil, s.requireEnterprise()
}

// CRUD operations for policies

func (s *ossSummarizerService) CreateSummarizationInferencePolicy(
	ctx context.Context, req *pb.CreateSummarizationInferencePolicyRequest,
) (*pb.SummarizationInferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) GetSummarizationInferencePolicy(
	ctx context.Context, req *pb.GetSummarizationInferencePolicyRequest,
) (*pb.SummarizationInferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) UpdateSummarizationInferencePolicy(
	ctx context.Context, req *pb.UpdateSummarizationInferencePolicyRequest,
) (*pb.SummarizationInferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) UpsertSummarizationInferencePolicy(
	ctx context.Context, req *pb.UpsertSummarizationInferencePolicyRequest,
) (*pb.SummarizationInferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) DeleteSummarizationInferencePolicy(
	ctx context.Context, req *pb.DeleteSummarizationInferencePolicyRequest,
) (*emptypb.Empty, error) {
	return nil, s.requireEnterprise()
}

func (s *ossSummarizerService) ListSummarizationInferencePolicies(
	ctx context.Context, req *pb.ListSummarizationInferencePoliciesRequest,
) (*pb.ListSummarizationInferencePoliciesResponse, error) {
	return nil, s.requireEnterprise()
}

func (*ossSummarizerService) requireEnterprise() error {
	return trace.AccessDenied("session recording summarization is only available with an enterprise license")
}
