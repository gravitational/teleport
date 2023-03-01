package main

import (
	"context"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"testing"

	"github.com/bufbuild/connect-go"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
)

type mockReportingServiceClient struct {
	eventRequest *v1alpha.SubmitTbotEventRequest
}

func (mrsc *mockReportingServiceClient) SubmitTbotEvent(
	ctx context.Context,
	req *connect.Request[v1alpha.SubmitTbotEventRequest],
) (*connect.Response[v1alpha.SubmitTbotEventResponse], error) {
	mrsc.eventRequest = req.Msg
	return connect.NewResponse(&v1alpha.SubmitTbotEventResponse{}), nil
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
		require.Equal(t, &v1alpha.SubmitTbotEventRequest_Start{
			Start: &v1alpha.TbotStartEvent{
				RunMode:  v1alpha.TbotStartEvent_RUN_MODE_ONE_SHOT,
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
