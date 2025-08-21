package summarizerv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
)

// SummaryDownloader provides backend access to session summary recordings.
type SummaryDownloader interface {
	// DownloadSummary downloads a session summary and writes it to a writer.
	DownloadSummary(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error
}

// ServiceConfig holds configuration for the [Service].
type ServiceConfig struct {
	Authorizer        authz.Authorizer
	Backend           services.Summarizer
	SummaryDownloader SummaryDownloader
}

// Service provides an implementation of [pb.SummarizerServiceServer] and
// facilitates CRUD operations for summarizer resources.
type Service struct {
	// Use a forward-compatible server to make it easier to add new methods in
	// OSS without breaking the enterprise build.
	pb.UnimplementedSummarizerServiceServer
	authorizer        authz.Authorizer
	backend           services.Summarizer
	summaryDownloader SummaryDownloader
	logger            *slog.Logger
}

var _ pb.SummarizerServiceServer = (*Service)(nil)

// NewService creates a new instance of [Service] with the provided
// configuration. It returns an error if any of the required dependencies are
// not provided.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend service is required")
	}
	if cfg.SummaryDownloader == nil {
		return nil, trace.BadParameter("upload handler is required")
	}

	return &Service{
		authorizer:        cfg.Authorizer,
		backend:           cfg.Backend,
		summaryDownloader: cfg.SummaryDownloader,
		logger:            slog.With(teleport.ComponentKey, "summarizer"),
	}, nil
}

// CRUD operations for models

// CreateInferenceModel creates a new InferenceModel.
func (s *Service) CreateInferenceModel(
	ctx context.Context, req *pb.CreateInferenceModelRequest,
) (*pb.CreateInferenceModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceModel, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.CreateInferenceModel(ctx, req.Model)
	return &pb.CreateInferenceModelResponse{Model: model}, trace.Wrap(err)
}

// GetInferenceModel retrieves an existing InferenceModel by name.
func (s *Service) GetInferenceModel(
	ctx context.Context, req *pb.GetInferenceModelRequest,
) (*pb.GetInferenceModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceModel, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.GetInferenceModel(ctx, req.Name)
	return &pb.GetInferenceModelResponse{Model: model}, trace.Wrap(err)
}

// UpdateInferenceModel updates an existing InferenceModel.
func (s *Service) UpdateInferenceModel(
	ctx context.Context, req *pb.UpdateInferenceModelRequest,
) (*pb.UpdateInferenceModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceModel, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.UpdateInferenceModel(ctx, req.Model)
	return &pb.UpdateInferenceModelResponse{Model: model}, trace.Wrap(err)
}

// UpsertInferenceModel creates a new InferenceModel or updates an existing one.
func (s *Service) UpsertInferenceModel(
	ctx context.Context, req *pb.UpsertInferenceModelRequest,
) (*pb.UpsertInferenceModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceModel, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	model, err := s.backend.UpsertInferenceModel(ctx, req.Model)
	return &pb.UpsertInferenceModelResponse{Model: model}, trace.Wrap(err)
}

