package summarizer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/vulcand/predicate"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apisummarizer "github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/bedrock"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/metrics"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/openai"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/prompts"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/ttyterminal"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

const (
	defaultTimeout = 10 * time.Minute
	// Maximum number of concurrent summarization jobs.
	concurrencyLimit = 25
	// Number of workers in the worker pool for summarization.
	workerCount = 10
)

// SummarizerConfig contains configuration for the SessionSummarizer.
type SummarizerConfig struct {
	Backend  services.Summarizer
	Streamer events.SessionStreamer
	// Encrypter is used to encrypt session summaries before uploading them.
	Encrypter events.EncryptionWrapper
	// SummaryUploader uploads session summaries.
	SummaryUploader SummaryUploader
	// OpenAIClientFactory creates OpenAI clients. Defaults to a production
	// implementation.
	OpenAIClientFactory openai.ClientFactory
	// BedrockClientFactory creates Amazon Bedrock clients. Defaults to a
	// production implementation.
	BedrockClientFactory bedrock.ClientFactory
	// Clock is used for time calculations. Defaults to a real clock.
	Clock clockwork.Clock
	// EnableBedrockWithoutRestrictions enables access to Amazon Bedrock models. Currently, this
	// should only be turned on outside Teleport Cloud. Setting it to true allows
	// using inference_model resources for inference.
	EnableBedrockWithoutRestrictions bool
	// AWSConfigCache is used to retrieve AWS OIDC tokens for Amazon Bedrock.
	AWSConfigCache *awsconfig.Cache
	// EnvBedrockRegion, if set to a non-empty value, will override Amazon
	// Bedrock region where it's set to {{env.bedrock_region}}
	EnvBedrockRegion string
	// EnvBedrockModelID, if set to a non-empty value, will override Amazon
	// Bedrock model ID where it's set to {{env.bedrock_model_id}}.
	EnvBedrockModelID string

	// UsageReporter reports usage events.
	UsageReporter usagereporter.UsageReporter
	// Emitter emits audit events.
	Emitter apievents.Emitter
}

// SummaryUploader allows uploading recording summaries.
type SummaryUploader interface {
	// UploadPendingSummary uploads a pending session summary and returns a URL
	// with uploaded file in case of success.
	UploadPendingSummary(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
	// UploadSummary uploads a full version of session summary and returns a URL
	// with uploaded file in case of success.
	UploadSummary(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
}

// InferenceProvider is an interface for providers that can summarize session
// recordings.
type InferenceProvider interface {
	// Summarizes a session. Should close the session reader when it's no longer
	// needed.
	Summarize(
		ctx context.Context, sessionID session.ID, systemPrompt string, reader io.ReadCloser,
	) (string, error)
	// SummarizeCommand summarizes a single command and returns the analysis.
	SummarizeCommand(ctx context.Context, sessionID session.ID, username, loginName, command string) (*schema.CommandAnalysis, error)
	// SummarizeMultipleCommands summarizes multiple commands and returns the
	// overall session analysis.
	SummarizeMultipleCommands(ctx context.Context, sessionID session.ID, username, loginName, prompt string) (*schema.SessionAnalysis, error)

	// GetTotalTokens returns the total number of input and output tokens used by this provider.
	GetTotalTokens() (input uint64, output uint64)

	// GetType returns the type of the inference provider.
	GetType() string
}

// SessionSummarizer summarizes session recordings using language model
// inference.
type SessionSummarizer struct {
	// TODO(bl-nero): use cache instead of raw backend.
	backend                          services.Summarizer
	streamer                         events.SessionStreamer
	summaryUploader                  SummaryUploader
	openAIClientFactory              openai.ClientFactory
	bedrockClientFactory             bedrock.ClientFactory
	clock                            clockwork.Clock
	logger                           *slog.Logger
	concurrencyLimiter               *semaphore.Weighted
	enableBedrockWithoutRestrictions bool
	encrypter                        events.EncryptionWrapper
	awsConfigCache                   *awsconfig.Cache
	pool                             *workerPool
	envBedrockRegion                 string
	envBedrockModelID                string
	usageReporter                    usagereporter.UsageReporter
	emitter                          apievents.Emitter
}

var _ summarizer.SessionSummarizer = (*SessionSummarizer)(nil)

// NewSessionSummarizer creates a new session summarizer with given
// configuration.
func NewSessionSummarizer(cfg SummarizerConfig) (*SessionSummarizer, error) {
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required")
	}
	if cfg.Streamer == nil {
		return nil, trace.BadParameter("streamer is required")
	}
	if cfg.SummaryUploader == nil {
		return nil, trace.BadParameter("upload handler is required")
	}
	if cfg.AWSConfigCache == nil {
		return nil, trace.BadParameter("AWS config cache is required")
	}
	if cfg.UsageReporter == nil {
		return nil, trace.BadParameter("usage reporter is required")
	}
	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required")
	}

	clock := cfg.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	return &SessionSummarizer{
		backend:                          cfg.Backend,
		streamer:                         cfg.Streamer,
		summaryUploader:                  cfg.SummaryUploader,
		openAIClientFactory:              cfg.OpenAIClientFactory,
		bedrockClientFactory:             cfg.BedrockClientFactory,
		clock:                            clock,
		logger:                           slog.With(teleport.ComponentKey, "summarizer"),
		concurrencyLimiter:               semaphore.NewWeighted(concurrencyLimit),
		enableBedrockWithoutRestrictions: cfg.EnableBedrockWithoutRestrictions,
		encrypter:                        cfg.Encrypter,
		awsConfigCache:                   cfg.AWSConfigCache,
		pool:                             newWorkerPool(workerCount),
		envBedrockRegion:                 cfg.EnvBedrockRegion,
		envBedrockModelID:                cfg.EnvBedrockModelID,
		usageReporter:                    cfg.UsageReporter,
		emitter:                          cfg.Emitter,
	}, nil
}

