package summarizer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apisummarizer "github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/bedrock"
	summopenai "github.com/gravitational/teleport/e/lib/auth/summarizer/openai"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/summarizerv1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/session"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// mockUsageReporter is a mock implementation of UsageReporter for testing.
type mockUsageReporter struct {
	mu     sync.Mutex
	events []usagereporter.Anonymizable
}

func (m *mockUsageReporter) AnonymizeAndSubmit(events ...usagereporter.Anonymizable) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, events...)
}

func (m *mockUsageReporter) getEvents() []usagereporter.Anonymizable {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.events)
}

func (m *mockUsageReporter) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
}

type summarizerTestPlugin struct {
	clock                            *clockwork.FakeClock
	enableBedrockWithoutRestrictions bool
	decrypter                        events.DecryptionWrapper
	encrypter                        events.EncryptionWrapper
	envBedrockRegion                 string
	envBedrockModelID                string
	mockEmitter                      *eventstest.MockRecorderEmitter
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
		Emitter:           authServer.AuthServer.GetEmitter(),
		Decrypter:         p.decrypter,
		UsageReporter:     authServer.AuthServer.UsageReporter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	srv, err := authServer.GetServer()
	if err != nil {
		return trace.Wrap(err)
	}
	summarizerv1pb.RegisterSummarizerServiceServer(srv, svc)

	cfgCache, err := createCache()
	if err != nil {
		return trace.Wrap(err)
	}

	openAIClientFactory := &fakeOpenAIClientFactory{
		clock: p.clock,
	}
	bedrockClientFactory := &bedrock.FakeClientFactory{
		Clock: p.clock,
	}

	emitter := apievents.Emitter(authServer.AuthServer)
	if p.mockEmitter != nil {
		emitter = p.mockEmitter
	}

	summarizer, err := NewSessionSummarizer(SummarizerConfig{
		Backend:                          authServer.AuthServer,
		Streamer:                         authServer.AuthServer,
		SummaryUploader:                  authServer.AuthServer,
		OpenAIClientFactory:              openAIClientFactory,
		BedrockClientFactory:             bedrockClientFactory,
		Clock:                            authServer.AuthServer.GetClock(),
		EnableBedrockWithoutRestrictions: p.enableBedrockWithoutRestrictions,
		Encrypter:                        p.encrypter,
		AWSConfigCache:                   cfgCache,
		EnvBedrockRegion:                 p.envBedrockRegion,
		EnvBedrockModelID:                p.envBedrockModelID,
		UsageReporter:                    authServer.AuthServer.UsageReporter,
		Emitter:                          emitter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	authServer.AuthServer.SetSummarizerService(summarizer)
	return nil
}

type summarizerTestTLSServerConfig struct {
	uploader                         events.MultipartHandler
	enableBedrockWithoutRestrictions bool
	encrypter                        events.EncryptionWrapper
	decrypter                        events.DecryptionWrapper
	envBedrockRegion                 string
	envBedrockModelID                string
	usageReporter                    usagereporter.UsageReporter
	mockEmitter                      *eventstest.MockRecorderEmitter
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
	t.Cleanup(func() { require.NoError(t, as.Close()) })

	// Override usage reporter if provided
	if scfg.usageReporter != nil {
		as.AuthServer.SetUsageReporter(scfg.usageReporter)
	}

	srv, err := as.NewTestTLSServer(func(cfg *authtest.TLSServerConfig) {
		cfg.APIConfig.PluginRegistry = plugin.NewRegistry()
		err = cfg.APIConfig.PluginRegistry.Add(&summarizerTestPlugin{
			clock:                            clock,
			enableBedrockWithoutRestrictions: scfg.enableBedrockWithoutRestrictions,
			decrypter:                        scfg.decrypter,
			encrypter:                        scfg.encrypter,
			envBedrockRegion:                 scfg.envBedrockRegion,
			envBedrockModelID:                scfg.envBedrockModelID,
			mockEmitter:                      scfg.mockEmitter,
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
	content := body.Messages[1].OfUser.Content.OfString.Value

	// Handle structured responses for command analysis.
	if strings.Contains(systemPrompt, "analyzing a single command from a session recording") {
		return m.handleCommandAnalysis(content)
	}

	// Handle structured responses for session analysis.
	if strings.Contains(systemPrompt, "You are preparing a summary") {
		return m.handleSessionAnalysis(content)
	}

	// Handle simple summarization.
	var responsePrefix string
	switch {
	case strings.Contains(systemPrompt, "Analyze this terminal session"):
		responsePrefix = "The user wrote: "
	case strings.Contains(systemPrompt, "Analyze this database session"):
		responsePrefix = "The user queried: "
	default:
		return nil, errors.New("unrecognized prompt")
	}

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
			Usage: openai.CompletionUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
			},
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
			Usage: openai.CompletionUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
			},
		}, nil
	}
}

func (m fakeOpenAIClient) handleCommandAnalysis(content string) (*openai.ChatCompletion, error) {
	if strings.Contains(content, "trigger enhanced error") || strings.Contains(content, "trigger command error") {
		return nil, errors.New("enhanced command analysis error")
	}

	response := schema.CommandAnalysis{
		Command:          "test-command",
		Category:         "other",
		Success:          true,
		RiskLevel:        "low",
		RiskScore:        10,
		ThreatCategory:   "none",
		TimelineTitle:    "Executed test command",
		ShortDescription: "Test command executed",
		Description:      "A test command was executed during the session.",
	}

	resp, err := json.Marshal(response)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: string(resp),
			},
			FinishReason: "stop",
		}},
		Usage: openai.CompletionUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
		},
	}, nil
}