// DeleteInferenceModel deletes an existing InferenceModel by name.
func (s *Service) DeleteInferenceModel(
	ctx context.Context, req *pb.DeleteInferenceModelRequest,
) (*pb.DeleteInferenceModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceModel, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.backend.DeleteInferenceModel(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.DeleteInferenceModelResponse{}, nil
}

// ListInferenceModels lists all InferenceModels that match the request.
func (s *Service) ListInferenceModels(
	ctx context.Context, req *pb.ListInferenceModelsRequest,
) (*pb.ListInferenceModelsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceModel, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	models, nextPageToken, err :=
		s.backend.ListInferenceModels(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.ListInferenceModelsResponse{
		Models:        models,
		NextPageToken: nextPageToken,
	}, nil
}

// CRUD operations for secrets

// CreateInferenceSecret creates a new InferenceSecret.
func (s *Service) CreateInferenceSecret(
	ctx context.Context, req *pb.CreateInferenceSecretRequest,
) (*pb.CreateInferenceSecretResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceSecret, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secret, err := s.backend.CreateInferenceSecret(ctx, req.Secret)
	return &pb.CreateInferenceSecretResponse{Secret: secret}, trace.Wrap(err)
}

// GetInferenceSecret retrieves an existing InferenceSecret by name.
func (s *Service) GetInferenceSecret(
	ctx context.Context, req *pb.GetInferenceSecretRequest,
) (*pb.GetInferenceSecretResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceSecret, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secret, err := s.backend.GetInferenceSecret(ctx, req.Name)
	// Don't leak the secret.
	if secret != nil {
		secret.Spec = nil
	}
	return &pb.GetInferenceSecretResponse{Secret: secret}, trace.Wrap(err)
}

// UpdateInferenceSecret updates an existing InferenceSecret.
func (s *Service) UpdateInferenceSecret(
	ctx context.Context, req *pb.UpdateInferenceSecretRequest,
) (*pb.UpdateInferenceSecretResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceSecret, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secret, err := s.backend.UpdateInferenceSecret(ctx, req.Secret)
	// Don't leak the secret.
	if secret != nil {
		secret.Spec = nil
	}
	return &pb.UpdateInferenceSecretResponse{Secret: secret}, trace.Wrap(err)
}

// UpsertInferenceSecret creates a new InferenceSecret or updates an existing one.
func (s *Service) UpsertInferenceSecret(
	ctx context.Context, req *pb.UpsertInferenceSecretRequest,
) (*pb.UpsertInferenceSecretResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceSecret, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secret, err := s.backend.UpsertInferenceSecret(ctx, req.Secret)
	// Don't leak the secret.
	if secret != nil {
		secret.Spec = nil
	}
	return &pb.UpsertInferenceSecretResponse{Secret: secret}, trace.Wrap(err)
}

// DeleteInferenceSecret deletes an existing InferenceSecret by name.
func (s *Service) DeleteInferenceSecret(
	ctx context.Context, req *pb.DeleteInferenceSecretRequest,
) (*pb.DeleteInferenceSecretResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceSecret, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.backend.DeleteInferenceSecret(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.DeleteInferenceSecretResponse{}, nil
}

// ListInferenceSecrets lists all InferenceSecrets that match the request.
func (s *Service) ListInferenceSecrets(
	ctx context.Context, req *pb.ListInferenceSecretsRequest,
) (*pb.ListInferenceSecretsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceSecret, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secrets, nextPageToken, err :=
		s.backend.ListInferenceSecrets(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, secret := range secrets {
		// Don't leak the secret.
		secret.Spec = nil
	}

	return &pb.ListInferenceSecretsResponse{
		Secrets:       secrets,
		NextPageToken: nextPageToken,
	}, nil
}

// CRUD operations for policies

// CreateInferencePolicy creates a new InferencePolicy.
func (s *Service) CreateInferencePolicy(
	ctx context.Context, req *pb.CreateInferencePolicyRequest,
) (*pb.CreateInferencePolicyResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferencePolicy, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	policy, err := s.backend.CreateInferencePolicy(ctx, req.Policy)
	return &pb.CreateInferencePolicyResponse{Policy: policy}, trace.Wrap(err)
}

// GetInferencePolicy retrieves an existing InferencePolicy by name.
func (s *Service) GetInferencePolicy(
	ctx context.Context, req *pb.GetInferencePolicyRequest,
) (*pb.GetInferencePolicyResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferencePolicy, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	policy, err := s.backend.GetInferencePolicy(ctx, req.Name)
	return &pb.GetInferencePolicyResponse{Policy: policy}, trace.Wrap(err)
}

// UpdateInferencePolicy updates an existing InferencePolicy.
func (s *Service) UpdateInferencePolicy(
	ctx context.Context, req *pb.UpdateInferencePolicyRequest,
) (*pb.UpdateInferencePolicyResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferencePolicy, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	policy, err := s.backend.UpdateInferencePolicy(ctx, req.Policy)
	return &pb.UpdateInferencePolicyResponse{Policy: policy}, trace.Wrap(err)
}

// UpsertInferencePolicy creates a new InferencePolicy or updates an existing one.
func (s *Service) UpsertInferencePolicy(
	ctx context.Context, req *pb.UpsertInferencePolicyRequest,
) (*pb.UpsertInferencePolicyResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferencePolicy, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	policy, err := s.backend.UpsertInferencePolicy(ctx, req.Policy)
	return &pb.UpsertInferencePolicyResponse{Policy: policy}, trace.Wrap(err)
}

// DeleteInferencePolicy deletes an existing InferencePolicy by name.
func (s *Service) DeleteInferencePolicy(
	ctx context.Context, req *pb.DeleteInferencePolicyRequest,
) (*pb.DeleteInferencePolicyResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferencePolicy, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.backend.DeleteInferencePolicy(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.DeleteInferencePolicyResponse{}, nil
}

// ListInferencePolicies lists all InferencePolicies that match the request.
func (s *Service) ListInferencePolicies(
	ctx context.Context, req *pb.ListInferencePoliciesRequest,
) (*pb.ListInferencePoliciesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferencePolicy, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	policies, nextPageToken, err :=
		s.backend.ListInferencePolicies(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.ListInferencePoliciesResponse{
		Policies:      policies,
		NextPageToken: nextPageToken,
	}, nil
}

// GetSummary retrieves the inference result for a session, which contains the session summary.
func (s *Service) GetSummary(
	ctx context.Context, req *pb.GetSummaryRequest,
) (*pb.GetSummaryResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Perform first access check: see if the user can possibly access any
	// session at all, without taking into consideration the `where` clauses.
	// This is done to spare us from downloading the entire session if user's
	// access controls prevent them from reading any sessions at all.
	sctx := &services.Context{User: authCtx.User}
	err = authCtx.Checker.GuessIfAccessIsPossible(
		sctx, defaults.Namespace, types.KindSession, types.VerbRead,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Read the session summary.
	sid := session.ID(req.GetSessionId())
	summary, sessionAuditEvent, err := s.insecureGetSummary(
		ctx, session.ID(req.GetSessionId()),
	)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		// The user hasn't been fully authorized yet, so we don't return this
		// error, as it may leak details about the accessed object.
		s.logger.ErrorContext(
			ctx, "Unable to read session summary recording", "session_id", sid, "error", err,
		)
		return nil, trace.AccessDenied(
			"access denied to perform action %q on %q", types.VerbRead, types.KindSession,
		)
	}

	// Perform a fine-grained check that takes the session into consideration.
	sctx.Session = sessionAuditEvent
	err = authCtx.CheckAccessToRule(sctx, types.KindSession, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// All checks passed, return the summary.
	return &pb.GetSummaryResponse{Summary: summary}, nil
}

// insecureGetSummary retrieves session summary and associated end event,
// ignoring access rules.
func (s *Service) insecureGetSummary(
	ctx context.Context, sid session.ID,
) (*pb.Summary, apievents.AuditEvent, error) {
	buf := &memBuffer{}
	err := s.summaryDownloader.DownloadSummary(ctx, sid, buf)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	summary := &pb.Summary{}
	err = protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(buf.Bytes(), summary)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	sessionAuditEvent, err := events.FromEventFields(summary.SessionEndEvent.AsMap())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return summary, sessionAuditEvent, nil
}

// IsEnabled checks if the summarizer should be considered enabled. Session
// summarizer considers itself enabled if there's at least one model configured
// in the backend. This is not perfect, since whether the summarizer is
// configured NOW doesn't tell us if it was configured IN THE PAST (and may
// have generated data). This is all we've got for now, though.
func (s *Service) IsEnabled(
	ctx context.Context, req *pb.IsEnabledRequest,
) (res *pb.IsEnabledResponse, err error) {
	//  TODO(bl-nero): Figure out a better way to do this.
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// This endpoint can only be called by the proxy itself.
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return nil, trace.AccessDenied("access denied")
	}

	if !modules.GetModules().Features().GetEntitlement(entitlements.Policy).Enabled {
		return &pb.IsEnabledResponse{Enabled: false}, nil
	}

	models, _, err := s.backend.ListInferenceModels(ctx, 1, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.IsEnabledResponse{
		Enabled: len(models) > 0,
	}, nil
}
