package summarizer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apisummarizer "github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/bedrock"
	summopenai "github.com/gravitational/teleport/e/lib/auth/summarizer/openai"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/summarizerv1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/session"
)

type summarizerTestPlugin struct {
	clock         *clockwork.FakeClock
	enableBedrock bool
}

func (p *summarizerTestPlugin) GetName() string {
	return "auth.enterprise"
}

func (p *summarizerTestPlugin) RegisterProxyWebHandlers(_ any) error { return nil }
func (p *summarizerTestPlugin) RegisterAuthWebHandlers(_ any) error  { return nil }

func (p *summarizerTestPlugin) RegisterAuthServices(
	ctx context.Context, anyServer any, getClientCert func() (*tls.Certificate, error),
) error {
	authServer, ok := anyServer.(*auth.GRPCServer)
	if !ok {
		return trace.BadParameter("expected *auth.GRPCServer, got %T", anyServer)
	}

	svc, err := summarizerv1.NewService(summarizerv1.ServiceConfig{
		Authorizer:        authServer.Authorizer,
		Backend:           authServer.AuthServer,
		SummaryDownloader: authServer.AuthServer,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	srv, err := authServer.GetServer()
	if err != nil {
		return trace.Wrap(err)
	}
	summarizerv1pb.RegisterSummarizerServiceServer(srv, svc)

	openAIClientFactory := &fakeOpenAIClientFactory{
		clock: p.clock,
	}
	bedrockClientFactory := &bedrock.FakeClientFactory{
		Clock: p.clock,
	}
	summarizer, err := NewSessionSummarizer(SummarizerConfig{
		Backend:              authServer.AuthServer,
		Streamer:             authServer.AuthServer,
		SummaryUploader:      authServer.AuthServer,
		OpenAIClientFactory:  openAIClientFactory,
		BedrockClientFactory: bedrockClientFactory,
		Clock:                authServer.AuthServer.GetClock(),
		EnableBedrock:        p.enableBedrock,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	authServer.AuthServer.SetSummarizerService(summarizer)
	return nil
}

type summarizerTestTLSServerConfig struct {
	uploader      events.MultipartHandler
	enableBedrock bool
}

func newSummarizerTestTLSServer(t *testing.T, scfg summarizerTestTLSServerConfig) *authtest.TLSServer {
	sessionSummarizerProvider := &summarizer.SessionSummarizerProvider{}

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:                  scfg.uploader,
		SessionSummarizerProvider: sessionSummarizerProvider,
	})
	require.NoError(t, err)

	clock := clockwork.NewFakeClockAt(time.Date(2025, 6, 7, 10, 11, 0, 0, time.UTC))
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:                       t.TempDir(),
		Clock:                     clock,
		UploadHandler:             scfg.uploader,
		Streamer:                  streamer,
		SessionSummarizerProvider: sessionSummarizerProvider,
	})
	require.NoError(t, err)

	srv, err := as.NewTestTLSServer(func(cfg *authtest.TLSServerConfig) {
		cfg.APIConfig.PluginRegistry = plugin.NewRegistry()
		err = cfg.APIConfig.PluginRegistry.Add(&summarizerTestPlugin{
			clock:         clock,
			enableBedrock: scfg.enableBedrock,
		})
		require.NoError(t, err)
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
		require.NoError(t, as.Close())
	})

	return srv
}

func createTestUser(
	t *testing.T, srv *authtest.TLSServer, name string, opts ...authtest.CreateUserAndRoleOption,
) types.User {
	user, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		name,
		[]string{},
		[]types.Rule{
			{
				Resources: []string{
					types.KindInferenceSecret,
					types.KindInferenceModel,
					types.KindInferencePolicy,
					types.KindNode,
					types.KindDatabase,
					types.KindSession,
				},
				Verbs: []string{types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete, types.VerbList},
			},
			{
				Resources: []string{types.KindDatabaseServer},
				Verbs:     []string{types.VerbCreate, types.VerbUpdate},
			},
		},
		opts...,
	)
	require.NoError(t, err)
	return user
}