func (m fakeOpenAIClient) handleSessionAnalysis(content string) (*openai.ChatCompletion, error) {
	if strings.Contains(content, "trigger enhanced error") {
		return nil, errors.New("enhanced session analysis error")
	}

	response := schema.SessionAnalysis{
		ShortDescription:      "Test session with commands",
		SessionDescription:    "The user executed test commands during this session.",
		NotableCommandIndexes: []int{0},
		RiskLevel:             "low",
		RiskScore:             15,
	}

	resp, err := json.Marshal(response)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: string(resp),
			},
			FinishReason: "stop",
		}},
		Usage: openai.CompletionUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
		},
	}, nil
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
		return err == nil && sr.Summary.State != summarizerv1pb.SummaryState_SUMMARY_STATE_PENDING
	}, time.Second*5, time.Millisecond*100)
	return sr.Summary
}

func TestSummarizer(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	mockReporter := &mockUsageReporter{}
	mockEmitter := &eventstest.MockRecorderEmitter{}
	srv := newSummarizerTestTLSServer(t, summarizerTestTLSServerConfig{
		uploader:                         eventstest.NewMemoryUploader(),
		enableBedrockWithoutRestrictions: true,
		usageReporter:                    mockReporter,
		mockEmitter:                      mockEmitter,
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
				mockReporter.reset()
				mockEmitter.Reset()
				startTime := srv.Clock().Now()
				sessionID := uuid.NewString()
				sessEvents := tc.setup(providerName+"-cluster", sessionID)

				ingestSession(t, ctx, srv.Auth(), sessionID, sessEvents)
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

				// Verify usage reporter was called with the correct event
				allEvents := mockReporter.getEvents()

				// Filter for SessionSummaryCreateEvent only
				var summaryEvents []*usagereporter.SessionSummaryCreateEvent
				for _, e := range allEvents {
					if summaryEvent, ok := e.(*usagereporter.SessionSummaryCreateEvent); ok {
						summaryEvents = append(summaryEvents, summaryEvent)
					}
				}

				require.Len(t, summaryEvents, 1, "Expected exactly one SessionSummaryCreateEvent to be emitted")
				summaryEvent := summaryEvents[0]

				// Verify the event fields
				assert.Equal(t, providerName, summaryEvent.Provider, "Provider mismatch")
				if tc.state == summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS {
					assert.Equal(t, uint64(100), summaryEvent.TotalInputTokens, "Input tokens mismatch")
					assert.Equal(t, uint64(50), summaryEvent.TotalOutputTokens, "Output tokens mismatch")
				}
				assert.Equal(t, tc.state == summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS, summaryEvent.Success, "Success flag mismatch")
				resourceType, resourceName := getResourceNameFromSessionEnd(sessEvents[len(sessEvents)-1])
				assert.Equal(t, resourceName, summaryEvent.ResourceName, "Resource name mismatch")
				assert.Equal(t, resourceType, summaryEvent.SessionType, "Resource type mismatch")

				auditEvents := mockEmitter.Events()
				var summaryCreateEvents []*apievents.SessionSummarized
				for _, evt := range auditEvents {
					if summaryCreate, ok := evt.(*apievents.SessionSummarized); ok {
						summaryCreateEvents = append(summaryCreateEvents, summaryCreate)
					}
				}

				require.Len(t, summaryCreateEvents, 1, "Expected exactly one SessionSummarized audit event to be emitted")
				auditEvent := summaryCreateEvents[0]

				assert.Equal(t, events.SessionSummarizedEvent, auditEvent.GetType(), "Event type mismatch")
				assert.Equal(t, sessionID, auditEvent.SessionID, "Session ID mismatch")
				assert.Equal(t, providerName+"-model", auditEvent.ModelName, "Model name mismatch")
				assert.Equal(t, tc.state == summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS, auditEvent.Success, "Success flag mismatch in audit event")
				assert.Equal(t, startTime, auditEvent.InferenceStartedAt, "Inference started time mismatch")
				assert.Equal(t, startTime.Add(10*time.Second), auditEvent.InferenceFinishedAt, "Inference finished time mismatch")

				if tc.state == summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS {
					assert.Equal(t, events.SessionSummarizedCode, auditEvent.GetCode(), "Event code mismatch for success")
				} else {
					assert.Equal(t, events.SessionSummarizedErrorCode, auditEvent.GetCode(), "Event code mismatch for error")
					assert.Equal(t, tc.errors[providerName], auditEvent.Error, "Error message mismatch in audit event")
				}

				switch resourceType {
				case "ssh":
					assert.Equal(t, string(types.SSHSessionKind), auditEvent.SessionType, "Session type mismatch")
					assert.Equal(t, resourceName, auditEvent.ServerID, "Server ID mismatch")
					assert.Equal(t, "alice", auditEvent.Username, "Username mismatch")
				case "k8s":
					assert.Equal(t, string(types.KubernetesSessionKind), auditEvent.SessionType, "Session type mismatch")
					assert.Equal(t, resourceName, auditEvent.KubernetesCluster, "Kubernetes cluster mismatch")
					assert.Equal(t, "alice", auditEvent.Username, "Username mismatch")
				case "db":
					assert.Equal(t, string(types.DatabaseSessionKind), auditEvent.SessionType, "Session type mismatch")
					assert.Equal(t, resourceName, auditEvent.DatabaseName, "Database name mismatch")
					assert.Equal(t, "alice", auditEvent.Username, "Username mismatch")
				}
			})
		}
	}
}

