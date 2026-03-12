package auth_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services/samltest"
)

func TestSAMLCertExpiryMonitor(t *testing.T) {
	srv := newTestTLSServer(t)
	t.Cleanup(func() {
		require.NoError(t, srv.Close())
	})

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	initialTTL := auth.SAMLCertExpiryTimeframe - 7*24*time.Hour
	rotatedTTL := auth.SAMLCertExpiryTimeframe + 7*24*time.Hour

	connector, err := types.NewSAMLConnector("test-connector-rotation", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "https://localhost:65535/acs", // Not called.
		EntityDescriptor:         samltest.CreateTestEntityDescriptor(t, []time.Duration{initialTTL}),
		SSO:                      "https://localhost.com/sso", // Not called.
		AttributesToRoles: []types.AttributeMapping{
			{Name: "group", Value: "devs", Roles: []string{"$1"}},
		},
	})
	require.NoError(t, err)

	initialConnector, err := srv.Auth().CreateSAMLConnector(ctx, connector)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := srv.Auth().DeleteSAMLConnector(
			context.Background(), connector.GetName(),
		); err != nil && !trace.IsNotFound(err) {
			require.NoError(t, err)
		}
	})

	monitor, err := auth.NewSAMLCertExpiryMonitor(auth.SAMLCertExpiryMonitorConfig{
		Connectors: srv.Auth().Services,
		Alerts:     srv.Auth().Services,
		Events:     srv.Auth().Services,
		Clock:      srv.Clock(),
		Logger:     slog.New(slog.DiscardHandler),
	})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		monitor.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		alerts, err := srv.Auth().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
			AlertID: auth.SAMLCertExpiryAlertID,
		})
		require.NoError(t, err)
		require.Len(t, alerts, 1)
		require.Equal(t, types.AlertSeverity_MEDIUM, alerts[0].Spec.Severity)
		require.Equal(t, fmt.Sprintf("%s:%s", types.KindSAML, types.VerbRead), alerts[0].GetAllLabels()[types.AlertVerbPermit])
		require.Equal(t, "yes", alerts[0].GetAllLabels()[types.AlertOnLogin])
		require.Contains(t, alerts[0].Spec.Message, initialConnector.GetName())
	}, time.Second, 10*time.Millisecond)

	initialConnector.SetEntityDescriptor(samltest.CreateTestEntityDescriptor(t, []time.Duration{rotatedTTL}))

	rotatedConnector, err := srv.Auth().UpdateSAMLConnector(ctx, initialConnector)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		alerts, err := srv.Auth().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
			AlertID: auth.SAMLCertExpiryAlertID,
		})
		require.NoError(t, err)
		require.Empty(t, alerts)
	}, time.Second, 10*time.Millisecond)

	rotatedConnector.SetEntityDescriptor(samltest.CreateTestEntityDescriptor(t, []time.Duration{initialTTL}))

	updatedConnector, err := srv.Auth().UpdateSAMLConnector(ctx, rotatedConnector)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		alerts, err := srv.Auth().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
			AlertID: auth.SAMLCertExpiryAlertID,
		})
		require.NoError(t, err)
		require.Len(t, alerts, 1)
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, srv.Auth().DeleteSAMLConnector(ctx, updatedConnector.GetName()))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		alerts, err := srv.Auth().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
			AlertID: auth.SAMLCertExpiryAlertID,
		})
		require.NoError(t, err)
		require.Empty(t, alerts)
	}, time.Second, 10*time.Millisecond)
}
