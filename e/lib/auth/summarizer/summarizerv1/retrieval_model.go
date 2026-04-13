package summarizerv1

import (
	"context"

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
)

// CreateRetrievalModel creates the RetrievalModel.
func (s *Service) CreateRetrievalModel(
	ctx context.Context, req *pb.CreateRetrievalModelRequest,
) (*pb.CreateRetrievalModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindRetrievalModel, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.validateRetrievalModel(ctx, req.GetModel()); err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.CreateRetrievalModel(ctx, req.GetModel())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.CreateRetrievalModelResponse{Model: model}, nil
}

// GetRetrievalModel retrieves the existing RetrievalModel.
func (s *Service) GetRetrievalModel(
	ctx context.Context, req *pb.GetRetrievalModelRequest,
) (*pb.GetRetrievalModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindRetrievalModel, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.GetRetrievalModel(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.GetRetrievalModelResponse{Model: model}, nil
}

// UpdateRetrievalModel updates the existing RetrievalModel.
func (s *Service) UpdateRetrievalModel(
	ctx context.Context, req *pb.UpdateRetrievalModelRequest,
) (*pb.UpdateRetrievalModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindRetrievalModel, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.validateRetrievalModel(ctx, req.GetModel()); err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.UpdateRetrievalModel(ctx, req.GetModel())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.UpdateRetrievalModelResponse{Model: model}, nil
}

// UpsertRetrievalModel creates the RetrievalModel or updates an existing one.
func (s *Service) UpsertRetrievalModel(
	ctx context.Context, req *pb.UpsertRetrievalModelRequest,
) (*pb.UpsertRetrievalModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindRetrievalModel, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.validateRetrievalModel(ctx, req.GetModel()); err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.UpsertRetrievalModel(ctx, req.GetModel())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.UpsertRetrievalModelResponse{Model: model}, nil
}

// DeleteRetrievalModel deletes the existing RetrievalModel.
func (s *Service) DeleteRetrievalModel(
	ctx context.Context, req *pb.DeleteRetrievalModelRequest,
) (*pb.DeleteRetrievalModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindRetrievalModel, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.backend.DeleteRetrievalModel(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.DeleteRetrievalModelResponse{}, nil
}

func (s *Service) validateRetrievalModel(ctx context.Context, model *pb.RetrievalModel) error {
	if err := s.validateInferenceModelExistence(ctx, model.GetSpec().GetInferenceModelName()); err != nil {
		return trace.Wrap(err)
	}

	if model.GetSpec().GetBedrock() == nil {
		return nil
	}

	if !s.enableBedrockWithoutRestrictions {
		return trace.BadParameter("Bedrock-based retrieval models are not supported in Teleport Cloud. Please use the preconfigured retrieval models provided by Teleport Cloud.")
	}
	return nil
}

// validateInferenceModelExistence checks if the inference model exists in cache or backend.
func (s *Service) validateInferenceModelExistence(ctx context.Context, name string) error {
	// This is a best effort to check if the inference model referenced by the
	// inference policy or retrieval model exists.
	// This is just to avoid referencing a non-existing inference model while creating
	// or updating the inference policy or retrieval model. The actual existence of the inference model
	// will be enforced when the inference policy or retrieval model is used and the inference model is accessed.
	if _, err := s.cache.GetInferenceModel(ctx, name); trace.IsNotFound(err) {
		_, err := s.backend.GetInferenceModel(ctx, name)
		return trace.Wrap(err)
	} else if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