func getResourceNameFromSessionEnd(evt apievents.AuditEvent) (string, string) {
	switch e := evt.(type) {
	case *apievents.SessionEnd:
		if e.KubernetesCluster != "" {
			return "k8s", e.KubernetesCluster
		}
		return "ssh", e.ServerID
	case *apievents.DatabaseSessionEnd:
		return "db", e.DatabaseName
	default:
		return "", ""
	}
}

func TestSummarizer_BedrockConfigFromEnvironment(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	srv := newSummarizerTestTLSServer(t, summarizerTestTLSServerConfig{
		uploader:                         eventstest.NewMemoryUploader(),
		envBedrockRegion:                 "eu-central-1",
		envBedrockModelID:                "anthropic.claude-3-5-sonnet-20240620-v1:0",
		enableBedrockWithoutRestrictions: true,
	})

	createTestUser(t, srv, "alice")
	clt, err := srv.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	// Create a model that expects configuration to be injected from the process
	// environment.
	_, err = sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
		Model: apisummarizer.NewInferenceModel(
			"test-model",
			&summarizerv1pb.InferenceModelSpec{
				Provider: &summarizerv1pb.InferenceModelSpec_Bedrock{
					Bedrock: &summarizerv1pb.BedrockProvider{
						BedrockModelId: "{{env.bedrock_model_id}}",
						Region:         "{{env.bedrock_region}}",
					},
				},
			},
		),
	})
	require.NoError(t, err)

	_, err = sclt.CreateInferencePolicy(ctx, &summarizerv1pb.CreateInferencePolicyRequest{
		Policy: apisummarizer.NewInferencePolicy(
			"test-policy",
			&summarizerv1pb.InferencePolicySpec{
				Kinds: []string{string(types.SSHSessionKind)},
				Model: "test-model",
			}),
	})
	require.NoError(t, err)

	// Generate a session and test the model.
	sid := uuid.NewString()
	sessEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
		PrintData: []string{"respond with region and Bedrock model ID"},
		SessionID: sid,
	})
	ingestSession(t, ctx, srv.Auth(), sid, sessEvents)

	summary := waitForSummary(t, ctx, sclt, sid)
	assert.Equal(t, "eu-central-1, anthropic.claude-3-5-sonnet-20240620-v1:0", summary.Content)
}

