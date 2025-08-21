package summarizer

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apisummarizer "github.com/gravitational/teleport/api/types/summarizer"
	summopenai "github.com/gravitational/teleport/e/lib/auth/summarizer/openai"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/summarizerv1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/session"
)

type summarizerTestPlugin struct {
	clock *clockwork.FakeClock
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
	summarizer, err := NewSessionSummarizer(SummarizerConfig{
		Backend:             authServer.AuthServer,
		Streamer:            authServer.AuthServer,
		ResourceGetter:      authServer.AuthServer,
		SummaryUploader:     authServer.AuthServer,
		OpenAIClientFactory: openAIClientFactory,
		Clock:               authServer.AuthServer.GetClock(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	authServer.AuthServer.SetSummarizerService(summarizer)
	return nil
}

func newSummarizerTestTLSServer(t *testing.T, uploader events.MultipartHandler) *authtest.TLSServer {
	sessionSummarizerProvider := &summarizer.SessionSummarizerProvider{}

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:                  uploader,
		SessionSummarizerProvider: sessionSummarizerProvider,
	})
	require.NoError(t, err)

	clock := clockwork.NewFakeClockAt(time.Date(2025, 6, 7, 10, 11, 0, 0, time.UTC))
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:                       t.TempDir(),
		Clock:                     clock,
		UploadHandler:             uploader,
		Streamer:                  streamer,
		SessionSummarizerProvider: sessionSummarizerProvider,
	})
	require.NoError(t, err)

	srv, err := as.NewTestTLSServer(func(cfg *authtest.TLSServerConfig) {
		cfg.APIConfig.PluginRegistry = plugin.NewRegistry()
		err = cfg.APIConfig.PluginRegistry.Add(&summarizerTestPlugin{
			clock: clock,
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

func createSummarizerConfig(t *testing.T, ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) {
	_, err := sclt.CreateInferenceSecret(ctx, &summarizerv1pb.CreateInferenceSecretRequest{
		Secret: apisummarizer.NewInferenceSecret("test-secret", &summarizerv1pb.InferenceSecretSpec{
			Value: "my-secret-value",
		}),
	})
	require.NoError(t, err)

	_, err = sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
		Model: apisummarizer.NewInferenceModel("test-model", &summarizerv1pb.InferenceModelSpec{
			Provider: &summarizerv1pb.InferenceModelSpec_Openai{
				Openai: &summarizerv1pb.OpenAIProvider{
					OpenaiModelId:   "gpt-4o",
					ApiKeySecretRef: "test-secret",
				},
			},
		}),
	})
	require.NoError(t, err)

	_, err = sclt.CreateInferencePolicy(ctx, &summarizerv1pb.CreateInferencePolicyRequest{
		Policy: apisummarizer.NewInferencePolicy("test-policy", &summarizerv1pb.InferencePolicySpec{
			Kinds: []string{
				string(types.SSHSessionKind), string(types.KubernetesSessionKind), string(types.DatabaseSessionKind),
			},
			Model: "test-model",
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
	content := body.Messages[1].OfUser.Content.OfString.Value
	if content == "cause an error" {
		return nil, errors.New("OpenAI error")
	}
	return &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: "The user wrote: " + content,
			},
		}},
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
		return err == nil
	}, time.Second*5, time.Millisecond*100)
	return sr.Summary
}

func TestSummarizer(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newSummarizerTestTLSServer(t, eventstest.NewMemoryUploader())

	createTestUser(t, srv, "alice")
	clt, err := srv.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()
	createSummarizerConfig(t, ctx, sclt)

	// Add a database.
	db, err := types.NewDatabaseV3(
		types.Metadata{Name: "treasure-trove"},
		types.DatabaseSpecV3{Protocol: types.DatabaseProtocolPostgreSQL, URI: "db.test:5432"},
	)
	require.NoError(t, err)
	dbServer, err := types.NewDatabaseServerV3(
		types.Metadata{Name: "my-db-server"},
		types.DatabaseServerSpecV3{
			HostID:   "1517bec3-ce77-466c-a988-6677a5743a64",
			Hostname: "dbhost",
			Database: db,
		},
	)
	require.NoError(t, err)
	_, err = clt.UpsertDatabaseServer(ctx, dbServer)
	require.NoError(t, err)

	sshSessionID := "24d8542a-8a7d-4683-a59b-18adc3a71f11"
	kubeSessionID := "8fef2bf5-3efa-4c5d-8502-9410dea3dc94"
	dbSessionID := "44608ee9-af78-4970-b523-ac4eb6121de7"
	errorSessionID := "9a0ec7f5-2d1c-4c15-bb8c-9ac45864a627"
	serverID := "9d68b09f-8c0c-49a3-b54d-f8791f0c3941"
	cases := []struct {
		name      string
		sessionID string
		events    []apievents.AuditEvent
		state     summarizerv1pb.SummaryState
		summary   string
		error     string
	}{
		{
			name:      "SSH session",
			sessionID: sshSessionID,
			events: eventstest.GenerateTestSession(eventstest.SessionParams{
				UserName:  "alice",
				SessionID: sshSessionID,
				// OpenSSH nodes have cluster name as a suffix.
				ServerID:  serverID + ".testcluster",
				PrintData: []string{"net", "stat"},
			}),
			state:   summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
			summary: "The user wrote: netstat",
		},
		{
			name:      "Kubernetes session",
			sessionID: kubeSessionID,
			events: generateTestKubeSession(eventstest.SessionParams{
				UserName:  "alice",
				SessionID: kubeSessionID,
				PrintData: []string{"ps ", "aux"},
			}),
			state:   summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
			summary: "The user wrote: ps aux",
		},
		{
			name:      "dynamic database session",
			sessionID: dbSessionID,
			events: eventstest.GenerateTestDBSession(eventstest.DBSessionParams{
				UserName:        "alice",
				SessionID:       dbSessionID,
				DatabaseService: "treasure-trove",
				Queries:         1,
			}),
			state:   summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
			summary: "The user wrote: SELECT order_id FROM order where customer_id=0",
		},
		{
			name:      "error",
			sessionID: errorSessionID,
			events: eventstest.GenerateTestSession(eventstest.SessionParams{
				UserName:  "alice",
				SessionID: errorSessionID,
				ServerID:  serverID,
				// This text will trigger the fake inference provider to fail.
				PrintData: []string{"cause an error"},
			}),
			state: summarizerv1pb.SummaryState_SUMMARY_STATE_ERROR,
			error: "OpenAI error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			startTime := srv.Clock().Now()

			// Ingest an example session.
			stream, err := srv.Auth().CreateAuditStream(ctx, session.ID(tc.sessionID))
			require.NoError(t, err)
			for _, event := range tc.events {
				err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
				require.NoError(t, err)
			}
			err = stream.Complete(ctx)
			require.NoError(t, err)

			summary := waitForSummary(t, ctx, sclt, tc.sessionID)

			sef, err := events.ToEventFields(tc.events[len(tc.events)-1])
			require.NoError(t, err)
			expectedEndEvent, err := structpb.NewStruct(sef)
			require.NoError(t, err)

			assert.Empty(t, cmp.Diff(
				&summarizerv1pb.Summary{
					SessionId:           tc.sessionID,
					State:               tc.state,
					InferenceStartedAt:  timestamppb.New(startTime),
					InferenceFinishedAt: timestamppb.New(startTime.Add(10 * time.Second)),
					Content:             tc.summary,
					ModelName:           "test-model",
					SessionEndEvent:     expectedEndEvent,
					ErrorMessage:        tc.error,
				},
				summary,
				protocmp.Transform(),
			))
		})
	}
}