// createSummarizerConfig creates a summarizer configuration that matches all
// sessions from cluster "openai-cluster" to OpenAI inference provider, and all
// sessions from cluster "bedrock-cluster" to Bedrock inference provider.
func createSummarizerConfig(t *testing.T, ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) {
	_, err := sclt.CreateInferenceSecret(ctx, &summarizerv1pb.CreateInferenceSecretRequest{
		Secret: apisummarizer.NewInferenceSecret("openai-secret", &summarizerv1pb.InferenceSecretSpec{
			Value: "my-secret-value",
		}),
	})
	require.NoError(t, err)

	_, err = sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
		Model: apisummarizer.NewInferenceModel("openai-model", &summarizerv1pb.InferenceModelSpec{
			Provider: &summarizerv1pb.InferenceModelSpec_Openai{
				Openai: &summarizerv1pb.OpenAIProvider{
					OpenaiModelId:   "gpt-4o",
					ApiKeySecretRef: "openai-secret",
				},
			},
		}),
	})
	require.NoError(t, err)

	_, err = sclt.CreateInferencePolicy(ctx, &summarizerv1pb.CreateInferencePolicyRequest{
		Policy: apisummarizer.NewInferencePolicy("openai-policy", &summarizerv1pb.InferencePolicySpec{
			Kinds: []string{
				string(types.SSHSessionKind), string(types.KubernetesSessionKind), string(types.DatabaseSessionKind),
			},
			Filter: `session.cluster_name == "openai-cluster"`,
			Model:  "openai-model",
		}),
	})
	require.NoError(t, err)

	_, err = sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
		Model: apisummarizer.NewInferenceModel("bedrock-model", &summarizerv1pb.InferenceModelSpec{
			Provider: &summarizerv1pb.InferenceModelSpec_Bedrock{
				Bedrock: &summarizerv1pb.BedrockProvider{
					BedrockModelId: "amazon.nova-lite-v1:0",
					Region:         "us-west-2",
				},
			},
		}),
	})
	require.NoError(t, err)

	_, err = sclt.CreateInferencePolicy(ctx, &summarizerv1pb.CreateInferencePolicyRequest{
		Policy: apisummarizer.NewInferencePolicy("bedrock-policy", &summarizerv1pb.InferencePolicySpec{
			Kinds: []string{
				string(types.SSHSessionKind), string(types.KubernetesSessionKind), string(types.DatabaseSessionKind),
			},
			Filter: `session.cluster_name == "bedrock-cluster"`,
			Model:  "bedrock-model",
		}),
	})
	require.NoError(t, err)
}

type fakeOpenAIClientFactory struct {
	clock *clockwork.FakeClock
}

func (m *fakeOpenAIClientFactory) NewClient(opts ...option.RequestOption) summopenai.Client {
	return fakeOpenAIClient{
		clock: m.clock,
	}
}

type fakeOpenAIClient struct {
	clock *clockwork.FakeClock
}

func (m fakeOpenAIClient) NewChatCompletion(
	ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption,
) (*openai.ChatCompletion, error) {
	// Advance the clock to test if the inference end timestamp is captured.
	m.clock.Advance(10 * time.Second)

	systemPrompt := body.Messages[0].OfSystem.Content.OfString.Value
	var responsePrefix string
	switch {
	case strings.Contains(systemPrompt, "Analyze this terminal session"):
		responsePrefix = "The user wrote: "
	case strings.Contains(systemPrompt, "Analyze this database session"):
		responsePrefix = "The user queried: "
	default:
		return nil, errors.New("Unrecognized prompt")
	}

	content := body.Messages[1].OfUser.Content.OfString.Value
	switch content {
	case "cause an error":
		return nil, errors.New("OpenAI error")
	case "make the output too long":
		return &openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{{
				Message: openai.ChatCompletionMessage{
					Content: "",
				},
				FinishReason: "length",
			}},
		}, nil
	case "no choices":
		return &openai.ChatCompletion{}, nil
	default:
		return &openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{{
				Message: openai.ChatCompletionMessage{
					Content: responsePrefix + content,
				},
				FinishReason: "stop",
			}},
		}, nil
	}
}

func waitForSummary(
	t *testing.T, ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient, sid string,
) *summarizerv1pb.Summary {
	var sr *summarizerv1pb.GetSummaryResponse
	require.Eventually(t, func() bool {
		var err error
		sr, err = sclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
			SessionId: sid,
		})
		return err == nil
	}, time.Second*5, time.Millisecond*100)
	return sr.Summary
}