// sessionDetails contains details about the session to be summarized,
// including any pending summarization result.
type sessionDetails struct {
	sessionID    session.ID
	username     string
	loginName    string
	resourceName string
	kind         types.SessionKind
	summary      *summarizerv1pb.Summary
	provider     InferenceProvider
	sessionEnd   apievents.AuditEvent
}

// TODO(bl-nero): rename SummarizeSSH to SummarizePTYSession.

// SummarizeSSH summarizes the SSH (or kubectl exec) session recording
// associated with the provided [apievents.SessionEnd] event.
func (s *SessionSummarizer) SummarizeSSH(ctx context.Context, sessionEndEvent *apievents.SessionEnd) error {
	if sessionEndEvent == nil {
		return trace.BadParameter("session end event is required to summarize an SSH session")
	}

	sessionID := session.ID(sessionEndEvent.SessionID)
	username := sessionEndEvent.User
	loginName := sessionEndEvent.Login

	var resourceName string
	var kind types.SessionKind
	switch sessionEndEvent.Protocol {
	case events.EventProtocolSSH:
		kind = types.SSHSessionKind
		resourceName = sessionEndEvent.ServerMetadata.ServerID
	case events.EventProtocolKube:
		kind = types.KubernetesSessionKind
		resourceName = sessionEndEvent.KubernetesClusterMetadata.KubernetesCluster
	default:
		return trace.BadParameter("unsupported session protocol %s", sessionEndEvent.Protocol)
	}

	start := time.Now()
	s.logger.DebugContext(
		ctx, "Starting session summarization", "session_id", sessionID, "user", username, "kind", kind,
	)

	details := sessionDetails{
		sessionID:    sessionID,
		resourceName: resourceName,
		username:     username,
		loginName:    loginName,
		kind:         kind,
		sessionEnd:   sessionEndEvent,
	}

	if err := s.summarize(ctx, details); err != nil {
		return trace.Wrap(err)
	}

	s.logger.DebugContext(
		ctx, "Completed session summarization", "session_id", sessionID, "user", username,
		"duration", time.Since(start).String(),
	)

	return nil
}

