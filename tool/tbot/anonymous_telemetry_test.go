package main

import (
	"context"
	"testing"

	"github.com/bufbuild/connect-go"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
)

type mockReportingServiceClient struct {
	eventRequest *prehogv1.SubmitTbotEventRequest
}

func (mrsc *mockReportingServiceClient) SubmitTbotEvent(
	ctx context.Context,
	req *connect.Request[prehogv1.SubmitTbotEventRequest],
) (*connect.Response[prehogv1.SubmitTbotEventResponse], error) {
	mrsc.eventRequest = req.Msg
	return connect.NewResponse(&prehogv1.SubmitTbotEventResponse{}), nil
}

func mockEnvGetter(data map[string]string) envGetter {
	return func(key string) string {
		return data[key]
	}
}

func TestSendTelemetry(t *testing.T) {
	ctx := context.Background()
	log := utils.NewLoggerForTests()

	t.Run("sends telemetry when enabled", func(t *testing.T) {
		mockClient := &mockReportingServiceClient{}
		env := map[string]string{
			helperEnv:                    "test",
			helperVersionEnv:             "13.37.0",
			anonymousTelemetryEnabledEnv: "1",
		}
		cfg := &config.BotConfig{
			Oneshot: true,
			Onboarding: &config.OnboardingConfig{
				JoinMethod: types.JoinMethodGitHub,
			},
			Destinations: []*config.DestinationConfig{
				{
					DestinationMixin: config.DestinationMixin{
						Directory: &config.DestinationDirectory{},
					},
				},
				{
					DestinationMixin: config.DestinationMixin{
						Directory: &config.DestinationDirectory{},
					},
					KubernetesCluster: &config.KubernetesCluster{
						ClusterName: "foo",
					},
				},
				{
					DestinationMixin: config.DestinationMixin{
						Directory: &config.DestinationDirectory{},
					},
					App: &config.App{
						App: "bar",
					},
				},
				{
					DestinationMixin: config.DestinationMixin{
						Directory: &config.DestinationDirectory{},
					},
					Database: &config.Database{
						Database: "biz",
					},
				},
			},
		}
		err := sendTelemetry(
			ctx,
			mockClient,
			mockEnvGetter(env),
			log,
			cfg,
		)
		require.NoError(t, err)
		require.NotNil(t, mockClient.eventRequest)
		require.NotZero(t, mockClient.eventRequest.Timestamp)
		require.NotZero(t, mockClient.eventRequest.DistinctId)
		require.Equal(t, &prehogv1.SubmitTbotEventRequest_Start{
			Start: &prehogv1.TbotStartEvent{
				RunMode:  prehogv1.TbotStartEvent_RUN_MODE_ONE_SHOT,
				JoinType: string(types.JoinMethodGitHub),
				Version:  teleport.Version,

				Helper:        env[helperEnv],
				HelperVersion: env[helperVersionEnv],

				DestinationsApplication: 1,
				DestinationsKubernetes:  1,
				DestinationsDatabase:    1,
				DestinationsOther:       1,
			},
		}, mockClient.eventRequest.Event)
	})
	t.Run("does not send telemetry when not explicitly enabled", func(t *testing.T) {
		mockClient := &mockReportingServiceClient{}
		env := map[string]string{}
		cfg := &config.BotConfig{}
		err := sendTelemetry(
			ctx,
			mockClient,
			mockEnvGetter(env),
			log,
			cfg,
		)
		require.NoError(t, err)
		require.Nil(t, mockClient.eventRequest)
	})
}