func TestSummarizer(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newSummarizerTestTLSServer(t, summarizerTestTLSServerConfig{
		uploader:      eventstest.NewMemoryUploader(),
		enableBedrock: true,
	})

	createTestUser(t, srv, "alice")
	clt, err := srv.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()
	createSummarizerConfig(t, ctx, sclt)

	serverID := "9d68b09f-8c0c-49a3-b54d-f8791f0c3941"

	// Test cases will be executed against all known inference provider kinds.
	cases := []struct {
		name    string
		setup   func(clusterName string, sessionID string) []apievents.AuditEvent
		state   summarizerv1pb.SummaryState
		summary string
		// errors are mostly inference-provider-specific, so they're indexed by
		// provider name.
		errors map[string]string
	}{
		{
			name: "SSH session",
			setup: func(clusterName string, sessionID string) []apievents.AuditEvent {
				return eventstest.GenerateTestSession(eventstest.SessionParams{
					ClusterName: clusterName,
					UserName:    "alice",
					SessionID:   sessionID,
					// OpenSSH nodes have cluster name as a suffix.
					ServerID:  serverID + ".testcluster",
					PrintData: []string{"net", "stat"},
				})
			},
			state:   summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
			summary: "The user wrote: netstat",
		},
		{
			name: "Kubernetes session",
			setup: func(clusterName, sessionID string) []apievents.AuditEvent {
				return eventstest.GenerateTestKubeSession(eventstest.SessionParams{
					ClusterName: clusterName,
					UserName:    "alice",
					SessionID:   sessionID,
					PrintData:   []string{"ps ", "aux"},
				})
			},
			state:   summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
			summary: "The user wrote: ps aux",
		},
		{
			name: "dynamic database session",
			setup: func(clusterName string, sessionID string) []apievents.AuditEvent {
				return eventstest.GenerateTestDBSession(eventstest.DBSessionParams{
					ClusterName:     clusterName,
					UserName:        "alice",
					SessionID:       sessionID,
					DatabaseService: "treasure-trove",
					Queries:         1,
				})
			},
			state:   summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
			summary: "The user queried: SELECT order_id FROM order where customer_id=0",
		},
		{
			name: "error",
			setup: func(clusterName string, sessionID string) []apievents.AuditEvent {
				return eventstest.GenerateTestSession(eventstest.SessionParams{
					ClusterName: clusterName,
					UserName:    "alice",
					SessionID:   sessionID,
					ServerID:    serverID,
					// This text will trigger the fake inference provider to fail.
					PrintData: []string{"cause an error"},
				})
			},
			state: summarizerv1pb.SummaryState_SUMMARY_STATE_ERROR,
			errors: map[string]string{
				"openai":  "OpenAI error",
				"bedrock": "operation error Bedrock Runtime: Converse, api error dummy: OMG",
			},
		},
		{
			name: "output too long",
			setup: func(clusterName string, sessionID string) []apievents.AuditEvent {
				return eventstest.GenerateTestSession(eventstest.SessionParams{
					ClusterName: clusterName,
					UserName:    "alice",
					SessionID:   sessionID,
					ServerID:    serverID,
					// This text will trigger the fake inference provider to simulate too
					// long model output.
					PrintData: []string{"make the output too long"},
				})
			},
			state: summarizerv1pb.SummaryState_SUMMARY_STATE_ERROR,
			errors: map[string]string{
				"openai":  "model response length limit exceeded",
				"bedrock": "model response length limit exceeded",
			},
		},
		{
			name: "no choices",
			setup: func(clusterName string, sessionID string) []apievents.AuditEvent {
				return eventstest.GenerateTestSession(eventstest.SessionParams{
					ClusterName: clusterName,
					UserName:    "alice",
					SessionID:   sessionID,
					ServerID:    serverID,
					// This text will trigger returning an empty choices slice.
					PrintData: []string{"no choices"},
				})
			},
			state: summarizerv1pb.SummaryState_SUMMARY_STATE_ERROR,
			errors: map[string]string{
				"openai":  "model returned no choices",
				"bedrock": "model returned a message without content",
			},
		},
	}

	for _, providerName := range []string{"openai", "bedrock"} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s provider %s", providerName, tc.name), func(t *testing.T) {
				ctx := t.Context()
				startTime := srv.Clock().Now()
				sessionID := uuid.NewString()
				sessEvents := tc.setup(providerName+"-cluster", sessionID)

				// Ingest an example session.
				stream, err := srv.Auth().CreateAuditStream(ctx, session.ID(sessionID))
				require.NoError(t, err)
				for _, event := range sessEvents {
					err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
					require.NoError(t, err)
				}
				err = stream.Complete(ctx)
				require.NoError(t, err)

				summary := waitForSummary(t, ctx, sclt, sessionID)

				sef, err := events.ToEventFields(sessEvents[len(sessEvents)-1])
				require.NoError(t, err)
				expectedEndEvent, err := structpb.NewStruct(sef)
				require.NoError(t, err)

				assert.Empty(t, cmp.Diff(
					&summarizerv1pb.Summary{
						SessionId:           sessionID,
						State:               tc.state,
						InferenceStartedAt:  timestamppb.New(startTime),
						InferenceFinishedAt: timestamppb.New(startTime.Add(10 * time.Second)),
						Content:             tc.summary,
						ModelName:           providerName + "-model",
						SessionEndEvent:     expectedEndEvent,
						ErrorMessage:        tc.errors[providerName],
					},
					summary,
					protocmp.Transform(),
				))
			})
		}
	}
}

