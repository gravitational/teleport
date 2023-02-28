package main

import (
	"context"
	"testing"

	"github.com/bufbuild/connect-go"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
)

type mockReportingServiceClient struct {
	eventRequest v1alpha.SubmitTbotEventRequest
}

func (mrsc *mockReportingServiceClient) SubmitTbotEvent(
	context.Context,
	*connect.Request[v1alpha.SubmitTbotEventRequest],
) (*connect.Response[v1alpha.SubmitTbotEventResponse], error) {
	return connect.NewResponse(&v1alpha.SubmitTbotEventResponse{}), nil
}

func mockEnvGetter(data map[string]string) envGetter {
	return func(key string) string {
		return data[key]
	}
}

func TestSendTelemetry(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockReportingServiceClient{}
	err := sendTelemetry(
		ctx,
		mockClient,
		mockEnvGetter(map[string]string{}),
		utils.NewLogger(),
		&config.BotConfig{},
	)
	require.NoError(t, err)
}
