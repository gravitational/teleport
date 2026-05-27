package sessionsearchv1

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	summarizerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/bedrock"
	summarizererrors "github.com/gravitational/teleport/e/lib/auth/summarizer/errors"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/openai"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// defaultPageSize is the number of session summaries requested per page
	// from the access graph when the caller does not specify max_results.
	defaultPageSize = 100
	// maxPageSize is the upper bound accepted for max_results to prevent
	// runaway memory use on both the access graph and the auth server.
	maxPageSize = 1000
	// maxSearchQueries bounds the number of embedding generations triggered by
	// a single search request.
	maxSearchQueries = 3
)

// AvailabilityChecker returns the cached session search availability state.
// Satisfied by [summarizer.AvailabilityCache].
type AvailabilityChecker interface {
	Get(ctx context.Context) (accessgraphv1.SessionSearchAvailability, error)
}

// EmbeddingProvider generates vector embeddings from text.
type EmbeddingProvider interface {
	GenerateEmbeddings(ctx context.Context, text string) ([]float32, int, error)
}

// ServiceConfig holds the dependencies for [Service].
type ServiceConfig struct {
	// Authorizer authorizes incoming requests.
	Authorizer authz.Authorizer
	// Cache provides access to summarizer resources (search model, secrets).
	Cache services.SummarizerServiceGetter
	// OpenAIClientFactory creates OpenAI clients. Optional; defaults to
	// the production implementation.
	OpenAIClientFactory openai.ClientFactory
	// BedrockClientFactory creates Amazon Bedrock clients. Optional; defaults
	// to the production implementation.
	BedrockClientFactory bedrock.ClientFactory
	// AWSConfigCache is used to obtain AWS credentials for Bedrock. Optional.
	AWSConfigCache *awsconfig.Cache
	// EnvBedrockRegion, if set to a non-empty value, will override Amazon
	// Bedrock region where it's set to {{env.bedrock_region}}
	EnvBedrockRegion string
	// AccessGraphClientGetter is the pre-built access graph client getter used to search
	// session summaries and store them.
	AccessGraphClientGetter func() (accessgraphv1.SessionRecordingServiceClient, error)
	// AvailabilityCache caches the session search availability state so that
	// SearchSessionSummaries can return a fast error before generating
	// embeddings when the access graph does not support session search.
	AvailabilityCache AvailabilityChecker
	// IsLicensed reports whether the SessionSummaries entitlement is active.
	IsLicensed func() bool
}

// Service implements [pb.SessionSearchServiceServer].
//
// It queries the access graph SearchService using the caller's filter
// criteria, generates vector embeddings for any keyword search, evaluates
// each returned session against the caller's access rules, and streams the
// approved sessions to the client.
type Service struct {
	// Embed the unimplemented server so that adding new RPCs to the proto
	// doesn't break the enterprise build.
	pb.UnimplementedSessionSearchServiceServer

	authorizer              authz.Authorizer
	cache                   services.SummarizerServiceGetter
	accessGraphClientGetter func() (accessgraphv1.SessionRecordingServiceClient, error)
	availabilityCache       AvailabilityChecker
	isLicensed              func() bool
	openAIClientFactory     openai.ClientFactory
	bedrockClientFactory    bedrock.ClientFactory
	awsConfigCache          *awsconfig.Cache
	logger                  *slog.Logger
	envBedrockRegion        string
}

var _ pb.SessionSearchServiceServer = (*Service)(nil)

// NewService validates cfg and returns a ready-to-use [Service].
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Cache == nil {
		return nil, trace.BadParameter("backend is required")
	}
	if cfg.AccessGraphClientGetter == nil {
		return nil, trace.BadParameter("access graph client getter is required")
	}
	if cfg.AvailabilityCache == nil {
		return nil, trace.BadParameter("availability cache is required")
	}
	if cfg.IsLicensed == nil {
		return nil, trace.BadParameter("is licensed function is required")
	}

	return &Service{
		authorizer:              cfg.Authorizer,
		cache:                   cfg.Cache,
		accessGraphClientGetter: cfg.AccessGraphClientGetter,
		availabilityCache:       cfg.AvailabilityCache,
		isLicensed:              cfg.IsLicensed,
		openAIClientFactory:     cfg.OpenAIClientFactory,
		bedrockClientFactory:    cfg.BedrockClientFactory,
		awsConfigCache:          cfg.AWSConfigCache,
		logger:                  slog.With(teleport.ComponentKey, "session-search"),
		envBedrockRegion:        cfg.EnvBedrockRegion,
	}, nil
}