func TestSummarizerNoEndEvent(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	uploader := eventstest.NewMemoryUploader()

	sshSessionID := "24d8542a-8a7d-4683-a59b-18adc3a71f11"
	serverID := "9d68b09f-8c0c-49a3-b54d-f8791f0c3941"
	evts := eventstest.GenerateTestSession(eventstest.SessionParams{
		UserName:  "alice",
		SessionID: sshSessionID,
		ServerID:  serverID,
		PrintData: []string{"net", "stat"},
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
	srv := newSummarizerTestTLSServer(t, uploader)

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
			ModelName:           "test-model",
			SessionEndEvent:     expectedEndEvent,
			ErrorMessage:        "",
		},
		summary,
		protocmp.Transform(),
	))
}

// generateTestKubeSession is a copy of [eventstest.GenerateTestSession]
// modified to return a dummy Kubernetes session data.
// TODO(bl-nero): move it where it belongs.
func generateTestKubeSession(params eventstest.SessionParams) []apievents.AuditEvent {
	params.SetDefaults()
	connectionMetadata := apievents.ConnectionMetadata{
		LocalAddr:  "127.0.0.1:3022",
		RemoteAddr: "[::1]:37718",
		Protocol:   events.EventProtocolKube,
	}
	kubernetesClusterMetadata := apievents.KubernetesClusterMetadata{
		KubernetesCluster: "my-kube-cluster",
		KubernetesUsers:   []string{"admin"},
		KubernetesGroups:  []string{"viewers"},
		KubernetesLabels: map[string]string{
			"teleport.internal/resource-id": "ed910b7b-fe3b-4959-bf2e-ac45f4648f2a",
		},
	}
	kubernetesPodMetadata := apievents.KubernetesPodMetadata{
		KubernetesPodName:        "simple-shell-pod",
		KubernetesPodNamespace:   "default",
		KubernetesContainerName:  "shell-container",
		KubernetesContainerImage: "busybox",
		KubernetesNodeName:       "docker-desktop",
	}
	sessionStart := apievents.SessionStart{
		Metadata: apievents.Metadata{
			Index:       0,
			Type:        events.SessionStartEvent,
			ID:          "36cee9e9-9a80-4c32-9163-3d9241cdac7a",
			Code:        events.SessionStartCode,
			Time:        params.Clock.Now().UTC(),
			ClusterName: params.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion: teleport.Version,
			ServerID:      params.ServerID,
			ServerLabels: map[string]string{
				"teleport.internal/resource-id": "ed910b7b-fe3b-4959-bf2e-ac45f4648f2a",
			},
			ServerHostname:  "planet",
			ServerNamespace: "default",
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: params.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User:  params.UserName,
			Login: "bob",
		},
		ConnectionMetadata:        connectionMetadata,
		TerminalSize:              "80:25",
		KubernetesClusterMetadata: kubernetesClusterMetadata,
		KubernetesPodMetadata:     kubernetesPodMetadata,
	}

	sessionEnd := apievents.SessionEnd{
		Metadata: apievents.Metadata{
			Index: 20,
			Type:  events.SessionEndEvent,
			ID:    "da455e0f-c27d-459f-a218-4e83b3db9426",
			Code:  events.SessionEndCode,
			Time:  params.Clock.Now().UTC().Add(time.Hour + time.Second + 7*time.Millisecond),
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        params.ServerID,
			ServerNamespace: "default",
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: params.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User: params.UserName,
		},
		ConnectionMetadata:        connectionMetadata,
		EnhancedRecording:         true,
		Interactive:               true,
		Participants:              []string{params.UserName},
		StartTime:                 params.Clock.Now().UTC(),
		EndTime:                   params.Clock.Now().UTC().Add(3*time.Hour + time.Second + 7*time.Millisecond),
		KubernetesClusterMetadata: kubernetesClusterMetadata,
		KubernetesPodMetadata:     kubernetesPodMetadata,
	}

	genEvents := []apievents.AuditEvent{&sessionStart}
	for i, data := range params.PrintData {
		event := &apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Index: int64(i) + 1,
				Type:  events.SessionPrintEvent,
				Time:  params.Clock.Now().UTC().Add(time.Minute + time.Duration(i)*time.Millisecond),
			},
			ChunkIndex:        int64(i),
			DelayMilliseconds: int64(i),
			Offset:            int64(i),
			Data:              []byte(data),
		}
		event.Bytes = int64(len(event.Data))
		event.Time = event.Time.Add(time.Duration(i) * time.Millisecond)

		genEvents = append(genEvents, event)
	}

	sessionEnd.Metadata.Index = int64(len(genEvents))
	genEvents = append(genEvents, &sessionEnd)

	return genEvents
}