// SummarizeDatabase summarizes the database session recording associated with
// the provided [apievents.DatabaseSessionEnd] event.
func (s *SessionSummarizer) SummarizeDatabase(ctx context.Context, sessionEndEvent *apievents.DatabaseSessionEnd) error {
	if sessionEndEvent == nil {
		return trace.BadParameter("session end event is required to summarize a database session")
	}

	sessionID := session.ID(sessionEndEvent.SessionID)
	username := sessionEndEvent.User
	kind := types.DatabaseSessionKind

	s.logger.DebugContext(
		ctx, "Summarizing a database session", "session_id", sessionID, "user", username,
	)

	details := sessionDetails{
		sessionID:    sessionID,
		resourceName: sessionEndEvent.DatabaseMetadata.DatabaseName,
		username:     username,
		kind:         kind,
		sessionEnd:   sessionEndEvent,
	}

	return trace.Wrap(s.summarize(ctx, details))
}

// summarize picks the appropriate inference provider and launches a
// summarization goroutine.
func (s *SessionSummarizer) summarize(ctx context.Context, details sessionDetails) error {
	supportedSessionKinds := [3]types.SessionKind{
		types.SSHSessionKind,
		types.KubernetesSessionKind,
		types.DatabaseSessionKind,
	}

	if !slices.Contains(supportedSessionKinds[:], details.kind) {
		return trace.BadParameter("unsupported session kind: %v", details.kind)
	}

	user, err := buildUserFromEvent(details.sessionEnd)
	if err != nil {
		return trace.Wrap(err, "failed to build user from event")
	}

	matchingCtx := &services.InferencePolicyMatchingContext{
		User: user,
	}
	matchingCtx.ExtendWithSessionEnd(details.sessionEnd)

	policy, err := s.matchPolicy(ctx, details.kind, matchingCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	if policy == nil {
		s.logger.DebugContext(ctx,
			"No matching summary inference policy found, session will not be summarized",
			"session_id", details.sessionID,
		)
		return nil
	}
	s.logger.DebugContext(
		ctx, "Matched summary inference policy", "session_id", details.sessionID, "policy", policy.Metadata.Name,
	)

	endEventFields, err := events.ToEventFields(details.sessionEnd)
	if err != nil {
		return trace.Wrap(err)
	}
	endEventStruct, err := structpb.NewStruct(endEventFields)
	if err != nil {
		return trace.Wrap(err)
	}

	provider, err := s.newProvider(ctx, policy.Spec.Model)
	if err != nil {
		return trace.Wrap(err)
	}

	details.provider = provider
	details.summary = &summarizerv1pb.Summary{
		SessionId:          details.sessionID.String(),
		State:              summarizerv1pb.SummaryState_SUMMARY_STATE_PENDING,
		InferenceStartedAt: timestamppb.New(s.clock.Now().UTC()),
		ModelName:          policy.Spec.Model,
		SessionEndEvent:    endEventStruct,
	}

	rBytes, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(details.summary)
	if err != nil {
		return trace.Wrap(err, "failed to marshal pending summary result")
	}
	s.logger.DebugContext(ctx, "Uploading pending session summary")
	_, err = s.summaryUploader.UploadPendingSummary(ctx, details.sessionID, bytes.NewReader(rBytes))
	if err != nil {
		return trace.Wrap(err, "failed to upload pending summary result")
	}

	// TODO(bl-nero): At this point, we should save the pending summary using the
	// upload handler, but the current implementations of our file storages has
	// wildly inconsistent behavior when overwriting existing files, so we can
	// only save the terminal state. Fix this and then enable the pending state.

	go s.summarizeNowAndReportMetrics(ctx, details)
	return nil
}

func (s *SessionSummarizer) summarizeNowAndReportMetrics(ctx context.Context, details sessionDetails) {
	metrics.SummarizationsTotal.WithLabelValues(details.summary.ModelName).Inc()

	success := true
	if err := s.summarizeNow(ctx, details); err != nil {
		s.logger.ErrorContext(ctx, "Failed to summarize session", "session_id", details.sessionID, "kind", details.kind, "error", err)
		metrics.SummarizationErrors.WithLabelValues(details.summary.ModelName).Inc()
		success = false
	}

	inputTokens, outputTokens := details.provider.GetTotalTokens()
	s.usageReporter.AnonymizeAndSubmit(&usagereporter.SessionSummaryCreateEvent{
		SessionType:       string(details.kind),
		Provider:          details.provider.GetType(),
		TotalInputTokens:  inputTokens,
		TotalOutputTokens: outputTokens,
		Success:           success,
		ResourceName:      details.resourceName,
		IsCloudDefaultModel: details.summary.GetModelName() == apisummarizer.CloudDefaultInferenceModelName &&
			s.enableBedrockWithoutRestrictions,
	})
}

// summarizeNow summarizes the session recording synchronously and uploads the
// result. If it's unable to summarize, it stores an error in the summary
// object. Regardless of the outcome, an attempt is then made to upload the
// summary. Any error that occurred either when summarizing or saving the
// summary is returned.
//
// The provided context is only used to create a new one with appropriate
// timeout and can be canceled at any time without affecting the summarization
// process.
func (s *SessionSummarizer) summarizeNow(ctx context.Context, details sessionDetails) error {
	// TODO(bl-nero): Make the timeout configurable, or at least depend on the
	// provider.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultTimeout)
	defer cancel()

	// Clone the pending result to avoid modifying the original one.
	result := proto.CloneOf(details.summary)

	log := s.logger.With("session_id", details.sessionID)

	modelName := details.summary.ModelName

	metrics.SummarizationsPending.WithLabelValues(modelName).Inc()
	// sumErr is a summarization error that can be saved into the summary state
	// and needs to be returned regardless of the state of other operations.
	sumErr := s.concurrencyLimiter.Acquire(ctx, 1)
	metrics.SummarizationsPending.WithLabelValues(modelName).Dec()

	if sumErr != nil {
		sumErr = trace.Wrap(sumErr, "Failed to acquire the concurrency limiter semaphore")
		result.State = summarizerv1pb.SummaryState_SUMMARY_STATE_ERROR
		result.ErrorMessage = sumErr.Error()
	} else {
		metrics.SummarizationsRunning.WithLabelValues(modelName).Inc()

		defer metrics.SummarizationsRunning.WithLabelValues(modelName).Dec()
		defer s.concurrencyLimiter.Release(1)

		switch details.kind {
		case types.SSHSessionKind:
			sumErr = s.summarizeSession(ctx, log, result, details)
		case types.DatabaseSessionKind, types.KubernetesSessionKind:
			sumErr = s.summarizeSimple(ctx, log, result, details)
		// This should be unreachable due to checks in the caller.
		default:
			sumErr = trace.BadParameter("unsupported session kind: %v", details.kind)
			result.State = summarizerv1pb.SummaryState_SUMMARY_STATE_ERROR
			result.ErrorMessage = sumErr.Error()
		}
	}

	result.InferenceFinishedAt = timestamppb.New(s.clock.Now().UTC())

	return s.uploadSummary(ctx, log, details.sessionID, result, sumErr, details.sessionEnd)
}

// summarizeSession performs the actual summarization of the session recording.
// It chooses between simple summarization and command analysis based on the
// session kind, and if SSH, whether the session contains bracketed paste sequences.
func (s *SessionSummarizer) summarizeSession(
	ctx context.Context,
	log *slog.Logger,
	result *summarizerv1pb.Summary,
	details sessionDetails,
) error {
	eventsCh, errCh := s.streamer.StreamSessionEvents(ctx, details.sessionID, 0)
	stream, err := ttyterminal.StreamTTYRecording(ctx, eventsCh, errCh)
	if err != nil {
		return handleError(ctx, log, result, err, "Failed to create session recording stream")
	}

	closeOnce := sync.OnceValue(stream.Close)
	defer closeOnce()

	if !stream.HasCommands() {
		return s.summarizeSimple(ctx, log, result, details)
	}

	analysis, commands, err := analyzeSessionCommands(ctx, details.sessionID, details.provider, s.pool, stream.Commands(), details.username, details.loginName)
	if err != nil {
		return handleError(ctx, log, result, err, "Failed to analyze session commands")
	}

	if err := closeOnce(); err != nil {
		return handleError(ctx, log, result, err, "Failed to process session recording stream")
	}

	result.State = summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS
	result.EnhancedSummary = schema.SessionAnalysisToProto(analysis, commands)

	return nil
}

// summarizeSimple performs a simple summarization of the session without
// command analysis. This is used for non-SSH sessions or SSH sessions that
// don't have bracketed paste sequences.
func (s *SessionSummarizer) summarizeSimple(
	ctx context.Context,
	log *slog.Logger,
	result *summarizerv1pb.Summary,
	details sessionDetails,
) error {
	reader := newSessionReader(ctx, s.streamer, details.sessionID)
	defer reader.Close()

	var systemPrompt string
	switch details.kind {
	case types.SSHSessionKind, types.KubernetesSessionKind:
		systemPrompt = prompts.SSHPrompt
	case types.DatabaseSessionKind:
		systemPrompt = prompts.DatabasePrompt
	// This should be unreachable due to checks in the caller.
	default:
		return handleError(ctx, log, result, trace.BadParameter("unsupported session kind: %v", details.kind), "Failed to summarize session")
	}

	content, err := details.provider.Summarize(ctx, details.sessionID, systemPrompt, reader)
	if err != nil {
		return handleError(ctx, log, result, err, "Failed to summarize session")
	}

	result.State = summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS
	result.Content = content

	return nil
}

func (s *SessionSummarizer) uploadSummary(
	ctx context.Context,
	log *slog.Logger,
	sessionID session.ID,
	result *summarizerv1pb.Summary,
	sumErr error,
	sessionEnd apievents.AuditEvent,
) error {
	rBytes, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(result)
	if err != nil {
		return trace.NewAggregate(sumErr, trace.Wrap(err, "failed to marshal summary result"))
	}

	if s.encrypter != nil {
		encrypted, err := s.encryptBytes(ctx, rBytes)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(encrypted) > 0 {
			rBytes = encrypted
		}
	}

	log.DebugContext(ctx, "Uploading session summary")
	path, err := s.summaryUploader.UploadSummary(ctx, sessionID, bytes.NewReader(rBytes))
	if err != nil {
		return trace.NewAggregate(sumErr, trace.Wrap(err, "failed to upload summary result"))
	}

	log.DebugContext(ctx, "Session summary uploaded", "path", path)

	// Emit audit event for the summary creation
	if err := s.emitSummaryCreateEvent(ctx, result, sessionEnd); err != nil {
		log.WarnContext(ctx, "Failed to emit session summary create audit event", "error", err)
	}

	return sumErr
}

func (s *SessionSummarizer) encryptBytes(ctx context.Context, data []byte) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	w, err := s.encrypter.WithEncryption(ctx, &nopCloser{buf})
	if errors.Is(err, recordingencryption.ErrEncryptionDisabled) {
		return nil, nil
	}
	if err != nil {
		return nil, trace.Wrap(err, "starting recording encrypter")
	}
	if _, err = w.Write(data); err != nil {
		_ = w.Close()
		return nil, trace.Wrap(err)
	}
	if err := w.Close(); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func handleError(
	ctx context.Context,
	log *slog.Logger,
	result *summarizerv1pb.Summary,
	err error,
	msg string,
) error {
	err = trace.Wrap(err)
	//nolint:sloglint // msg is not a string literal or constant
	log.ErrorContext(ctx, msg, "error", err)

	result.State = summarizerv1pb.SummaryState_SUMMARY_STATE_ERROR
	result.ErrorMessage = err.Error()

	return err
}

type nopCloser struct {
	io.Writer
}

func (n *nopCloser) Close() error {
	return nil
}

// SummarizeWithoutEndEvent summarizes a session recording with a given ID.
// Used if the caller doesn't have a reference to the end event.
func (s *SessionSummarizer) SummarizeWithoutEndEvent(ctx context.Context, sessionID session.ID) error {
	sEnd, err := events.FindSessionEndEvent(ctx, s.streamer, sessionID)
	if err != nil {
		return trace.Wrap(err, "failed to find session end event")
	}

	switch o := sEnd.(type) {
	case *apievents.SessionEnd:
		return trace.Wrap(s.SummarizeSSH(ctx, o))
	case *apievents.DatabaseSessionEnd:
		return trace.Wrap(s.SummarizeDatabase(ctx, o))
	default:
		return trace.BadParameter("unsupported session end event type %T", sEnd)
	}
}

// matchPolicy matches the session kind and context against the available
// inference policies. It returns the first matching policy or nil if no policy
// matches.
func (s *SessionSummarizer) matchPolicy(
	ctx context.Context, kind types.SessionKind, matchingCtx *services.InferencePolicyMatchingContext,
) (*summarizerv1pb.InferencePolicy, error) {
	parser, err := services.NewWhereParser(matchingCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for policy, err := range s.backend.AllInferencePolicies(ctx) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !slices.Contains(policy.Spec.Kinds, string(kind)) {
			continue
		}

		if policy.Spec.Filter == "" {
			return policy, nil
		}

		parseResult, err := parser.Parse(policy.Spec.Filter)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		pred, ok := parseResult.(predicate.BoolPredicate)
		if !ok {
			return nil, trace.BadParameter("unsupported type: %T", parseResult)
		}

		if pred() {
			return policy, nil
		}
	}

	return nil, nil
}

// newProvider creates a new inference provider based on the model name.
func (s *SessionSummarizer) newProvider(ctx context.Context, modelName string) (InferenceProvider, error) {
	model, err := s.backend.GetInferenceModel(ctx, modelName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch providerCfg := model.Spec.Provider.(type) {
	case *summarizerv1pb.InferenceModelSpec_Openai:
		apiKey, err := s.backend.GetInferenceSecret(ctx, providerCfg.Openai.GetApiKeySecretRef())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		p, err := openai.NewProvider(ctx, openai.ProviderConfig{
			ModelProvider:     providerCfg.Openai,
			SecretSpec:        apiKey.GetSpec(),
			MaxSessionLength:  model.GetSpec().GetMaxSessionLengthBytes(),
			ClientFactory:     s.openAIClientFactory,
			ModelResourceName: modelName,
		})
		return p, trace.Wrap(err)

	case *summarizerv1pb.InferenceModelSpec_Bedrock:
		if !s.enableBedrockWithoutRestrictions &&
			modelName != apisummarizer.CloudDefaultInferenceModelName &&
			model.GetSpec().GetBedrock().GetIntegration() == "" {
			return nil, trace.AccessDenied(
				"only the default model is allowed to use Amazon Bedrock without OIDC in Teleport Cloud",
			)
		}

		bedrockCfg := proto.CloneOf(providerCfg.Bedrock) // Protect from modifying function arguments
		if strings.ReplaceAll(bedrockCfg.BedrockModelId, " ", "") == apisummarizer.BedrockModelExpansionPlaceholder {
			if s.envBedrockModelID == "" {
				return nil, trace.BadParameter("bedrock_model_id cannot be empty. Please set the TELEPORT_BEDROCK_MODEL environment variable")
			}
			bedrockCfg.BedrockModelId = s.envBedrockModelID
		}

		if strings.ReplaceAll(bedrockCfg.Region, " ", "") == apisummarizer.BedrockRegionExpansionPlaceholder {
			if s.envBedrockRegion == "" {
				return nil, trace.BadParameter("region cannot be empty. Please set the TELEPORT_BEDROCK_REGION environment variable")
			}
			bedrockCfg.Region = s.envBedrockRegion
		}

		p, err := bedrock.NewProvider(ctx, bedrock.ProviderConfig{
			Spec:              bedrockCfg,
			MaxSessionLength:  model.GetSpec().GetMaxSessionLengthBytes(),
			ClientFactory:     s.bedrockClientFactory,
			ModelResourceName: modelName,
			AWSConfigCache:    s.awsConfigCache,
		})
		return p, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unsupported provider type: %T", model.Spec.Provider)
	}
}

func buildUserFromEvent(event apievents.AuditEvent) (types.User, error) {
	var (
		username   string
		userRoles  []string
		userTraits wrappers.Traits
	)

	switch e := event.(type) {
	case *apievents.SessionEnd:
		if e == nil {
			return nil, trace.BadParameter("nil %T event", e)
		}
		username = e.User
		userRoles = e.UserMetadata.UserRoles
		userTraits = e.UserMetadata.UserTraits
	case *apievents.DatabaseSessionEnd:
		if e == nil {
			return nil, trace.BadParameter("nil %T event", e)
		}
		username = e.User
		userRoles = e.UserMetadata.UserRoles
		userTraits = e.UserMetadata.UserTraits
	case *apievents.WindowsDesktopSessionEnd:
		if e == nil {
			return nil, trace.BadParameter("nil %T event", e)
		}
		username = e.User
		userRoles = e.UserMetadata.UserRoles
		userTraits = e.UserMetadata.UserTraits
	default:
		return nil, trace.BadParameter("unsupported event type %T", event)
	}

	if username == "" {
		return nil, trace.BadParameter("empty user name in event")
	}

	user, err := types.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetRoles(userRoles)
	user.SetTraits(userTraits)

	return user, nil
}

// emitSummaryCreateEvent emits a SessionSummarized audit event.
func (s *SessionSummarizer) emitSummaryCreateEvent(ctx context.Context, summary *summarizerv1pb.Summary, sessionEndEvent apievents.AuditEvent) error {
	// Create the audit event
	event := &apievents.SessionSummarized{
		Metadata: apievents.Metadata{
			Type:        events.SessionSummarizedEvent,
			Code:        events.SessionSummarizedCode,
			Time:        s.clock.Now().UTC(),
			ClusterName: sessionEndEvent.GetClusterName(),
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: summary.GetSessionId(),
		},
		Status: apievents.Status{
			Success: summary.GetState() == summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
			Error:   summary.GetErrorMessage(),
		},
		ModelName:           summary.GetModelName(),
		InferenceStartedAt:  summary.GetInferenceStartedAt().AsTime(),
		InferenceFinishedAt: summary.GetInferenceFinishedAt().AsTime(),
	}

	if !event.Success {
		event.Metadata.Code = events.SessionSummarizedErrorCode
	}

	// Extract enhanced summary fields if available
	if enhancedSummary := summary.GetEnhancedSummary(); enhancedSummary != nil {
		event.ShortDescription = enhancedSummary.GetShortDescription()
		event.RiskLevel = strings.TrimPrefix(enhancedSummary.GetRiskLevel().String(), "RISK_LEVEL_")
	} else {
		// For simple summaries, use the content as description
		if summary.GetContent() != "" {
			// Take first 200 characters as short description
			content := summary.GetContent()
			if len(content) > 200 {
				event.ShortDescription = content[:200] + "..."
			} else {
				event.ShortDescription = content
			}
		}
	}

	// Populate session-type-specific metadata
	switch e := sessionEndEvent.(type) {
	case *apievents.SessionEnd:
		event.SessionType = string(types.SSHSessionKind)
		event.Username = e.User
		if e.Protocol == events.EventProtocolKube {
			event.SessionType = string(types.KubernetesSessionKind)
			event.KubernetesClusterMetadata = e.KubernetesClusterMetadata
			event.KubernetesPodMetadata = e.KubernetesPodMetadata
		} else {
			event.ServerMetadata = e.ServerMetadata
		}
	case *apievents.DatabaseSessionEnd:
		event.SessionType = string(types.DatabaseSessionKind)
		event.DatabaseMetadata = e.DatabaseMetadata
		event.Username = e.User
	case *apievents.WindowsDesktopSessionEnd:
		event.SessionType = string(types.WindowsDesktopSessionKind)
		event.WindowsDesktopMetadata.WindowsDesktopService = e.WindowsDesktopService
		event.WindowsDesktopMetadata.DesktopAddr = e.DesktopAddr
		event.WindowsDesktopMetadata.Domain = e.Domain
		event.WindowsDesktopMetadata.WindowsUser = e.WindowsUser
		event.WindowsDesktopMetadata.DesktopLabels = e.DesktopLabels
		event.Username = e.User
	default:
		return trace.BadParameter("unsupported session end event type: %T", sessionEndEvent)
	}

	// Emit the audit event
	return trace.Wrap(s.emitter.EmitAuditEvent(ctx, event))
}