func TestSummarizer_BedrockRestricted(t *testing.T) {
	ctx := t.Context()

	srv := newSummarizerTestTLSServer(t, summarizerTestTLSServerConfig{
		uploader:                         eventstest.NewMemoryUploader(),
		enableBedrockWithoutRestrictions: false,
	})

	// What we test here is a secondary line of defense in case a Bedrock
	// resource somehow appears in the backend. Since a server that has Bedrock
	// disabled will also disable adding Bedrock models, we need to inject one by
	// creating a separate summarizer service with the same backend, but Bedrock
	// enabled.
	ssrvWithBedrock, err := local.NewSummarizerService(local.SummarizerServiceConfig{
		Backend:                          srv.AuthServer.Backend,
		EnableBedrockWithoutRestrictions: true,
	})
	require.NoError(t, err)

	// Create a model that uses default AWS authentication.
	modelSpec := &summarizerv1pb.InferenceModelSpec{
		Provider: &summarizerv1pb.InferenceModelSpec_Bedrock{
			Bedrock: &summarizerv1pb.BedrockProvider{
				BedrockModelId: "amazon.nova-lite-v1:0",
				Region:         "us-west-2",
			},
		},
	}
	_, err = ssrvWithBedrock.CreateInferenceModel(ctx, apisummarizer.NewInferenceModel(
		"bedrock-model", modelSpec,
	))
	require.NoError(t, err)

	// Create a model that uses authentication via OIDC.
	oidcModelSpec := proto.CloneOf(modelSpec)
	oidcModelSpec.GetBedrock().Integration = "dummy-integration"
	_, err = ssrvWithBedrock.CreateInferenceModel(ctx, apisummarizer.NewInferenceModel(
		"bedrock-oidc-model", oidcModelSpec,
	))
	require.NoError(t, err)

	// Create a policy that uses default AWS authentication.
	policySpec := &summarizerv1pb.InferencePolicySpec{
		Kinds: []string{
			string(types.SSHSessionKind), string(types.KubernetesSessionKind), string(types.DatabaseSessionKind),
		},
		Filter: `session.cluster_name == "bedrock-cluster"`,
		Model:  "bedrock-model",
	}
	_, err = ssrvWithBedrock.CreateInferencePolicy(ctx, apisummarizer.NewInferencePolicy(
		"bedrock-policy", policySpec,
	))
	require.NoError(t, err)

	// Create a policy that uses authentication via OIDC.
	oidcPolicySpec := proto.CloneOf(policySpec)
	oidcPolicySpec.Filter = `session.cluster_name == "bedrock-oidc-cluster"`
	oidcPolicySpec.Model = "bedrock-oidc-model"
	_, err = ssrvWithBedrock.CreateInferencePolicy(ctx, apisummarizer.NewInferencePolicy(
		"bedrock-oidc-policy", oidcPolicySpec,
	))
	require.NoError(t, err)

	// Test a model without OIDC. This should be forbidden.
	sessEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
		ClusterName: "bedrock-cluster",
	})

	err = srv.AuthServer.SessionSummarizerProvider.SessionSummarizer().SummarizeSSH(
		ctx, sessEvents[len(sessEvents)-1].(*apievents.SessionEnd))
	require.Error(t, err)
	assert.ErrorIs(t, err, trace.AccessDenied(
		"only the default model is allowed to use Amazon Bedrock without OIDC in Teleport Cloud",
	))

	// Test a model with OIDC integration. This should be allowed.
	oidcSID := uuid.NewString()
	oidcSessEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
		ClusterName: "bedrock-oidc-cluster",
		SessionID:   oidcSID,
		PrintData:   []string{"ls"},
	})
	ingestSession(t, ctx, srv.Auth(), oidcSID, oidcSessEvents)

	createTestUser(t, srv, "alice")
	clt, err := srv.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()
	summary := waitForSummary(t, ctx, sclt, oidcSID)
	assert.Equal(t, "The user wrote: ls", summary.Content)
}

