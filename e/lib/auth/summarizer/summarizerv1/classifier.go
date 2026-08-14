package summarizerv1

import (
	"context"

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
)

// CreateClassifier creates a new Classifier.
func (s *Service) CreateClassifier(
	ctx context.Context, req *pb.CreateClassifierRequest,
) (*pb.CreateClassifierResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindClassifier, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	classifier, err := s.backend.CreateClassifier(ctx, req.GetClassifier())
	if err != nil {
		s.emitCreateClassifierEvent(ctx, authCtx, req.GetClassifier(), err)
		return nil, trace.Wrap(err)
	}

	s.emitCreateClassifierEvent(ctx, authCtx, classifier, nil)
	return pb.CreateClassifierResponse_builder{Classifier: classifier}.Build(), nil
}

// GetClassifier retrieves an existing Classifier by name.
func (s *Service) GetClassifier(
	ctx context.Context, req *pb.GetClassifierRequest,
) (*pb.GetClassifierResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindClassifier, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	classifier, err := s.backend.GetClassifier(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pb.GetClassifierResponse_builder{Classifier: classifier}.Build(), nil
}

// UpdateClassifier updates an existing Classifier.
func (s *Service) UpdateClassifier(
	ctx context.Context, req *pb.UpdateClassifierRequest,
) (*pb.UpdateClassifierResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindClassifier, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	classifier, err := s.backend.UpdateClassifier(ctx, req.GetClassifier())
	if err != nil {
		s.emitUpdateClassifierEvent(ctx, authCtx, req.GetClassifier(), err)
		return nil, trace.Wrap(err)
	}

	s.emitUpdateClassifierEvent(ctx, authCtx, classifier, nil)
	return pb.UpdateClassifierResponse_builder{Classifier: classifier}.Build(), nil
}

// UpsertClassifier creates a new Classifier or updates an existing one.
func (s *Service) UpsertClassifier(
	ctx context.Context, req *pb.UpsertClassifierRequest,
) (*pb.UpsertClassifierResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindClassifier, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	// Determine whether the classifier already exists so we can emit a create
	// or update audit event accordingly.
	exists := false
	if _, err := s.cache.GetClassifier(ctx, req.GetClassifier().GetMetadata().GetName()); err == nil {
		exists = true
	}

	emit := s.emitCreateClassifierEvent
	if exists {
		emit = s.emitUpdateClassifierEvent
	}

	classifier, err := s.backend.UpsertClassifier(ctx, req.GetClassifier())
	if err != nil {
		emit(ctx, authCtx, req.GetClassifier(), err)
		return nil, trace.Wrap(err)
	}

	emit(ctx, authCtx, classifier, nil)
	return pb.UpsertClassifierResponse_builder{Classifier: classifier}.Build(), nil
}

// DeleteClassifier deletes an existing Classifier by name.
func (s *Service) DeleteClassifier(
	ctx context.Context, req *pb.DeleteClassifierRequest,
) (*pb.DeleteClassifierResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindClassifier, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	err = s.backend.DeleteClassifier(ctx, req.GetName())
	s.emitDeleteClassifierEvent(ctx, authCtx, req.GetName(), err)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.DeleteClassifierResponse{}, nil
}

// ListClassifiers lists all Classifiers that match the request.
func (s *Service) ListClassifiers(
	ctx context.Context, req *pb.ListClassifiersRequest,
) (*pb.ListClassifiersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindClassifier, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !s.isLicensed() {
		return nil, errNotLicensed
	}

	classifiers, nextPageToken, err := s.backend.ListClassifiers(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return pb.ListClassifiersResponse_builder{
		Classifiers:   classifiers,
		NextPageToken: nextPageToken,
	}.Build(), nil
}

func (s *Service) emitCreateClassifierEvent(ctx context.Context, authCtx *authz.Context, classifier *pb.Classifier, opErr error) {
	code := events.ClassifierCreateCode
	if opErr != nil {
		code = events.ClassifierCreateFailureCode
	}
	event := &apievents.ClassifierCreate{
		Metadata:           apievents.Metadata{Type: events.ClassifierCreateEvent, Code: code},
		UserMetadata:       authCtx.GetUserMetadata(),
		ResourceMetadata:   apievents.ResourceMetadata{Name: classifier.GetMetadata().GetName(), Expires: classifier.GetMetadata().GetExpires().AsTime()},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             classifierEventStatus(opErr),
	}
	if opErr == nil {
		event.Payload = s.encodeResourcePayload(classifier)
	}
	if err := s.emitter.EmitAuditEvent(ctx, event); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit classifier create event", "error", err)
	}
}

func (s *Service) emitUpdateClassifierEvent(ctx context.Context, authCtx *authz.Context, classifier *pb.Classifier, opErr error) {
	code := events.ClassifierUpdateCode
	if opErr != nil {
		code = events.ClassifierUpdateFailureCode
	}
	event := &apievents.ClassifierUpdate{
		Metadata:           apievents.Metadata{Type: events.ClassifierUpdateEvent, Code: code},
		UserMetadata:       authCtx.GetUserMetadata(),
		ResourceMetadata:   apievents.ResourceMetadata{Name: classifier.GetMetadata().GetName(), Expires: classifier.GetMetadata().GetExpires().AsTime()},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             classifierEventStatus(opErr),
	}
	if opErr == nil {
		event.Payload = s.encodeResourcePayload(classifier)
	}
	if err := s.emitter.EmitAuditEvent(ctx, event); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit classifier update event", "error", err)
	}
}

func (s *Service) emitDeleteClassifierEvent(ctx context.Context, authCtx *authz.Context, name string, opErr error) {
	code := events.ClassifierDeleteCode
	if opErr != nil {
		code = events.ClassifierDeleteFailureCode
	}
	if err := s.emitter.EmitAuditEvent(ctx, &apievents.ClassifierDelete{
		Metadata:           apievents.Metadata{Type: events.ClassifierDeleteEvent, Code: code},
		UserMetadata:       authCtx.GetUserMetadata(),
		ResourceMetadata:   apievents.ResourceMetadata{Name: name},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             classifierEventStatus(opErr),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit classifier delete event", "error", err)
	}
}

// classifierEventStatus returns an audit event status for the given operation error.
func classifierEventStatus(err error) apievents.Status {
	if err == nil {
		return apievents.Status{Success: true}
	}
	return apievents.Status{Success: false, Error: err.Error()}
}
