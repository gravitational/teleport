package summarizer

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
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
	"github.com/gravitational/teleport/api/defaults"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/metrics"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/openai"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
)

const (
	defaultTimeout = 180 * time.Second
	// Maximum number of concurrent summarization jobs. Currently estimated from
	// the lowest OpenAI tier, which is 500 RPM. Assuming about 30s per request,
	// this gives maximum concurrency of 250; we arbitrarily dial it down to 150.
	concurrencyLimit = 150
)

// SummarizerConfig contains configuration for the SessionSummarizer.
type SummarizerConfig struct {
	Backend         services.Summarizer
	Streamer        events.SessionStreamer
	ResourceGetter  ResourceGetter
	SummaryUploader SummaryUploader
	// OpenAIClientFactory creates OpenAI clients. Defaults to a production
	// implementation.
	OpenAIClientFactory openai.ClientFactory
	// Clock is used for time calculations. Defaults to a real clock.
	Clock clockwork.Clock
}

// SummaryUploader allows uploading recording summaries.
type SummaryUploader interface {
	// UploadSummary uploads a session summary and returns a URL with uploaded
	// file in case of success.
	UploadSummary(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error)
}

// InferenceProvider is an interface for providers that can summarize session
// recordings.
type InferenceProvider interface {
	// Summarizes a session. Should close the session reader when it's no longer
	// needed.
	Summarize(
		ctx context.Context, sessionID session.ID, sessionKind types.SessionKind, reader io.ReadCloser,
	) (string, error)
}

// SessionSummarizer summarizes session recordings using language model
// inference.
type SessionSummarizer struct {
	// TODO(bl-nero): use cache instead of raw backend.
	backend             services.Summarizer
	streamer            events.SessionStreamer
	resourceGetter      ResourceGetter
	summaryUploader     SummaryUploader
	openAIClientFactory openai.ClientFactory
	clock               clockwork.Clock
	logger              *slog.Logger
	concurrencyLimiter  *semaphore.Weighted
}

var _ summarizer.SessionSummarizer = (*SessionSummarizer)(nil)

// ResourceGetter retrieves resources from Teleport backend.
type ResourceGetter interface {
	services.UserGetter
	// GetDatabaseServers returns all database servers.
	GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error)
}

// NewSessionSummarizer creates a new session summarizer with given
// configuration.
func NewSessionSummarizer(cfg SummarizerConfig) (*SessionSummarizer, error) {
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required")
	}
	if cfg.Streamer == nil {
		return nil, trace.BadParameter("streamer is required")
	}
	if cfg.ResourceGetter == nil {
		return nil, trace.BadParameter("resource getter is required")
	}
	if cfg.SummaryUploader == nil {
		return nil, trace.BadParameter("upload handler is required")
	}

	clock := cfg.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	return &SessionSummarizer{
		backend:             cfg.Backend,
		streamer:            cfg.Streamer,
		resourceGetter:      cfg.ResourceGetter,
		summaryUploader:     cfg.SummaryUploader,
		openAIClientFactory: cfg.OpenAIClientFactory,
		clock:               clock,
		logger:              slog.With(teleport.ComponentKey, "summarizer"),
		concurrencyLimiter:  semaphore.NewWeighted(concurrencyLimit),
	}, nil
}

// TODO(bl-nero): rename SummarizeSSH to SummarizePTYSession.

// SummarizeSSH summarizes the SSH (or kubectl exec) session recording
// associated with the provided [apievents.SessionEnd] event.
func (s *SessionSummarizer) SummarizeSSH(ctx context.Context, sessionEndEvent *apievents.SessionEnd) error {
	if sessionEndEvent == nil {
		return trace.BadParameter("session end event is required to summarize an SSH session")
	}

	sessionID := session.ID(sessionEndEvent.SessionID)
	userName := sessionEndEvent.User
	var kind types.SessionKind
	switch sessionEndEvent.Protocol {
	case events.EventProtocolSSH:
		kind = types.SSHSessionKind
	case events.EventProtocolKube:
		kind = types.KubernetesSessionKind
	default:
		return trace.BadParameter("unsupported session protocol %s", sessionEndEvent.Protocol)
	}

	s.logger.DebugContext(
		ctx, "Summarizing session", "session_id", sessionID, "user", userName, "kind", kind,
	)

	// Remove the portion of server ID after the first dot, if any.
	serverID, err := splitServerID(sessionEndEvent.ServerID)
	if err != nil {
		return trace.Wrap(err)
	}

	// Recreate the node or k8s cluster from session event, as the node might
	// have disappeared since then and wouldn't be available in the first place
	// for Kubernetes sessions.
	var resource types.Resource
	switch sessionEndEvent.Protocol {
	case events.EventProtocolSSH:
		sm := sessionEndEvent.ServerMetadata
		var err error
		resource, err = types.NewServerWithLabels(
			serverID,
			types.KindNode,
			types.ServerSpecV2{
				Addr:     sm.ServerAddr,
				Hostname: sm.ServerHostname,
				Version:  sm.ServerVersion,
			},
			sm.ServerLabels,
		)
		if err != nil {
			return trace.Wrap(err)
		}

	case events.EventProtocolKube:
		km := sessionEndEvent.KubernetesClusterMetadata
		var err error
		resource, err = types.NewKubernetesClusterV3(
			types.Metadata{
				Name:   km.KubernetesCluster,
				Labels: km.KubernetesLabels,
			},
			types.KubernetesClusterSpecV3{},
		)
		if err != nil {
			return trace.Wrap(err)
		}

	default:
		return trace.BadParameter("unsupported session protocol %s", sessionEndEvent.Protocol)
	}

	return trace.Wrap(s.summarize(ctx, sessionID, kind, sessionEndEvent, userName, resource))
}

