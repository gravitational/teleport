package summarizerv1

import (
	"context"

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apisummarizer "github.com/gravitational/teleport/api/types/summarizer"
	summarizererrors "github.com/gravitational/teleport/e/lib/auth/summarizer/errors"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
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
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	if err := s.validateRetrievalModel(ctx, req.GetModel()); err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.CreateRetrievalModel(ctx, req.GetModel())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.emitCreateRetrievalModelEvent(ctx, authCtx, model)
	return pb.CreateRetrievalModelResponse_builder{Model: model}.Build(), nil
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
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	model, err := s.backend.GetRetrievalModel(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pb.GetRetrievalModelResponse_builder{Model: model}.Build(), nil
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
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	if err := s.validateRetrievalModel(ctx, req.GetModel()); err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.UpdateRetrievalModel(ctx, req.GetModel())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.emitUpdateRetrievalModelEvent(ctx, authCtx, model)
	return pb.UpdateRetrievalModelResponse_builder{Model: model}.Build(), nil
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
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	if err := s.validateRetrievalModel(ctx, req.GetModel()); err != nil {
		return nil, trace.Wrap(err)
	}

	exists := false
	if _, err := s.cache.GetRetrievalModel(ctx); err == nil {
		exists = true
	}

	model, err := s.backend.UpsertRetrievalModel(ctx, req.GetModel())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if exists {
		s.emitUpdateRetrievalModelEvent(ctx, authCtx, model)
	} else {
		s.emitCreateRetrievalModelEvent(ctx, authCtx, model)
	}

	return pb.UpsertRetrievalModelResponse_builder{Model: model}.Build(), nil
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
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	err = s.backend.DeleteRetrievalModel(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.emitDeleteRetrievalModelEvent(ctx, authCtx)
	return &pb.DeleteRetrievalModelResponse{}, nil
}

func (s *Service) emitCreateRetrievalModelEvent(ctx context.Context, authCtx *authz.Context, model *pb.RetrievalModel) {
	if err := s.emitter.EmitAuditEvent(ctx, &apievents.RetrievalModelCreate{
		Metadata:           apievents.Metadata{Type: events.RetrievalModelCreateEvent, Code: events.RetrievalModelCreateCode},
		UserMetadata:       authCtx.GetUserMetadata(),
		ResourceMetadata:   apievents.ResourceMetadata{Name: model.GetMetadata().GetName(), Expires: model.GetMetadata().GetExpires().AsTime()},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             apievents.Status{Success: true},
		Payload:            s.encodeResourcePayload(model),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit retrieval model create event", "error", err)
	}
}

func (s *Service) emitUpdateRetrievalModelEvent(ctx context.Context, authCtx *authz.Context, model *pb.RetrievalModel) {
	if err := s.emitter.EmitAuditEvent(ctx, &apievents.RetrievalModelUpdate{
		Metadata:           apievents.Metadata{Type: events.RetrievalModelUpdateEvent, Code: events.RetrievalModelUpdateCode},
		UserMetadata:       authCtx.GetUserMetadata(),
		ResourceMetadata:   apievents.ResourceMetadata{Name: model.GetMetadata().GetName(), Expires: model.GetMetadata().GetExpires().AsTime()},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             apievents.Status{Success: true},
		Payload:            s.encodeResourcePayload(model),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit retrieval model update event", "error", err)
	}
}

func (s *Service) emitDeleteRetrievalModelEvent(ctx context.Context, authCtx *authz.Context) {
	if err := s.emitter.EmitAuditEvent(ctx, &apievents.RetrievalModelDelete{
		Metadata:           apievents.Metadata{Type: events.RetrievalModelDeleteEvent, Code: events.RetrievalModelDeleteCode},
		UserMetadata:       authCtx.GetUserMetadata(),
		ResourceMetadata:   apievents.ResourceMetadata{Name: types.MetaNameRetrievalModel},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             apievents.Status{Success: true},
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit retrieval model delete event", "error", err)
	}
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

func (s *Service) TestRetrievalModel(
	ctx context.Context, req *pb.TestRetrievalModelRequest,
) (*pb.TestRetrievalModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err = authCtx.CheckAccessToKind(types.KindRetrievalModel, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	if resp := validateRetrievalTestResources(req); resp != nil {
		return resp, nil
	}

	provider, err := s.newTestEmbeddingsProvider(ctx, req)
	if err != nil {
		return pb.TestRetrievalModelResponse_builder{
			Success: false,
			Message: err.Error(),
		}.Build(), nil
	}

	if _, _, err = provider.GenerateEmbeddings(ctx, "test"); err != nil {
		return pb.TestRetrievalModelResponse_builder{
			Success: false,
			Message: summarizererrors.FormatRetrievalError(err, req.GetModel()),
		}.Build(), nil
	}

	return pb.TestRetrievalModelResponse_builder{
		Success: true,
		Message: "Successfully connected to the inference provider and received a response",
	}.Build(), nil
}

func validateRetrievalTestResources(req *pb.TestRetrievalModelRequest) *pb.TestRetrievalModelResponse {
	if req.GetModel() == nil {
		return pb.TestRetrievalModelResponse_builder{
			Success: false,
			Message: "model spec is required",
		}.Build()
	}
	testRetrievalModel := apisummarizer.NewRetrievalModel(req.GetModel())
	if err := apisummarizer.ValidateRetrievalModel(testRetrievalModel); err != nil {
		return pb.TestRetrievalModelResponse_builder{
			Success: false,
			Message: "invalid model spec: " + err.Error(),
		}.Build()
	}

	if secret := req.GetSecret(); secret != nil {
		testInferenceSecret := apisummarizer.NewInferenceSecret("test-secret", secret)
		if err := apisummarizer.ValidateInferenceSecret(testInferenceSecret); err != nil {
			return pb.TestRetrievalModelResponse_builder{
				Success: false,
				Message: "invalid secret spec: " + err.Error(),
			}.Build()
		}
	}
	return nil
}
