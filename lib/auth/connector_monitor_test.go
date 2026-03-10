package auth_test

import (
	"context"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCheckSAMLCertExpiry(t *testing.T) {
	srv := newTestTLSServer(t)
	t.Cleanup(func() {
		require.NoError(t, srv.Close())
	})

	assertOneAlert := func(t require.TestingT, alerts any, _ ...any) {
		require.Len(t, alerts, 1)
	}

	tests := []struct {
		name         string
		ttls         []time.Duration
		assertAlerts require.ValueAssertionFunc
	}{
		{
			name:         "no connectors",
			ttls:         []time.Duration{},
			assertAlerts: require.Empty,
		},
		{
			name: "no certs expiring or expired",
			ttls: []time.Duration{
				auth.SAMLCertExpiryTimeframe + 1*time.Hour,
				auth.SAMLCertExpiryTimeframe + 7*24*time.Hour,
				auth.SAMLCertExpiryTimeframe + 365*24*time.Hour,
			},
			assertAlerts: require.Empty,
		},
		{
			name: "single cert expiring",
			ttls: []time.Duration{
				auth.SAMLCertExpiryTimeframe + 1*time.Hour,
				auth.SAMLCertExpiryTimeframe - 7*24*time.Hour,
				auth.SAMLCertExpiryTimeframe + 365*24*time.Hour,
			},
			assertAlerts: assertOneAlert,
		},
		{
			name: "multiple certs expiring",
			ttls: []time.Duration{
				auth.SAMLCertExpiryTimeframe - 1*time.Hour,
				auth.SAMLCertExpiryTimeframe - 7*24*time.Hour,
				auth.SAMLCertExpiryTimeframe + 365*24*time.Hour,
			},
			assertAlerts: assertOneAlert,
		},
		{
			name: "cert already expired",
			ttls: []time.Duration{
				auth.SAMLCertExpiryTimeframe + 1*time.Hour,
				-7 * 24 * time.Hour,
				auth.SAMLCertExpiryTimeframe + 365*24*time.Hour,
			},
			assertAlerts: assertOneAlert,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			if len(tt.ttls) > 0 {
				connector, err := types.NewSAMLConnector(fmt.Sprintf("test-connector-%d", i), types.SAMLConnectorSpecV2{
					AssertionConsumerService: "https://localhost:65535/acs", // Not called.
					EntityDescriptor:         createTestEntityDescriptor(t, tt.ttls),
					SSO:                      "https://localhost.com/sso", // Not called.
					AttributesToRoles: []types.AttributeMapping{
						{Name: "group", Value: "devs", Roles: []string{"$1"}},
					},
				})
				require.NoError(t, err)

				createdConnector, err := srv.Auth().Services.CreateSAMLConnector(ctx, connector)
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, srv.Auth().Services.DeleteSAMLConnector(context.Background(), createdConnector.GetName()))
					if err := srv.Auth().DeleteClusterAlert(
						context.Background(), auth.SAMLCertExpiryAlertName,
					); err != nil && !trace.IsNotFound(err) {
						require.NoError(t, err)
					}
				})
			}

			err := srv.Auth().CheckSAMLCertExpiry(ctx)
			require.NoError(t, err)

			alerts, err := srv.Auth().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
				AlertID: auth.SAMLCertExpiryAlertName,
			})
			require.NoError(t, err)
			tt.assertAlerts(t, alerts)
		})
	}

	t.Run("end-to-end cert expiry alert flow", func(t *testing.T) {
		ctx := t.Context()

		initialTTL := auth.SAMLCertExpiryTimeframe - 7*24*time.Hour
		rotatedTTL := auth.SAMLCertExpiryTimeframe + 7*24*time.Hour

		connector, err := types.NewSAMLConnector("test-connector-rotation", types.SAMLConnectorSpecV2{
			AssertionConsumerService: "https://localhost:65535/acs", // Not called.
			EntityDescriptor:         createTestEntityDescriptor(t, []time.Duration{initialTTL}),
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

		alerts, err := srv.Auth().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
			AlertID: auth.SAMLCertExpiryAlertName,
		})
		require.NoError(t, err)
		require.Len(t, alerts, 1)
		require.Equal(t, types.AlertSeverity_MEDIUM, alerts[0].Spec.Severity)
		require.Equal(t, fmt.Sprintf("%s:%s", types.KindSAML, types.VerbRead), alerts[0].GetAllLabels()[types.AlertVerbPermit])
		require.Equal(t, "yes", alerts[0].GetAllLabels()[types.AlertOnLogin])
		require.Contains(t, alerts[0].Spec.Message, initialConnector.GetName())

		initialConnector.SetEntityDescriptor(createTestEntityDescriptor(t, []time.Duration{rotatedTTL}))

		rotatedConnector, err := srv.Auth().UpdateSAMLConnector(ctx, initialConnector)
		require.NoError(t, err)

		alerts, err = srv.Auth().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
			AlertID: auth.SAMLCertExpiryAlertName,
		})
		require.NoError(t, err)
		require.Empty(t, alerts)

		rotatedConnector.SetEntityDescriptor(createTestEntityDescriptor(t, []time.Duration{initialTTL}))

		updatedConnector, err := srv.Auth().UpdateSAMLConnector(ctx, rotatedConnector)
		require.NoError(t, err)

		alerts, err = srv.Auth().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
			AlertID: auth.SAMLCertExpiryAlertName,
		})
		require.NoError(t, err)
		require.Len(t, alerts, 1)

		require.NoError(t, srv.Auth().DeleteSAMLConnector(ctx, updatedConnector.GetName()))

		alerts, err = srv.Auth().GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
			AlertID: auth.SAMLCertExpiryAlertName,
		})
		require.NoError(t, err)
		require.Empty(t, alerts)
	})
}

func createTestEntityDescriptor(t *testing.T, ttls []time.Duration) string {
	t.Helper()

	var certs []string

	for _, ttl := range ttls {
		_, certPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{}, nil, ttl)
		require.NoError(t, err)

		block, _ := pem.Decode(certPEM)
		certs = append(certs, fmt.Sprintf(
			`<md:KeyDescriptor use="signing"><ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:X509Data><ds:X509Certificate>%s</ds:X509Certificate></ds:X509Data></ds:KeyInfo></md:KeyDescriptor>`,
			base64.StdEncoding.EncodeToString(block.Bytes),
		))
	}

	return fmt.Sprintf(
		`<?xml version="1.0"?><md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="test"><md:IDPSSODescriptor>%s</md:IDPSSODescriptor></md:EntityDescriptor>`,
		strings.Join(certs, ""),
	)
}
