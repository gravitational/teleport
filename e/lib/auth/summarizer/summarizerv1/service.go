package summarizerv1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/aws/smithy-go"
	"github.com/gravitational/trace"
	openailib "github.com/openai/openai-go/v3"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apisummarizer "github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/bedrock"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/openai"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// SummaryDownloader provides backend access to session summary recordings.
type SummaryDownloader interface {
	// DownloadSummary downloads a final session summary and writes it to a
	// writer.
	DownloadSummary(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error
}

// ServiceConfig holds configuration for the [Service].
type ServiceConfig struct {
	Authorizer        authz.Authorizer
	Backend           services.Summarizer
	SummaryDownloader SummaryDownloader
	Decrypter         events.DecryptionWrapper
	// OpenAIClientFactory creates OpenAI clients for testing. Optional.
	OpenAIClientFactory openai.ClientFactory
	// BedrockClientFactory creates Amazon Bedrock clients for testing. Optional.
	BedrockClientFactory bedrock.ClientFactory
	// AWSConfigCache is used to retrieve AWS OIDC tokens for Amazon Bedrock. Optional.
	AWSConfigCache *awsconfig.Cache
	// EnableBedrockWithoutRestrictions enables access to Amazon Bedrock models
	// outside of Teleport Cloud restrictions. Optional.
	EnableBedrockWithoutRestrictions bool
	UsageReporter                    usagereporter.UsageReporter
}

// Service provides an implementation of [pb.SummarizerServiceServer] and
// facilitates CRUD operations for summarizer resources.
type Service struct {
	// Use a forward-compatible server to make it easier to add new methods in
	// OSS without breaking the enterprise build.
	pb.UnimplementedSummarizerServiceServer
	authorizer                       authz.Authorizer
	backend                          services.Summarizer
	summaryDownloader                SummaryDownloader
	logger                           *slog.Logger
	decrypter                        events.DecryptionWrapper
	openAIClientFactory              openai.ClientFactory
	bedrockClientFactory             bedrock.ClientFactory
	awsConfigCache                   *awsconfig.Cache
	enableBedrockWithoutRestrictions bool
	usageReporter                    usagereporter.UsageReporter
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
	if cfg.UsageReporter == nil {
		return nil, trace.BadParameter("usage reporter is required")
	}

	return &Service{
		authorizer:                       cfg.Authorizer,
		backend:                          cfg.Backend,
		summaryDownloader:                cfg.SummaryDownloader,
		logger:                           slog.With(teleport.ComponentKey, "summarizer"),
		decrypter:                        cfg.Decrypter,
		openAIClientFactory:              cfg.OpenAIClientFactory,
		bedrockClientFactory:             cfg.BedrockClientFactory,
		awsConfigCache:                   cfg.AWSConfigCache,
		enableBedrockWithoutRestrictions: cfg.EnableBedrockWithoutRestrictions,
		usageReporter:                    cfg.UsageReporter,
	}, nil
}

// CRUD operations for models

func rejectReservedInferenceModelName(m *pb.InferenceModel) error {
	if m.GetMetadata().GetName() == apisummarizer.CloudDefaultInferenceModelName {
		return trace.BadParameter(
			"metadata.name %q is reserved", apisummarizer.CloudDefaultInferenceModelName,
		)
	}
	return nil
}

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

	if err := rejectReservedInferenceModelName(req.Model); err != nil {
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

	if err := rejectReservedInferenceModelName(req.Model); err != nil {
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

	if err := rejectReservedInferenceModelName(req.Model); err != nil {
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

	if req.Name == apisummarizer.CloudDefaultInferenceModelName {
		// TODO(bl-nero): Add a link to the documentation on default Bedrock model
		// once it's released.
		return nil, trace.BadParameter(
			"deleting the default Amazon Bedrock model is not supported in Teleport Cloud; edit or delete inference_policy resources that use it instead",
		)
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

	models, nextPageToken, err := s.backend.ListInferenceModels(ctx, int(req.PageSize), req.PageToken)
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

	secrets, nextPageToken, err := s.backend.ListInferenceSecrets(ctx, int(req.PageSize), req.PageToken)
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

	policies, nextPageToken, err := s.backend.ListInferencePolicies(ctx, int(req.PageSize), req.PageToken)
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
		// The user hasn't been fully authorized yet, so we don't return this
		// error, as it may leak details about the accessed object.
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("a recording summary for session %v was not found", sid)
		}
		s.logger.ErrorContext(
			ctx, "Unable to read session summary recording", "session_id", sid, "error", err,
		)
		return nil, trace.AccessDenied(
			"access denied to perform action %q on %q", types.VerbRead, types.KindSession,
		)
	}

	// Extend the context with the session end event and rebuild the resource
	// from the event.
	sctx.ExtendWithSessionEnd(sessionAuditEvent, authCtx.Checker)
	// Perform a fine-grained check that takes the session into consideration.
	err = authCtx.CheckAccessToRule(sctx, types.KindSession, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceName, sessionKind := resourceToSessionKind(sctx.Resource)
	s.usageReporter.AnonymizeAndSubmit(
		&usagereporter.SessionSummaryAccessEvent{
			UserName:     authCtx.User.GetName(),
			SessionType:  string(sessionKind),
			ResourceName: resourceName,
			UserKind:     usagereporter.PrehogUserKindFromEventKind(authCtx.GetUserMetadata().UserKind),
		},
	)

	// All checks passed, return the summary.
	return &pb.GetSummaryResponse{Summary: summary}, nil
}

// insecureGetSummary retrieves session summary and associated end event,
// ignoring access rules.
func (s *Service) insecureGetSummary(
	ctx context.Context, sid session.ID,
) (*pb.Summary, apievents.AuditEvent, error) {
	buf := &events.MemBuffer{}
	err := s.summaryDownloader.DownloadSummary(ctx, sid, buf)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	payload, err := s.decryptIfNeeded(ctx, buf.Bytes())
	if err != nil {
		return nil, nil, trace.Wrap(err, "decrypting session summary")
	}

	summary := &pb.Summary{}
	err = protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(payload, summary)
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

// decryptIfNeeded decrypts the data if it is encrypted.
// If the data is not encrypted, it is returned as-is.
func (r *Service) decryptIfNeeded(ctx context.Context, data []byte) ([]byte, error) {
	decryptedData, err := recordingencryption.DecryptBufferIfEncrypted(ctx, data, r.decrypter)
	return decryptedData, trace.Wrap(err)
}

func (s *Service) TestInferenceModel(
	ctx context.Context, req *pb.TestInferenceModelRequest,
) (*pb.TestInferenceModelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindInferenceModel, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resp := validateTestResources(req); resp != nil {
		return resp, nil
	}

	// Create a test provider based on the model spec
	provider, err := s.createTestProvider(ctx, req)
	if err != nil {
		return &pb.TestInferenceModelResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Make a simple test request with minimal input
	testSessionID := session.ID("test-session")
	testPrompt := "Test prompt"
	testInput := "echo 'test'"

	reader := io.NopCloser(strings.NewReader(testInput))
	_, err = provider.Summarize(ctx, testSessionID, testPrompt, reader)
	if err != nil {
		return &pb.TestInferenceModelResponse{
			Success: false,
			Message: formatInferenceError(err, req.GetModel()),
		}, nil
	}

	return &pb.TestInferenceModelResponse{
		Success: true,
		Message: "Successfully connected to the inference provider and received a response",
	}, nil
}

// createTestProvider creates an inference provider from the test request specs.
func (s *Service) createTestProvider(ctx context.Context, req *pb.TestInferenceModelRequest) (testProvider, error) {
	modelSpec := req.GetModel()
	if modelSpec == nil {
		return nil, trace.BadParameter("model spec is required")
	}

	switch providerCfg := modelSpec.Provider.(type) {
	case *pb.InferenceModelSpec_Openai:
		if req.GetSecret() == nil {
			if providerCfg.Openai.GetApiKeySecretRef() == "" {
				return nil, trace.BadParameter("api_key_secret_ref is required for OpenAI models when no secret is provided in the request")
			}
			// Fetch the secret from the backend
			if secret, err := s.backend.GetInferenceSecret(ctx, providerCfg.Openai.GetApiKeySecretRef()); trace.IsNotFound(err) {
				return nil, trace.BadParameter("secret %q not found in backend; please provide it in the request or ensure it exists", providerCfg.Openai.GetApiKeySecretRef())
			} else if err != nil {
				return nil, trace.Wrap(err)
			} else {
				req.Secret = secret.Spec
			}
		}

		p, err := openai.NewProvider(ctx, openai.ProviderConfig{
			ModelProvider:     providerCfg.Openai,
			SecretSpec:        req.GetSecret(),
			MaxSessionLength:  modelSpec.GetMaxSessionLengthBytes(),
			ClientFactory:     s.openAIClientFactory,
			ModelResourceName: "test-model",
		})
		return p, trace.Wrap(err)

	case *pb.InferenceModelSpec_Bedrock:
		if !s.enableBedrockWithoutRestrictions &&
			providerCfg.Bedrock.GetIntegration() == "" {
			return nil, trace.AccessDenied(
				"access to Amazon Bedrock models provided by Teleport Cloud is restricted; " +
					"please refer to the documentation for more information on enabling Bedrock integrations",
			)
		}

		p, err := bedrock.NewProvider(ctx, bedrock.ProviderConfig{
			Spec:              providerCfg.Bedrock,
			MaxSessionLength:  modelSpec.GetMaxSessionLengthBytes(),
			ClientFactory:     s.bedrockClientFactory,
			ModelResourceName: "test-model",
			AWSConfigCache:    s.awsConfigCache,
		})
		return p, trace.Wrap(err)

	default:
		return nil, trace.BadParameter("unsupported provider type: %T", modelSpec.Provider)
	}
}

// testProvider is a minimal interface for testing inference models.
type testProvider interface {
	Summarize(ctx context.Context, sessionID session.ID, systemPrompt string, reader io.ReadCloser) (string, error)
}

func validateTestResources(req *pb.TestInferenceModelRequest) *pb.TestInferenceModelResponse {
	if req.GetModel() == nil {
		return &pb.TestInferenceModelResponse{
			Success: false,
			Message: "model spec is required",
		}
	}
	testInferenceModel := apisummarizer.NewInferenceModel("test-model", req.GetModel())
	if err := apisummarizer.ValidateInferenceModel(testInferenceModel); err != nil {
		return &pb.TestInferenceModelResponse{
			Success: false,
			Message: "invalid model spec: " + err.Error(),
		}
	}

	if secret := req.GetSecret(); secret != nil {
		testInferenceSecret := apisummarizer.NewInferenceSecret("test-secret", secret)
		if err := apisummarizer.ValidateInferenceSecret(testInferenceSecret); err != nil {
			return &pb.TestInferenceModelResponse{
				Success: false,
				Message: "invalid secret spec: " + err.Error(),
			}
		}
	}
	return nil
}

// formatInferenceError formats errors from OpenAI and AWS Bedrock providers into user-friendly messages.
func formatInferenceError(err error, modelSpec *pb.InferenceModelSpec) string {
	if err == nil {
		return ""
	}

	switch providerCfg := modelSpec.Provider.(type) {
	case *pb.InferenceModelSpec_Openai:
		return formatOpenAIError(err, providerCfg.Openai)
	case *pb.InferenceModelSpec_Bedrock:
		return formatBedrockError(err, providerCfg.Bedrock)
	default:
		return fmt.Sprintf("inference request failed: %v", err)
	}
}

// formatOpenAIError formats OpenAI API errors into user-friendly messages.
func formatOpenAIError(err error, provider *pb.OpenAIProvider) string {
	// Check for OpenAI API errors
	var openaiErr *openailib.Error
	if errors.As(err, &openaiErr) {
		switch openaiErr.Code {
		case "invalid_api_key":
			return "Invalid API key provided. Please verify your OpenAI API key is correct."
		case "insufficient_quota":
			return "OpenAI API quota exceeded. Please check your usage limits and billing status."
		case "rate_limit_exceeded":
			return "OpenAI API rate limit exceeded. Please try again in a few moments."
		case "model_not_found":
			return fmt.Sprintf("Model %q not found. Please verify the model ID is correct and accessible with your API key.", provider.GetOpenaiModelId())
		case "invalid_request_error":
			return fmt.Sprintf("Invalid request to OpenAI API: %s", openaiErr.Message)
		case "server_error", "service_unavailable":
			return "OpenAI API is currently unavailable. Please try again later."
		default:
			return fmt.Sprintf("OpenAI API error (%s): %s", openaiErr.Code, openaiErr.Message)
		}
	}

	// Check for network/connection errors
	if errors.Is(err, context.DeadlineExceeded) {
		return "Request to OpenAI API timed out. Please check your network connection and try again."
	}
	if errors.Is(err, context.Canceled) {
		return "Request to OpenAI API was canceled."
	}

	// Check for trace errors
	if trace.IsConnectionProblem(err) {
		baseURL := provider.GetBaseUrl()
		if baseURL != "" {
			return fmt.Sprintf("Failed to connect to OpenAI API at %s. Please verify the base URL is correct and accessible.", baseURL)
		}
		return "Failed to connect to OpenAI API. Please check your network connection."
	}
	if trace.IsAccessDenied(err) {
		return "Access denied by OpenAI API. Please verify your API key has the necessary permissions."
	}

	// Generic error
	return fmt.Sprintf("Failed to connect to OpenAI API: %v", err)
}

// formatBedrockError formats AWS Bedrock errors into user-friendly messages.
func formatBedrockError(err error, provider *pb.BedrockProvider) string {
	// Check for AWS API errors
	var smithyErr smithy.APIError
	if errors.As(err, &smithyErr) {
		errorCode := smithyErr.ErrorCode()
		switch errorCode {
		case "ValidationException":
			return fmt.Sprintf("Invalid request to Amazon Bedrock: %s", smithyErr.ErrorMessage())
		case "ResourceNotFoundException":
			return fmt.Sprintf("Model %q not found in region %s. Please verify the model ID and ensure the model is available in this region.", provider.GetBedrockModelId(), provider.GetRegion())
		case "AccessDeniedException":
			integration := provider.GetIntegration()
			if integration != "" {
				return fmt.Sprintf("Access denied to Amazon Bedrock. Please verify the integration %q has the necessary IAM permissions to access Bedrock in region %s.", integration, provider.GetRegion())
			}
			return fmt.Sprintf("Access denied to Amazon Bedrock. Please verify your AWS credentials have the necessary IAM permissions to access Bedrock in region %s.", provider.GetRegion())
		case "ThrottlingException":
			return "Amazon Bedrock API rate limit exceeded. Please try again in a few moments."
		case "ServiceQuotaExceededException":
			return "Amazon Bedrock service quota exceeded. Please check your service limits."
		case "ModelTimeoutException":
			return "Amazon Bedrock model request timed out. Please try again."
		case "ModelNotReadyException":
			return fmt.Sprintf("Model %q is not ready. Please try again in a few moments.", provider.GetBedrockModelId())
		case "ModelErrorException":
			return fmt.Sprintf("Amazon Bedrock model error: %s", smithyErr.ErrorMessage())
		case "InternalServerException", "ServiceUnavailableException":
			return "Amazon Bedrock service is currently unavailable. Please try again later."
		default:
			return fmt.Sprintf("Amazon Bedrock API error (%s): %s", errorCode, smithyErr.ErrorMessage())
		}
	}

	// Check for network/connection errors
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("Request to Amazon Bedrock in region %s timed out. Please check your network connection and try again.", provider.GetRegion())
	}
	if errors.Is(err, context.Canceled) {
		return "Request to Amazon Bedrock was canceled."
	}

	// Check for trace errors
	if trace.IsConnectionProblem(err) {
		return fmt.Sprintf("Failed to connect to Amazon Bedrock in region %s. Please verify the region is correct and accessible.", provider.GetRegion())
	}
	if trace.IsAccessDenied(err) {
		integration := provider.GetIntegration()
		if integration != "" {
			return fmt.Sprintf("Access denied to Amazon Bedrock. Please verify the integration %q is configured correctly with proper IAM permissions.", integration)
		}
		return "Access denied to Amazon Bedrock. Please verify your AWS credentials and IAM permissions."
	}
	if trace.IsNotFound(err) {
		return fmt.Sprintf("Amazon Bedrock model %q not found in region %s.", provider.GetBedrockModelId(), provider.GetRegion())
	}

	// Generic error
	return fmt.Sprintf("Failed to connect to Amazon Bedrock: %v", err)
}

func resourceToSessionKind(resource types.Resource) (string, types.SessionKind) {
	if resource == nil {
		return "unknown", types.UnknownSessionKind
	}
	switch resource.GetKind() {
	case types.KindNode:
		return resource.GetName(), types.DatabaseSessionKind
	case types.KindKubernetesCluster:
		return resource.GetName(), types.KubernetesSessionKind
	case types.KindApp:
		return resource.GetName(), types.AppSessionKind
	case types.KindDatabase:
		return resource.GetName(), types.DatabaseSessionKind
	case types.KindWindowsDesktop:
		return resource.GetName(), types.WindowsDesktopSessionKind
	default:
		return "unknown", types.UnknownSessionKind
	}
}