func TestSummarizerEncrypedDecrypted(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	memoryUploader := eventstest.NewMemoryUploader()

	srv := newSummarizerTestTLSServer(t,
		summarizerTestTLSServerConfig{
			uploader:  memoryUploader,
			encrypter: &fakeEncryptedIO{},
			decrypter: &fakeEncryptedIO{},
		})

	createTestUser(t, srv, "alice")
	clt, err := srv.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()
	createSummarizerConfig(t, ctx, sclt)

	encryptedSessionID := "24d8542a-8a7d-4683-a59b-18adc3a71f11"

	startTime := srv.Clock().Now()

	// Ingest an example session.
	eventsData := eventstest.GenerateTestSession(eventstest.SessionParams{
		ClusterName: "openai-cluster",
		UserName:    "alice",
		SessionID:   encryptedSessionID,
		// OpenSSH nodes have cluster name as a suffix.
		ServerID:  "9d68b09f-8c0c-49a3-b54d-f8791f0c3941.testcluster",
		PrintData: []string{"net", "stat"},
	})
	ingestSession(t, ctx, srv.Auth(), encryptedSessionID, eventsData)

	summary := waitForSummary(t, ctx, sclt, encryptedSessionID)

	sef, err := events.ToEventFields(eventsData[len(eventsData)-1])
	require.NoError(t, err)
	expectedEndEvent, err := structpb.NewStruct(sef)
	require.NoError(t, err)

	assert.Empty(t, cmp.Diff(
		&summarizerv1pb.Summary{
			SessionId:           encryptedSessionID,
			State:               summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
			InferenceStartedAt:  timestamppb.New(startTime),
			InferenceFinishedAt: timestamppb.New(startTime.Add(10 * time.Second)),
			Content:             "The user wrote: netstat",
			ModelName:           "openai-model",
			SessionEndEvent:     expectedEndEvent,
		},
		summary,
		protocmp.Transform(),
	))
	writer := &memBuffer{}
	err = memoryUploader.DownloadSummary(ctx, session.ID(encryptedSessionID), writer)
	require.NoError(t, err)

	// Verify that the uploaded summary is indeed encrypted.
	encryptedData := writer.Bytes()
	require.True(t, bytes.HasPrefix(encryptedData, []byte(agePrefix)))
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

func TestSummarizerEnhancedSession(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newSummarizerTestTLSServer(t, summarizerTestTLSServerConfig{
		uploader:                         eventstest.NewMemoryUploader(),
		enableBedrockWithoutRestrictions: true,
	})

	createTestUser(t, srv, "alice")
	clt, err := srv.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()
	createSummarizerConfig(t, ctx, sclt)

	ingestEnhancedSession := func(t *testing.T, clusterName, command string) *summarizerv1pb.Summary {
		ctx := t.Context()
		sessionID := uuid.NewString()
		sessEvents := generateEnhancedTestSession(clusterName, "alice", sessionID, command)

		stream, err := srv.Auth().CreateAuditStream(ctx, session.ID(sessionID))
		require.NoError(t, err)
		for _, event := range sessEvents {
			require.NoError(t, stream.RecordEvent(ctx, eventstest.PrepareEvent(event)))
		}
		require.NoError(t, stream.Complete(ctx))

		return waitForSummary(t, ctx, sclt, sessionID)
	}

	for _, providerName := range []string{"openai", "bedrock"} {
		t.Run(providerName+" provider success", func(t *testing.T) {
			summary := ingestEnhancedSession(t, providerName+"-cluster", "ls -la")

			require.Equal(t, summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS, summary.State)
			require.Equal(t, providerName+"-model", summary.ModelName)
			require.Empty(t, summary.Content)
			require.NotNil(t, summary.EnhancedSummary)
			require.Equal(t, "Test session with commands", summary.EnhancedSummary.ShortDescription)
			require.Equal(t, summarizerv1pb.RiskLevel_RISK_LEVEL_LOW, summary.EnhancedSummary.RiskLevel)
		})

		t.Run(providerName+" provider error", func(t *testing.T) {
			summary := ingestEnhancedSession(t, providerName+"-cluster", "trigger enhanced error")

			require.Equal(t, summarizerv1pb.SummaryState_SUMMARY_STATE_ERROR, summary.State)
			require.Equal(t, providerName+"-model", summary.ModelName)
			require.Contains(t, summary.ErrorMessage, "enhanced session analysis error")
			require.Empty(t, summary.Content)
			require.Nil(t, summary.EnhancedSummary)
		})

		t.Run(providerName+" command analysis failure is graceful", func(t *testing.T) {
			summary := ingestEnhancedSession(t, providerName+"-cluster", "trigger command error")

			require.Equal(t, summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS, summary.State)
			require.Equal(t, providerName+"-model", summary.ModelName)
			require.Empty(t, summary.ErrorMessage)
			require.NotNil(t, summary.EnhancedSummary)
			require.NotNil(t, summary.EnhancedSummary.NeedsFurtherReview)
			require.Equal(t, summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED, *summary.EnhancedSummary.NeedsFurtherReview)
		})
	}
}

// TestSummarizerSessionRouting verifies that SSH and Kubernetes sessions use the
// enhanced summarization path while database sessions use the simple summarization path
func TestSummarizerSessionRouting(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newSummarizerTestTLSServer(t, summarizerTestTLSServerConfig{
		uploader:                         eventstest.NewMemoryUploader(),
		enableBedrockWithoutRestrictions: true,
	})

	createTestUser(t, srv, "alice")
	clt, err := srv.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()
	createSummarizerConfig(t, ctx, sclt)

	t.Run("SSH sessions get an enhanced summary", func(t *testing.T) {
		ctx := t.Context()
		sessionID := uuid.NewString()
		sessEvents := generateEnhancedTestSession("openai-cluster", "alice", sessionID, "ls -la")

		ingestSession(t, ctx, srv.Auth(), sessionID, sessEvents)
		summary := waitForSummary(t, ctx, sclt, sessionID)

		require.Equal(t, summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS, summary.State)
		require.NotNil(t, summary.EnhancedSummary, "SSH sessions should produce an enhanced summary via summarizeSession")
		require.Empty(t, summary.Content, "SSH sessions with commands should not produce simple content")
	})

	t.Run("Kubernetes sessions get an enhanced summary", func(t *testing.T) {
		ctx := t.Context()
		sessionID := uuid.NewString()
		sessEvents := eventstest.GenerateTestKubeSession(eventstest.SessionParams{
			ClusterName: "openai-cluster",
			UserName:    "alice",
			SessionID:   sessionID,
			PrintData: []string{
				"\x1b[?2004h",
				"kubectl get pods",
				"\x1b[?2004l",
				"\r\n",
				"NAME    READY   STATUS    RESTARTS   AGE",
			},
		})

		ingestSession(t, ctx, srv.Auth(), sessionID, sessEvents)
		summary := waitForSummary(t, ctx, sclt, sessionID)

		require.Equal(t, summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS, summary.State)
		require.NotNil(t, summary.EnhancedSummary, "Kubernetes sessions should produce an enhanced summary via summarizeSession")
		require.Empty(t, summary.Content, "Kubernetes sessions with commands should not produce simple content")
	})

	t.Run("Database sessions use simple summarization", func(t *testing.T) {
		ctx := t.Context()
		sessionID := uuid.NewString()
		sessEvents := eventstest.GenerateTestDBSession(eventstest.DBSessionParams{
			ClusterName:     "openai-cluster",
			UserName:        "alice",
			SessionID:       sessionID,
			DatabaseService: "treasure-trove",
			Queries:         1,
		})

		ingestSession(t, ctx, srv.Auth(), sessionID, sessEvents)
		summary := waitForSummary(t, ctx, sclt, sessionID)

		require.Equal(t, summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS, summary.State)
		require.Nil(t, summary.EnhancedSummary, "Database sessions should not produce an enhanced summary")
		require.NotEmpty(t, summary.Content, "Database sessions should produce simple content via summarizeSimple")
		require.Contains(t, summary.Content, "The user queried:", "Database sessions should use the database prompt")
	})
}

// generateEnhancedTestSession creates session events with bracketed paste mode
// escape sequences to trigger the enhanced summarization path.
func generateEnhancedTestSession(clusterName, userName, sessionID, command string) []apievents.AuditEvent {
	params := eventstest.SessionParams{
		ClusterName: clusterName,
		UserName:    userName,
		SessionID:   sessionID,
		ServerID:    "9d68b09f-8c0c-49a3-b54d-f8791f0c3941",
		PrintData: []string{
			"\x1b[?2004h",         // Enable bracketed paste mode
			command,               // Command input
			"\x1b[?2004l",         // Disable bracketed paste mode
			"\r\n",                // Enter key
			"total 0\ndrwxr-xr-x", // Command output
		},
	}
	return eventstest.GenerateTestSession(params)
}

// encryptedIO is really just a reversible transform, so we fake encryption by encoding/decoding as hex
type fakeEncryptedIO struct{}

type fakeEncrypter struct {
	inner  io.WriteCloser
	writer io.Writer
}

func (f *fakeEncrypter) Write(out []byte) (int, error) {
	return f.writer.Write(out)
}

func (f *fakeEncrypter) Close() error {
	return f.inner.Close()
}

func (f *fakeEncryptedIO) WithEncryption(ctx context.Context, writer io.WriteCloser) (io.WriteCloser, error) {
	writer.Write([]byte("age-encryption.org")) // fake header
	hexWriter := hex.NewEncoder(writer)
	encrypter := &fakeEncrypter{
		inner:  writer,
		writer: hexWriter,
	}

	return encrypter, nil
}

const agePrefix = "age-encryption.org"

func (f *fakeEncryptedIO) WithDecryption(ctx context.Context, reader io.Reader) (io.Reader, error) {
	// read and discard fake header
	header := make([]byte, len(agePrefix))
	_, err := io.ReadFull(reader, header)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if string(header) != agePrefix {
		return nil, trace.BadParameter("invalid encryption header")
	}
	return hex.NewDecoder(reader), nil
}

// memBuffer is a in-memory byte buffer that implements both io.Writer and
// io.WriterAt interfaces.
type memBuffer struct {
	buf manager.WriteAtBuffer
	// mu is a mutex to protect concurrent writes to the buffer. Even though the
	// underlying buffer is thread-safe, we need to prevent a race condition in
	// [MemBuffer.Write] between checking the length of the buffer and writing to
	// it.
	mu sync.Mutex
}

func (b *memBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.WriteAt(p, int64(len(b.buf.Bytes())))
}

func (b *memBuffer) WriteAt(p []byte, pos int64) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.WriteAt(p, pos)
}

// Bytes return the underlying byte slice.
func (b *memBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Bytes()
}

func ingestSession(
	t *testing.T, ctx context.Context, asrv *auth.Server, sid string, sessEvents []apievents.AuditEvent,
) {
	stream, err := asrv.CreateAuditStream(ctx, session.ID(sid))
	require.NoError(t, err)
	for _, event := range sessEvents {
		err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
		require.NoError(t, err)
	}
	err = stream.Complete(ctx)
	require.NoError(t, err)
}

func createCache() (*awsconfig.Cache, error) {
	awsOIDCIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "dummy-integration"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:sts::123456789012:role/TestRole",
		},
	)
	if err != nil {
		return nil, err
	}
	oidcIntegrationClient := mocks.FakeOIDCIntegrationClient{
		Integration: awsOIDCIntegration,
	}
	return awsconfig.NewCache(
		awsconfig.WithDefaults(
			awsconfig.WithOIDCIntegrationClient(&oidcIntegrationClient),
			awsconfig.WithSTSClientProvider(func(c aws.Config) awsconfig.STSClient {
				return &mocks.STSClient{}
			}),
		),
	)
}
