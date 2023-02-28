package main

import (
	"context"
	"github.com/bufbuild/connect-go"
	"github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
	"testing"
)

type mockReportingServiceClient struct {
}

func (mrsc *mockReportingServiceClient) SubmitTbotEvent(
	context.Context,
	*connect.Request[v1alpha.SubmitTbotEventRequest],
) (*connect.Response[v1alpha.SubmitTbotEventResponse], error) {
	return nil, nil
}

func mockEnvGetter() envGetter {
	return func(key string) string {
		return ""
	}
}

func TestSendTelemetry(t *testing.T) {
	ctx := context.Background()
	mock := &mockReportingServiceClient{}
	err := sendTelemetry(
		ctx,
		mock,
		mockEnvGetter(),
		utils.NewLogger(),
		&config.BotConfig{},
	)
	require.NoError(t, err)
}
