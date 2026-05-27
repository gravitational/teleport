package summarizer

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/bedrock"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/openai"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/ragpipeline"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1"
	"github.com/gravitational/teleport/lib/events"
)

// pushSummaryToAccessGraph generates embeddings for the session summary and pushes them to the access graph.
// sessionDetails are used as input and output of this function:
//   - sessionDetails.summary is the input session summary for which to generate embeddings and push to the access graph.
//   - sessionDetails.hadEmbeddingsGenerated is set to true if embeddings were successfully generated and pushed to the access graph, and false otherwise.
func (s *SessionSummarizer) pushSummaryToAccessGraph(
	ctx context.Context,
	details *sessionDetails,
) error {
	availability, err := s.accessGraphAvailabilityChecker.Get(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	switch availability {
	case accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_NOT_IMPLEMENTED,
		accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_TRGM_UNAVAILABLE,
		accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_VECTOR_UNAVAILABLE:
		s.logger.DebugContext(ctx, "skipping access graph push: session search not available",
			"availability", availability)
		return nil
	}

	model, err := s.cache.GetRetrievalModel(ctx)
	if trace.IsNotFound(err) {
		s.logger.DebugContext(ctx, "no embedding provider configured, skipping pushing session summary to access graph")
		return nil // If no embedding provider is configured, skip pushing to access graph.
	} else if err != nil {
		return trace.Wrap(err)
	}

	embeddingProvider, err := s.newEmbeddingProvider(ctx, model)
	if err != nil {
		return trace.Wrap(err, "failed to create embedding provider")
	}
	proseProvider, err := s.newProseProvider(ctx, model)
	if err != nil {
		return trace.Wrap(err, "failed to create prose provider")
	}
	client, err := s.accessGraphClientGetter()
	if err != nil {
		return trace.Wrap(err)
	}
	processor, err := ragpipeline.NewProcessorWithConfig(
		embeddingProvider, proseProvider, ragpipeline.DefaultProcessorConfig(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	docs, err := processor.ProcessSession(ctx, details.summary)
	if err != nil {
		return trace.Wrap(err, "failed to process session")
	}

	var embeddingsToSend []*accessgraphv1.EmbeddingChunk
	for i, doc := range docs {
		embeddingsToSend = append(embeddingsToSend, &accessgraphv1.EmbeddingChunk{
			Values:     doc.Embedding,
			Chunk:      doc.Content,
			ChunkIndex: uint32(i),
		})
	}

	req := &accessgraphv1.StoreSessionSummaryRequest{
		SessionId:       details.sessionID.String(),
		Kind:            string(details.kind),
		Embeddings:      embeddingsToSend,
		SessionEndEvent: details.summary.GetSessionEndEvent(),
		Severity:        details.summary.GetEnhancedSummary().GetRiskLevel(),
	}

	if err := populateSessionSummaryRequest(req, details.sessionEnd); err != nil {
		return trace.Wrap(err)
	}

	_, err = client.StoreSessionSummary(ctx, req)
	details.hadEmbeddingsGenerated = err == nil
	return trace.Wrap(err, "failed to store session summary in access graph")
}

func populateSessionSummaryRequest(req *accessgraphv1.StoreSessionSummaryRequest, sessionEnd any) error {
	switch o := sessionEnd.(type) {
	case *apievents.SessionEnd:
		userTraits, err := traitsToStruct(o.UserMetadata.UserTraits)
		if err != nil {
			return trace.Wrap(err, "failed to convert user traits")
		}
		req.Username = o.User
		req.SessionStart = timestamppb.New(o.StartTime)
		req.SessionEnd = timestamppb.New(o.EndTime)
		req.UserRoles = o.UserRoles
		req.AccessRequestIds = o.AccessRequests
		req.Participants = o.Participants
		req.UserTraits = userTraits
		if o.Protocol == events.EventProtocolKube {
			podNamespace, podName := o.KubernetesPodNamespace, o.KubernetesPodName
			req.ResourceKind = types.KindKubernetesCluster
			req.ResourceName = o.KubernetesCluster
			req.ResourceId = o.KubernetesCluster
			req.HostId = o.ServerID
			req.ResourceLabels = o.KubernetesLabels
			req.ResourceProperties = &accessgraphv1.ResourceProperties{
				Type: &accessgraphv1.ResourceProperties_Kubernetes{
					Kubernetes: &accessgraphv1.KubernetesProperties{
						PodNamespace: &podNamespace,
						PodName:      &podName,
					},
				},
			}
		} else {
			hostname, addr := o.ServerHostname, o.ServerAddr
			req.ResourceKind = types.KindNode
			req.ResourceName = o.ServerHostname
			req.ResourceId = o.ServerID
			req.HostId = o.ServerID
			req.ResourceLabels = o.ServerLabels
			req.ResourceProperties = &accessgraphv1.ResourceProperties{
				Type: &accessgraphv1.ResourceProperties_Ssh{
					Ssh: &accessgraphv1.SSHProperties{
						ServerHostname: &hostname,
						ServerAddr:     &addr,
					},
				},
			}
		}
	case *apievents.DatabaseSessionEnd:
		userTraits, err := traitsToStruct(o.UserMetadata.UserTraits)
		if err != nil {
			return trace.Wrap(err, "failed to convert user traits")
		}
		dbName := o.DatabaseName
		req.Username = o.User
		req.SessionStart = timestamppb.New(o.StartTime)
		req.SessionEnd = timestamppb.New(o.EndTime)
		req.UserRoles = o.UserRoles
		req.AccessRequestIds = o.AccessRequests
		req.Participants = o.Participants
		req.UserTraits = userTraits
		req.ResourceKind = types.KindDatabase
		req.ResourceName = o.DatabaseName
		req.ResourceId = o.DatabaseService
		req.HostId = o.DatabaseService
		req.ResourceLabels = o.DatabaseLabels
		req.ResourceProperties = &accessgraphv1.ResourceProperties{
			Type: &accessgraphv1.ResourceProperties_Database{
				Database: &accessgraphv1.DatabaseProperties{
					DatabaseName: &dbName,
				},
			},
		}
	default:
		return trace.BadParameter("unsupported session end event type %T", sessionEnd)
	}
	return nil
}

func (s *SessionSummarizer) newEmbeddingProvider(
	ctx context.Context,
	model *summarizerv1pb.RetrievalModel,
) (EmbeddingProvider, error) {
	switch providerCfg := model.Spec.EmbeddingsProvider.(type) {
	case *summarizerv1pb.RetrievalModelSpec_Openai:
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
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return p, nil

	case *summarizerv1pb.RetrievalModelSpec_Bedrock:
		p, err := bedrock.NewEmbeddingProvider(ctx, bedrock.EmbeddingProviderConfig{
			Spec:              providerCfg.Bedrock,
			ClientFactory:     s.bedrockClientFactory,
			ModelResourceName: model.GetMetadata().GetName(),
			AWSConfigCache:    s.awsConfigCache,
			EnvBedrockRegion:  s.envBedrockRegion,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return p, nil
	default:
		return nil, trace.BadParameter("unsupported embedding provider type: %T", model.Spec.EmbeddingsProvider)
	}
}

func (s *SessionSummarizer) newProseProvider(
	ctx context.Context,
	model *summarizerv1pb.RetrievalModel,
) (ProseProvider, error) {
	provider, _, err := s.newProvider(ctx, model.GetSpec().GetInferenceModelName())
	if err != nil {
		return nil, trace.Wrap(err, "failed to create inference provider for prose generation")
	}
	return provider, nil
}

func traitsToStruct(traits wrappers.Traits) (*structpb.Struct, error) {
	if len(traits) == 0 {
		return nil, nil
	}
	m := make(map[string]any, len(traits))
	for k, vals := range traits {
		slice := make([]any, len(vals))
		for i, v := range vals {
			slice[i] = v
		}
		m[k] = slice
	}
	return structpb.NewStruct(m)
}