func TestSummarizer_BedrockDisabled(t *testing.T) {
	ctx := t.Context()

	srv := newSummarizerTestTLSServer(t, summarizerTestTLSServerConfig{
		uploader:      eventstest.NewMemoryUploader(),
		enableBedrock: false,
	})

	// What we test here is a secondary line of defense in case a Bedrock
	// resource somehow appears in the backend. Since a server that has Bedrock
	// disabled will also disable adding Bedrock models, we need to inject one by
	// creating a separate summarizer service with the same backend, but Bedrock
	// enabled.
	ssrvWithBedrock, err := local.NewSummarizerService(local.SummarizerServiceConfig{
		Backend:       srv.AuthServer.Backend,
		EnableBedrock: true,
	})
	require.NoError(t, err)

	_, err = ssrvWithBedrock.CreateInferenceModel(ctx, apisummarizer.NewInferenceModel(
		"bedrock-model",
		&summarizerv1pb.InferenceModelSpec{
			Provider: &summarizerv1pb.InferenceModelSpec_Bedrock{
				Bedrock: &summarizerv1pb.BedrockProvider{
					BedrockModelId: "amazon.nova-lite-v1:0",
					Region:         "us-west-2",
				},
			},
		}),
	)
	require.NoError(t, err)

	_, err = ssrvWithBedrock.CreateInferencePolicy(ctx, apisummarizer.NewInferencePolicy(
		"bedrock-policy",
		&summarizerv1pb.InferencePolicySpec{
			Kinds: []string{
				string(types.SSHSessionKind), string(types.KubernetesSessionKind), string(types.DatabaseSessionKind),
			},
			Filter: `session.cluster_name == "bedrock-cluster"`,
			Model:  "bedrock-model",
		}),
	)
	require.NoError(t, err)

	sessEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
		ClusterName: "bedrock-cluster",
	})

	err = srv.AuthServer.SessionSummarizerProvider.SessionSummarizer().SummarizeSSH(
		ctx, sessEvents[len(sessEvents)-1].(*apievents.SessionEnd))
	require.Error(t, err)
	assert.ErrorIs(t, err, trace.AccessDenied("Amazon Bedrock models are unavailable in Teleport Cloud"))
}

func TestSummarizerNoEndEvent(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	uploader := eventstest.NewMemoryUploader()

	sshSessionID := "24d8542a-8a7d-4683-a59b-18adc3a71f11"
	serverID := "9d68b09f-8c0c-49a3-b54d-f8791f0c3941"
	evts := eventstest.GenerateTestSession(eventstest.SessionParams{
		ClusterName: "openai-cluster",
		UserName:    "alice",
		SessionID:   sshSessionID,
		ServerID:    serverID,
		PrintData:   []string{"net", "stat"},
	})

	// Ingest an example session.
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: uploader,
	})
	require.NoError(t, err)
	stream, err := streamer.CreateAuditStream(ctx, session.ID(sshSessionID))
	require.NoError(t, err)
	for _, event := range evts {
		err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
		require.NoError(t, err)
	}
	st := <-stream.Status()
	uploadID := st.UploadID
	err = stream.Close(ctx)
	require.NoError(t, err)

	// Create the auth server.
	srv := newSummarizerTestTLSServer(t, summarizerTestTLSServerConfig{
		uploader: uploader,
	})

	createTestUser(t, srv, "alice")
	clt, err := srv.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()
	createSummarizerConfig(t, ctx, sclt)

	stream, err = srv.Auth().ResumeAuditStream(ctx, session.ID(sshSessionID), uploadID)
	require.NoError(t, err)
	startTime := srv.Clock().Now()
	err = stream.Complete(ctx)
	require.NoError(t, err)

	summary := waitForSummary(t, ctx, sclt, sshSessionID)

	sef, err := events.ToEventFields(evts[len(evts)-1])
	require.NoError(t, err)
	expectedEndEvent, err := structpb.NewStruct(sef)
	require.NoError(t, err)

	assert.Empty(t, cmp.Diff(
		&summarizerv1pb.Summary{
			SessionId:           sshSessionID,
			State:               summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
			InferenceStartedAt:  timestamppb.New(startTime),
			InferenceFinishedAt: timestamppb.New(startTime.Add(10 * time.Second)),
			Content:             "The user wrote: netstat",
			ModelName:           "openai-model",
			SessionEndEvent:     expectedEndEvent,
			ErrorMessage:        "",
		},
		summary,
		protocmp.Transform(),
	))
}