// SummarizeDatabase summarizes the database session recording associated with
// the provided [apievents.DatabaseSessionEnd] event.
func (s *SessionSummarizer) SummarizeDatabase(ctx context.Context, sessionEndEvent *apievents.DatabaseSessionEnd) error {
	if sessionEndEvent == nil {
		return trace.BadParameter("session end event is required to summarize a database session")
	}

	sessionID := session.ID(sessionEndEvent.SessionID)
	userName := sessionEndEvent.User
	kind := types.DatabaseSessionKind

	s.logger.DebugContext(
		ctx, "Summarizing a database session", "session_id", sessionID, "user", userName,
	)

	// TODO(bl-nero): extract and reuse rebuildResourceFromSessionEndEvent from
	// lib/auth.ServerWithRoles.
	database, err := s.getDatabase(ctx, sessionEndEvent.DatabaseService)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.summarize(ctx, sessionID, kind, sessionEndEvent, userName, database))
}

func (s *SessionSummarizer) getDatabase(ctx context.Context, name string) (types.Database, error) {
	dbServers, err := s.resourceGetter.GetDatabaseServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, dbs := range dbServers {
		db := dbs.GetDatabase()
		if db != nil && db.GetName() == name {
			return db, nil
		}
	}

	return nil, trace.NotFound("database \"%s\" not found", name)
}