// validateRequest checks the semantic validity of a SearchSessionSummariesRequest.
func validateRequest(req *pb.SearchSessionSummariesRequest) error {
	if req.GetStartTime() == nil || req.GetStartTime().AsTime().IsZero() {
		return trace.BadParameter("start_time is required")
	}
	if req.GetEndTime() == nil || req.GetEndTime().AsTime().IsZero() {
		return trace.BadParameter("end_time is required")
	}

	from := req.GetStartTime().AsTime()
	to := req.GetEndTime().AsTime()
	if from.After(to) {
		return trace.BadParameter("start_time (%v) must not be after end_time (%v)", from, to)
	}

	if req.GetMaxResults() > maxPageSize {
		return trace.BadParameter("max_results %d exceeds maximum allowed value of %d", req.GetMaxResults(), maxPageSize)
	}
	if len(req.GetSearchQueries()) > maxSearchQueries {
		return trace.BadParameter("search_queries count %d exceeds maximum allowed value of %d", len(req.GetSearchQueries()), maxSearchQueries)
	}
	return nil
}

// IsEnabled implements [pb.SessionSearchServiceServer].
//
// It returns the cached availability state of session search, refreshing from
// the access graph when the TTL has elapsed. Only the proxy role or clients with
// audit log access should call this method, and it is used to short-circuit the
// session search flow with a fast error when the access graph does not support
// session search. The CLI calls this method before starting the session search
// flow and embedding generation, and the UI calls this method before showing
// the session search page.
func (s *Service) IsEnabled(
	ctx context.Context, _ *pb.IsEnabledRequest,
) (*pb.IsEnabledResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authorizeIsEnabled(authCtx); err != nil {
		return nil, trace.Wrap(err)
	}
	if !s.isLicensed() {
		return &pb.IsEnabledResponse{Availability: pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_UNSPECIFIED}, nil
	}

	availability, err := s.availabilityCache.Get(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.IsEnabledResponse{
		Availability: convertAvailability(availability),
	}, nil
}

// authorizeIsEnabled enforces that the caller is either a proxy or admin role, or has access to
// list/read sessions. This is a coarse-grained check that doesn't consider the
// caller's access to individual sessions, but it is sufficient to gate the
// IsEnabled method since its purpose is just to short-circuit the session search
// flow for unauthorized callers and when session search is unavailable.
func authorizeIsEnabled(authCtx *authz.Context) error {
	// Authorize proxy role.
	if authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return nil
	}

	// Authorize admin role.
	if authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return nil
	}

	// If not a proxy but still a service, return access denied.
	if authz.IsLocalOrRemoteService(*authCtx) {
		return trace.AccessDenied("access denied")
	}

	// Authorize access to sessions for users.
	err := authCtx.MaybeAccessToKind(
		types.KindSession, types.VerbList, types.VerbRead,
	)
	return trace.Wrap(err)
}

// convertAvailability maps the access graph's SessionSearchAvailability enum
// to the Teleport API's equivalent. Both enums share the same values; this
// function makes the boundary between the two packages explicit.
func convertAvailability(a accessgraphv1.SessionSearchAvailability) pb.SessionSearchAvailability {
	switch a {
	case accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_AVAILABLE:
		return pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_AVAILABLE
	case accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_NOT_IMPLEMENTED:
		return pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_NOT_IMPLEMENTED
	case accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_TRGM_UNAVAILABLE:
		return pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_TRGM_UNAVAILABLE
	case accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_VECTOR_UNAVAILABLE:
		return pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_VECTOR_UNAVAILABLE
	default:
		return pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_UNSPECIFIED
	}
}

