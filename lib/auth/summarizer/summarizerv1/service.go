package summarizerv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

type SummarizerServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    services.Summarizer
	Logger     *slog.Logger
}

type SummarizerService struct {
	// Opt out of forward compatibility, fail fast if the proto changes.
	// pb.UnsafeSummarizerServiceServer
	pb.UnimplementedSummarizerServiceServer
	authorizer authz.Authorizer
	backend    services.Summarizer
	logger     *slog.Logger
}

func (s *SummarizerService) CreateSummarizationInferenceModel(
	ctx context.Context, req *pb.CreateSummarizationInferenceModelRequest,
) (*pb.SummarizationInferenceModel, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindSummarizationInferenceModel, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.CreateSummarizationInferenceModel(ctx, req.Model)
	return model, trace.Wrap(err)
}

func (s *SummarizerService) GetSummarizationInferenceModel(
	ctx context.Context, req *pb.GetSummarizationInferenceModelRequest,
) (*pb.SummarizationInferenceModel, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindSummarizationInferenceModel, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.GetSummarizationInferenceModel(ctx, req.Name)
	return model, trace.Wrap(err)
}

func (s *SummarizerService) ListSummarizationInferenceModels(
	ctx context.Context, req *pb.ListSummarizationInferenceModelsRequest,
) (*pb.ListSummarizationInferenceModelsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindSummarizationInferenceModel, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	models, nextPageToken, err :=
		s.backend.ListSummarizationInferenceModels(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.ListSummarizationInferenceModelsResponse{
		Models:        models,
		NextPageToken: nextPageToken,
	}, nil
}

func NewSummarizerService(cfg SummarizerServiceConfig) (*SummarizerService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend service is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "summarizer")
	}

	return &SummarizerService{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		logger:     cfg.Logger,
	}, nil
}