// summarize picks the appropriate inference provider and launches a
// summarization goroutine.
func (s *SessionSummarizer) summarize(
	ctx context.Context,
	sessionID session.ID,
	kind types.SessionKind,
	sessionEndEvent apievents.AuditEvent,
	userName string,
	resource types.Resource,
) error {
	// TODO(bl-nero): Reconstruct user from event data after
	// https://github.com/gravitational/teleport/pull/58114/files is merged to
	// fully support SSO users whose records expire after a while.
	user, err := s.resourceGetter.GetUser(ctx, userName, false /* withSecrets */)
	if err != nil {
		return trace.Wrap(err)
	}

	sessionCtx := &services.Context{
		User:     user,
		Resource: resource,
		Session:  sessionEndEvent,
	}

	policy, err := s.matchPolicy(ctx, kind, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	if policy == nil {
		s.logger.DebugContext(ctx,
			"No matching summary inference policy found, session will not be summarized",
			"session_id", sessionID,
		)
		return nil
	}
	s.logger.DebugContext(
		ctx, "Matched summary inference policy", "session_id", sessionID, "policy", policy.Metadata.Name,
	)

	endEventFields, err := events.ToEventFields(sessionEndEvent)
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

	pendingResult := &summarizerv1pb.Summary{
		SessionId:          sessionID.String(),
		State:              summarizerv1pb.SummaryState_SUMMARY_STATE_PENDING,
		InferenceStartedAt: timestamppb.New(s.clock.Now().UTC()),
		ModelName:          policy.Spec.Model,
		SessionEndEvent:    endEventStruct,
	}

	// TODO(bl-nero): At this point, we should save the pending summary using the
	// upload handler, but the current implementations of our file storages has
	// wildly inconsistent behavior when overwriting existing files, so we can
	// only save the terminal state. Fix this and then enable the pending state.

	go s.summarizeNowAndReportMetrics(ctx, sessionID, provider, kind, pendingResult)
	return nil
}

func (s *SessionSummarizer) summarizeNowAndReportMetrics(
	ctx context.Context,
	sessionID session.ID,
	provider InferenceProvider,
	kind types.SessionKind,
	pendingResult *summarizerv1pb.Summary,
) {
	metrics.SummarizationsTotal.WithLabelValues(pendingResult.ModelName).Inc()
	err := s.summarizeNow(ctx, sessionID, provider, kind, pendingResult)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to summarize session", "session_id", sessionID, "error", err)
		metrics.SummarizationErrors.WithLabelValues(pendingResult.ModelName).Inc()
	}
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
func (s *SessionSummarizer) summarizeNow(
	ctx context.Context,
	sessionID session.ID,
	provider InferenceProvider,
	kind types.SessionKind,
	pendingResult *summarizerv1pb.Summary,
) error {
	// TODO(bl-nero): Make the timeout configurable, or at least depend on the
	// provider.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultTimeout)
	defer cancel()

	// Clone the pending result to avoid modifying the original one.
	result := proto.CloneOf(pendingResult)

	log := s.logger.With("session_id", sessionID)
	reader := newSessionReader(ctx, s.streamer, sessionID)
	defer reader.Close()

	metrics.SummarizationsPending.WithLabelValues(pendingResult.ModelName).Inc()
	// sumErr is a summarization error that can be saved into the summary state
	// and needs to be returned regardless of the state of other operations.
	sumErr := s.concurrencyLimiter.Acquire(ctx, 1)
	metrics.SummarizationsPending.WithLabelValues(pendingResult.ModelName).Dec()
	metrics.SummarizationsRunning.WithLabelValues(pendingResult.ModelName).Inc()
	defer metrics.SummarizationsRunning.WithLabelValues(pendingResult.ModelName).Dec()
	if sumErr != nil {
		sumErr = trace.Wrap(sumErr, "Failed to acquire the concurrency limiter semaphore")
		// log.ErrorContext(ctx, "Failed to acquire the concurrency limiter semaphore", "error", sumErr)
		result.State = summarizerv1pb.SummaryState_SUMMARY_STATE_ERROR
		result.ErrorMessage = sumErr.Error()
	} else {
		defer s.concurrencyLimiter.Release(1)
		var summaryContent string // Need to declare it here to prevent shadowing sumErr
		summaryContent, sumErr = provider.Summarize(ctx, sessionID, kind, reader)
		if sumErr != nil {
			sumErr = trace.Wrap(sumErr)
			// log.ErrorContext(ctx, "Failed to summarize session", "error", sumErr)
			result.State = summarizerv1pb.SummaryState_SUMMARY_STATE_ERROR
			result.ErrorMessage = sumErr.Error()
		} else {
			result.State = summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS
			result.Content = summaryContent
		}
	}

	result.InferenceFinishedAt = timestamppb.New(s.clock.Now().UTC())
	rBytes, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(result)
	if err != nil {
		return trace.NewAggregate(sumErr, trace.Wrap(err, "failed to marshal summary result"))
	}

	log.DebugContext(ctx, "Uploading session summary")
	path, err := s.summaryUploader.UploadSummary(ctx, sessionID, bytes.NewReader(rBytes))
	if err != nil {
		return trace.NewAggregate(sumErr, trace.Wrap(err, "failed to upload summary result"))
	}

	log.DebugContext(ctx, "Session summary uploaded", "path", path)
	return sumErr
}

// SummarizeWithoutEndEvent summarizes a session recording with a given ID.
// Used if the caller doesn't have a reference to the end event.
func (s *SessionSummarizer) SummarizeWithoutEndEvent(ctx context.Context, sessionID session.ID) error {
	sshEnd, dbEnd, err := s.findSessionEndEvent(ctx, sessionID)
	switch {
	case err != nil:
		return trace.Wrap(err)
	case sshEnd != nil:
		return s.SummarizeSSH(ctx, sshEnd)
	case dbEnd != nil:
		return s.SummarizeDatabase(ctx, dbEnd)
	default:
		return trace.NotFound("session end event not found")
	}
}

func (s *SessionSummarizer) findSessionEndEvent(ctx context.Context, sessionID session.ID) (*apievents.SessionEnd, *apievents.DatabaseSessionEnd, error) {
	eventsCh, errCh := s.streamer.StreamSessionEvents(ctx, sessionID, 0)

	for {
		select {
		case event, ok := <-eventsCh:
			if !ok {
				return nil, nil, nil
			}
			switch e := event.(type) {
			case *apievents.SessionEnd:
				return e, nil, nil
			case *apievents.DatabaseSessionEnd:
				return nil, e, nil
			}
		case err := <-errCh:
			return nil, nil, trace.Wrap(err)
		case <-ctx.Done():
			return nil, nil, trace.Wrap(ctx.Err())
		}
	}
}

// splitServerID splits a server ID into a node ID and cluster name.
func splitServerID(address string) (string, error) {
	split := strings.SplitN(address, ".", 2)
	if len(split) == 0 || split[0] == "" {
		return "", trace.BadParameter("invalid server id: \"%s\"", address)
	}

	return split[0], nil
}

// matchPolicy matches the session kind and context against the available
// inference policies. It returns the first matching policy or nil if no policy
// matches.
func (s *SessionSummarizer) matchPolicy(
	ctx context.Context, kind types.SessionKind, sessionCtx *services.Context,
) (*summarizerv1pb.InferencePolicy, error) {
	parser, err := services.NewWhereParser(sessionCtx)
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
		p, err := openai.NewProvider(ctx, openai.ProviderConfig{
			Spec:              providerCfg.Openai,
			Backend:           s.backend,
			MaxSessionLength:  model.GetSpec().GetMaxSessionLengthBytes(),
			ClientFactory:     s.openAIClientFactory,
			ModelResourceName: modelName,
		})
		return p, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unsupported provider type: %T", model.Spec.Provider)
	}
}
