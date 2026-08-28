/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
package auth_test

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services/samltest"
	"github.com/gravitational/teleport/lib/utils"
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

	// Upsert an unrelated cluster alert that we expect not to be affected by SAML expiry alert reconcilliation.
	miscAlert, err := types.NewClusterAlert("unrelated-alert", "Some unrelated alert that should not be affected.")
	require.NoError(t, err)
	require.NoError(t, srv.Auth().Services.UpsertClusterAlert(ctx, miscAlert))

	_, certPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{}, nil, 0)
	require.NoError(t, err)

	// Create a connector with expiring cert in entity descriptor and on cert field.
	connector, err := types.NewSAMLConnector("test-connector-rotation", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "https://localhost:65535/acs", // Not called.
		// Expired cert in the entity descriptor.
		EntityDescriptor: samltest.CreateTestEntityDescriptor(t, []time.Duration{initialTTL}),
		SSO:              "https://localhost.com/sso", // Not called.
		AttributesToRoles: []types.AttributeMapping{
			{Name: "group", Value: "devs", Roles: []string{"$1"}},
		},
		// Expired cert in the cert field.
		Cert: string(certPEM),
	})
	require.NoError(t, err)

	initialConnector, err := srv.Auth().CreateSAMLConnector(ctx, connector)
	require.NoError(t, err)

	// Create the SAML cert expiry monitor.
	monitor, err := auth.NewSAMLCertExpiryMonitor(auth.SAMLCertExpiryMonitorConfig{
		Connectors: srv.Auth().Services,
		Alerts:     srv.Auth().Services,
		Events:     srv.Auth().Services,
		Clock:      srv.Clock(),
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
		Backend:    srv.AuthServer.Backend,
	})
	require.NoError(t, err)

	// Start the SAML cert expiry monitor.
	done := make(chan struct{})
	go func() {
		defer close(done)
		monitor.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	// Initial state should have 3 alerts:
	//   - one for the expired entity descriptor cert
	//   - one for the expired cert field cert.
	//   - one pre-existing unrelated alert.
	requireClusterAlerts(t, ctx, srv.Auth(), 3, 2, connector.GetName())

	// Set the entity descriptor cert to be outside the expiry period and update the connector.
	initialConnector.SetEntityDescriptor(samltest.CreateTestEntityDescriptor(t, []time.Duration{rotatedTTL}))

	rotatedCertConnector, err := srv.Auth().UpdateSAMLConnector(ctx, initialConnector)
	require.NoError(t, err)

	// Should have 2 alerts:
	//   - one for the expired cert field cert.
	//   - one pre-existing unrelated alert.
	requireClusterAlerts(t, ctx, srv.Auth(), 2, 1, rotatedCertConnector.GetName())

	// Remove the cert field on the connector.
	rotatedCertConnector.SetCert("")
	removedCertConnector, err := srv.Auth().UpdateSAMLConnector(ctx, rotatedCertConnector)
	require.NoError(t, err)

	// Should have 1 alert:
	//   - one pre-existing unrelated alert.
	requireClusterAlerts(t, ctx, srv.Auth(), 1, 0, "")

	// Set the entity descriptor cert to expire.
	removedCertConnector.SetEntityDescriptor(samltest.CreateTestEntityDescriptor(t, []time.Duration{initialTTL}))

	updatedConnector, err := srv.Auth().UpdateSAMLConnector(ctx, removedCertConnector)
	require.NoError(t, err)

	// Should have 2 alerts:
	//   - one for the newly expired entity descriptor cert
	//   - one pre-existing unrelated alert.
	requireClusterAlerts(t, ctx, srv.Auth(), 2, 1, updatedConnector.GetName())

	// Delete the SAML connector.
	require.NoError(t, srv.Auth().DeleteSAMLConnector(ctx, updatedConnector.GetName()))

	// Should have 1 alert:
	//   - one pre-existing unrelated alert.
	requireClusterAlerts(t, ctx, srv.Auth(), 1, 0, "")
}

func requireClusterAlerts(t *testing.T, ctx context.Context, srv *auth.Server, wantTotal, wantSAML int, connectorName string) {
	t.Helper()

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		alerts, err := srv.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
		require.NoError(t, err)
		require.Len(t, alerts, wantTotal)

		samlCount := 0
		for _, alert := range alerts {
			if !strings.HasPrefix(alert.GetName(), auth.SAMLCertExpiryAlertID) {
				continue
			}
			require.Equal(t, types.AlertSeverity_MEDIUM, alert.Spec.Severity)
			require.Equal(t, fmt.Sprintf("%s:%s", types.KindSAML, types.VerbRead), alert.GetAllLabels()[types.AlertVerbPermit])
			require.Equal(t, "yes", alert.GetAllLabels()[types.AlertOnLogin])
			require.Contains(t, alert.Spec.Message, connectorName)
			samlCount++
		}

		require.Equal(t, wantSAML, samlCount)
	}, 5*time.Second, 10*time.Millisecond)
}