// SearchSessionSummaries implements [pb.SessionSearchServiceServer].
//
// The method:
//  1. Authorizes the caller and checks coarse-grained session visibility.
//  2. Optionally generates vector embeddings from search_queries.
//  3. Opens a bidirectional stream to the access graph and sends the query.
//  4. Consumes the stream: buffers approved sessions from each results batch,
//     requests further pages via FetchMore, and forwards approved sessions to
//     the caller as they accumulate across page boundaries.
func (s *Service) SearchSessionSummaries(
	req *pb.SearchSessionSummariesRequest,
	stream pb.SessionSearchService_SearchSessionSummariesServer,
) error {
	ctx := stream.Context()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := validateRequest(req); err != nil {
		return trace.Wrap(err)
	}

	if !s.isLicensed() {
		return summarizererrors.ErrUnlicensed
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := authCtx.MaybeAccessToKind(
		types.KindSession, types.VerbList, types.VerbRead,
	); err != nil {
		return trace.Wrap(err)
	}

	availability, err := s.availabilityCache.Get(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	switch availability {
	case accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_NOT_IMPLEMENTED,
		accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_TRGM_UNAVAILABLE,
		accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_VECTOR_UNAVAILABLE:
		return trace.NotImplemented("session search is not available: %v", availability)
	}

	agParams, err := s.buildAccessGraphParams(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.streamFromAccessGraph(ctx, authCtx, agParams, func(summary *pb.SessionSummary, nextBatchToken string) error {
		switch {
		case summary != nil:
			return stream.Send(&pb.SearchSessionSummariesResponse{
				Payload: &pb.SearchSessionSummariesResponse_Summary{
					Summary: summary,
				},
			})
		default:
			return stream.Send(&pb.SearchSessionSummariesResponse{
				Payload: &pb.SearchSessionSummariesResponse_BatchComplete_{
					BatchComplete: &pb.SearchSessionSummariesResponse_BatchComplete{
						HasMore:        nextBatchToken != "",
						NextBatchToken: nextBatchToken,
					},
				},
			})
		}
	}))
}

// streamFromAccessGraph opens a bidi stream to the access graph, sends agParams,
// and forwards RBAC-filtered results by calling send for each page of approved
// summaries. It is the shared implementation used by both SearchSessionSummaries
// and NaturalLanguageSearchSessionSummaries.
func (s *Service) streamFromAccessGraph(
	ctx context.Context,
	authCtx *authz.Context,
	agParams *accessgraphv1.SearchSessionSummariesParams,
	send func(summary *pb.SessionSummary, nextBatchToken string) error,
) error {
	client, err := s.accessGraphClientGetter()
	if err != nil {
		return trace.Wrap(err)
	}
	agStream, err := client.SearchSessionSummaries(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := agStream.Send(&accessgraphv1.SearchSessionSummariesRequest{
		Payload: &accessgraphv1.SearchSessionSummariesRequest_SearchParams{
			SearchParams: agParams,
		},
	}); err != nil {
		// Send errors on bidi streams are typically opaque (e.g. io.EOF).
		// Drain the real server-side error via Recv.
		var recvErr error
		for recvErr == nil {
			_, recvErr = agStream.Recv()
		}
		return trace.Wrap(recvErr, "sending search parameters to access graph")
	}

	var (
		sentCount           int
		lastCheckpointToken string
	)
loop:
	for {
		agResp, err := agStream.Recv()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return trace.Wrap(err)
		}

		switch p := agResp.Payload.(type) {
		case *accessgraphv1.SearchSessionSummariesResponse_Summary:
			// Always update the checkpoint cursor before RBAC filtering: the
			// access graph has already advanced past this position regardless
			// of whether we surface the summary to the caller.
			lastCheckpointToken = p.Summary.GetCheckpointToken()
			agSummary := p.Summary.GetSummary()
			if !s.canViewSession(ctx, authCtx, agSummary) {
				continue
			}
			sentCount++
			if err := send(convertSummary(agSummary), ""); err != nil {
				return trace.Wrap(err)
			}

			if uint32(sentCount) >= agParams.GetMaxSummaries() {
				if err := send(nil, lastCheckpointToken); err != nil {
					return trace.Wrap(err)
				}
				_ = agStream.CloseSend()
				break loop
			}

		case *accessgraphv1.SearchSessionSummariesResponse_BatchComplete_:
			var nextBatchToken string
			missingEntries := int32(agParams.GetMaxSummaries()) - int32(sentCount)
			done := false
			switch {
			case p.BatchComplete.GetHasMore() && missingEntries > 0:
				// Advance to the next page. On send failure, consume the real
				// error via Recv (send errors on bidi streams are typically
				// opaque, e.g. io.EOF).
				if err := agStream.Send(&accessgraphv1.SearchSessionSummariesRequest{
					Payload: &accessgraphv1.SearchSessionSummariesRequest_FetchMore_{
						FetchMore: &accessgraphv1.SearchSessionSummariesRequest_FetchMore{
							MaxSummaries: agParams.GetMaxSummaries(),
						},
					},
				}); err != nil {
					_, recvErr := agStream.Recv()
					return trace.Wrap(recvErr)
				}
			case p.BatchComplete.GetHasMore():
				// Quota reached: record the resume cursor, half-close, and stop.
				nextBatchToken = lastCheckpointToken
				_ = agStream.CloseSend()
				done = true
			default:
				// Search exhausted: half-close and stop.
				_ = agStream.CloseSend()
				done = true
			}

			if done {
				if err := send(nil, nextBatchToken); err != nil {
					return trace.Wrap(err)
				}
				break loop
			}
		}
	}
	return nil
}

// buildAccessGraphParams maps the caller's request into the access graph's
// SearchSessionSummariesParams, generating vector embeddings when search
// queries are present.
func (s *Service) buildAccessGraphParams(
	ctx context.Context,
	req *pb.SearchSessionSummariesRequest,
) (*accessgraphv1.SearchSessionSummariesParams, error) {
	maxSummaries := req.GetMaxResults()
	if maxSummaries == 0 {
		maxSummaries = defaultPageSize
	}

	params := &accessgraphv1.SearchSessionSummariesParams{
		StartTime:          req.GetStartTime(),
		EndTime:            req.GetEndTime(),
		Kinds:              req.GetKinds(),
		Username:           req.Username,
		UserRoles:          req.GetUserRoles(),
		AccessRequestIds:   req.GetAccessRequestIds(),
		ResourceKind:       req.ResourceKind,
		ResourceName:       req.ResourceName,
		ResourceLabels:     req.GetResourceLabels(),
		ResourceProperties: convertResourceProperties(req.GetResourceProperties()),
		Severity:           req.GetSeverity(),
		MaxSummaries:       maxSummaries,
		ResumeToken:        req.GetBatchToken(),
		SearchMode:         accessgraphv1.SearchMode(req.GetSearchMode()),
	}

	skipEmbeddings := req.GetSearchMode() == pb.SearchMode_SEARCH_MODE_KEYWORD_ONLY

	for _, query := range req.GetSearchQueries() {
		eq := &accessgraphv1.EmbeddedQuery{Text: query}
		if !skipEmbeddings {
			vec, modelName, err := s.generateEmbeddings(ctx, query)
			if err != nil {
				return nil, trace.Wrap(err, "generating embeddings for search query")
			}
			eq.Embeddings = vec
			eq.ModelName = modelName
		}
		params.SearchQueries = append(params.SearchQueries, eq)
	}

	return params, nil
}

// generateEmbeddings looks up the configured search model and produces a
// vector embedding for text using the model's embedding provider. It returns
// the embedding vector and the model resource name used to produce it.
func (s *Service) generateEmbeddings(ctx context.Context, text string) ([]float32, string, error) {
	retrievalModel, err := s.cache.GetRetrievalModel(ctx)
	if trace.IsNotFound(err) {
		return nil, "", trace.NotFound("no retrieval model configured for embedding generation")
	} else if err != nil {
		return nil, "", trace.Wrap(err, "getting search model for embedding generation")
	}

	provider, err := s.newEmbeddingsProvider(ctx, retrievalModel)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	vec, _, err := provider.GenerateEmbeddings(ctx, text)
	return vec, retrievalModel.GetMetadata().GetName(), trace.Wrap(err)
}

// canViewSession returns true when authCtx permits reading the session
// described by summary. It parses the raw session_end_event carried in the
// access graph response and evaluates the caller's RBAC rules against it.
func (s *Service) canViewSession(ctx context.Context, authCtx *authz.Context, summary *accessgraphv1.SessionSummary) bool {
	endEventStruct := summary.GetSessionEndEvent()
	if endEventStruct == nil {
		// No access control information available; deny by default.
		return false
	}

	auditEvent, err := events.FromEventFields(endEventStruct.AsMap())
	if err != nil {
		s.logger.WarnContext(
			ctx,
			"Failed to parse session_end_event; denying access",
			"session_id", summary.GetSessionId(),
			"error", err,
		)
		return false
	}

	sctx := &services.Context{User: authCtx.User}
	sctx.ExtendWithSessionEnd(auditEvent, authCtx.Checker)
	return authCtx.CheckAccessToRule(sctx, types.KindSession, types.VerbRead) == nil
}

// convertSummary maps an access graph SessionSummary to its auth server
// counterpart, dropping the internal session_end_event field.
func convertSummary(src *accessgraphv1.SessionSummary) *pb.SessionSummary {
	return &pb.SessionSummary{
		SessionId:          src.GetSessionId(),
		Kind:               src.GetKind(),
		SessionStart:       src.GetSessionStart(),
		Username:           src.GetUsername(),
		UserTraits:         src.GetUserTraits(),
		UserRoles:          src.GetUserRoles(),
		AccessRequestIds:   src.GetAccessRequestIds(),
		Participants:       src.GetParticipants(),
		ResourceKind:       src.GetResourceKind(),
		ResourceLabels:     src.GetResourceLabels(),
		ResourceId:         src.GetResourceId(),
		ResourceName:       src.GetResourceName(),
		ResourceProperties: convertAGResourceProperties(src.GetResourceProperties()),
		Severity:           src.GetSeverity(),
		SessionEnd:         src.GetSessionEnd(),
		HostId:             src.GetHostId(),
	}
}

// convertResourceProperties maps the caller's ResourceProperties to the
// access graph equivalent. The two message types have the same shape but
// live in different Go packages.
func convertResourceProperties(src *pb.ResourceProperties) *accessgraphv1.ResourceProperties {
	if src == nil {
		return nil
	}
	switch f := src.Type.(type) {
	case *pb.ResourceProperties_Ssh:
		return &accessgraphv1.ResourceProperties{
			Type: &accessgraphv1.ResourceProperties_Ssh{
				Ssh: &accessgraphv1.SSHProperties{
					ServerHostname: f.Ssh.ServerHostname,
					ServerAddr:     f.Ssh.ServerAddr,
				},
			},
		}
	case *pb.ResourceProperties_Kubernetes:
		return &accessgraphv1.ResourceProperties{
			Type: &accessgraphv1.ResourceProperties_Kubernetes{
				Kubernetes: &accessgraphv1.KubernetesProperties{
					PodNamespace: f.Kubernetes.PodNamespace,
					PodName:      f.Kubernetes.PodName,
				},
			},
		}
	case *pb.ResourceProperties_Database:
		return &accessgraphv1.ResourceProperties{
			Type: &accessgraphv1.ResourceProperties_Database{
				Database: &accessgraphv1.DatabaseProperties{
					DatabaseName: f.Database.DatabaseName,
				},
			},
		}
	default:
		return nil
	}
}

// convertAGResourceProperties maps an access graph ResourceProperties to its
// auth server counterpart.
func convertAGResourceProperties(src *accessgraphv1.ResourceProperties) *pb.ResourceProperties {
	if src == nil {
		return nil
	}
	switch f := src.Type.(type) {
	case *accessgraphv1.ResourceProperties_Ssh:
		return &pb.ResourceProperties{
			Type: &pb.ResourceProperties_Ssh{
				Ssh: &pb.SSHProperties{
					ServerHostname: f.Ssh.ServerHostname,
					ServerAddr:     f.Ssh.ServerAddr,
				},
			},
		}
	case *accessgraphv1.ResourceProperties_Kubernetes:
		return &pb.ResourceProperties{
			Type: &pb.ResourceProperties_Kubernetes{
				Kubernetes: &pb.KubernetesProperties{
					PodNamespace: f.Kubernetes.PodNamespace,
					PodName:      f.Kubernetes.PodName,
				},
			},
		}
	case *accessgraphv1.ResourceProperties_Database:
		return &pb.ResourceProperties{
			Type: &pb.ResourceProperties_Database{
				Database: &pb.DatabaseProperties{
					DatabaseName: f.Database.DatabaseName,
				},
			},
		}
	default:
		return nil
	}
}

// newEmbeddingsProvider constructs an embedding provider from the search
// model configuration, reusing the same provider logic as the rest of the
// summarizer subsystem.
func (s *Service) newEmbeddingsProvider(
	ctx context.Context,
	model *summarizerpb.RetrievalModel,
) (EmbeddingProvider, error) {
	switch providerCfg := model.GetSpec().GetEmbeddingsProvider().(type) {
	case *summarizerpb.RetrievalModelSpec_Openai:
		secret, err := s.cache.GetInferenceSecret(ctx, providerCfg.Openai.GetApiKeySecretRef())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		p, err := openai.NewEmbeddingProvider(ctx, openai.EmbeddingProviderConfig{
			EmbeddingsSpec:    providerCfg.Openai,
			SecretSpec:        secret.GetSpec(),
			ClientFactory:     s.openAIClientFactory,
			ModelResourceName: model.GetMetadata().GetName(),
		})
		return p, trace.Wrap(err)

	case *summarizerpb.RetrievalModelSpec_Bedrock:
		p, err := bedrock.NewEmbeddingProvider(ctx, bedrock.EmbeddingProviderConfig{
			Spec:              providerCfg.Bedrock,
			ClientFactory:     s.bedrockClientFactory,
			ModelResourceName: model.GetMetadata().GetName(),
			AWSConfigCache:    s.awsConfigCache,
			EnvBedrockRegion:  s.envBedrockRegion,
		})
		return p, trace.Wrap(err)

	default:
		return nil, trace.BadParameter("unsupported embeddings provider type: %T", model.GetSpec().GetEmbeddingsProvider())
	}
}
