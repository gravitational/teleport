/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
)

type mockReportingServiceClient struct {
	eventRequest *prehogv1a.SubmitTbotEventRequest
}

func (mrsc *mockReportingServiceClient) SubmitTbotEvent(
	ctx context.Context,
	req *connect.Request[prehogv1a.SubmitTbotEventRequest],
) (*connect.Response[prehogv1a.SubmitTbotEventResponse], error) {
	mrsc.eventRequest = req.Msg
	return connect.NewResponse(&prehogv1a.SubmitTbotEventResponse{}), nil
}

func mockEnvGetter(data map[string]string) envGetter {
	return func(key string) string {
		return data[key]
	}
}

func TestSendTelemetry(t *testing.T) {
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()

	t.Run("sends telemetry when enabled", func(t *testing.T) {
		mockClient := &mockReportingServiceClient{}
		env := map[string]string{
			helperEnv:                    "test",
			helperVersionEnv:             "13.37.0",
			anonymousTelemetryEnabledEnv: "1",
		}
		cfg := &config.BotConfig{
			Oneshot: true,
			Onboarding: config.OnboardingConfig{
				JoinMethod: types.JoinMethodGitHub,
			},
			Services: config.ServiceConfigs{
				&config.IdentityOutput{
					Destination: &config.DestinationDirectory{},
				},
				&config.KubernetesOutput{
					Destination: &config.DestinationDirectory{},
				},
				&config.ApplicationOutput{
					Destination: &config.DestinationDirectory{},
				},
				&config.DatabaseOutput{
					Destination: &config.DestinationDirectory{},
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
		require.NotEmpty(t, mockClient.eventRequest.DistinctId)
		require.Equal(t, &prehogv1a.SubmitTbotEventRequest_Start{
			Start: &prehogv1a.TbotStartEvent{
				RunMode:  prehogv1a.TbotStartEvent_RUN_MODE_ONE_SHOT,
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
