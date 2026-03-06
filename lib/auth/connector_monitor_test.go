package auth_test

import (
	"context"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCheckSAMLSigningKeyExpiry(t *testing.T) {
	srv := newTestTLSServer(t)
	t.Cleanup(func() {
		require.NoError(t, srv.Close())
	})

	tests := []struct {
		name               string
		ttls               []time.Duration
		expectNotification bool
	}{
		{
			name:               "no connectors",
			ttls:               []time.Duration{},
			expectNotification: false,
		},
		{
			name: "no certs expiring or expired",
			ttls: []time.Duration{
				auth.SAMLSigningKeyExpiryTimeframe + 1*time.Hour,
				auth.SAMLSigningKeyExpiryTimeframe + 7*24*time.Hour,
				auth.SAMLSigningKeyExpiryTimeframe + 365*24*time.Hour,
			},
			expectNotification: false,
		},
		{
			name: "single cert expiring",
			ttls: []time.Duration{
				auth.SAMLSigningKeyExpiryTimeframe + 1*time.Hour,
				auth.SAMLSigningKeyExpiryTimeframe - 7*24*time.Hour,
				auth.SAMLSigningKeyExpiryTimeframe + 365*24*time.Hour,
			},
			expectNotification: true,
		},
		{
			name: "multiple certs expiring",
			ttls: []time.Duration{
				auth.SAMLSigningKeyExpiryTimeframe - 1*time.Hour,
				auth.SAMLSigningKeyExpiryTimeframe - 7*24*time.Hour,
				auth.SAMLSigningKeyExpiryTimeframe + 365*24*time.Hour,
			},
			expectNotification: true,
		},
		{
			name: "cert already expired",
			ttls: []time.Duration{
				auth.SAMLSigningKeyExpiryTimeframe + 1*time.Hour,
				-7 * 24 * time.Hour,
				auth.SAMLSigningKeyExpiryTimeframe + 365*24*time.Hour,
			},
			expectNotification: true,
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
					if err := srv.Auth().DeleteGlobalNotification(
						context.Background(), auth.SAMLSigningKeyExpiryNotificationName,
					); err != nil && !trace.IsNotFound(err) {
						require.NoError(t, err)
					}
				})
			}

			err := srv.Auth().CheckSAMLSigningKeyExpiry(ctx)
			require.NoError(t, err)

			notifications, _, err := srv.Auth().Services.ListGlobalNotifications(ctx, 10, "")
			require.NoError(t, err)

			require.Equal(t, tt.expectNotification, slices.ContainsFunc(notifications, func(n *notificationsv1.GlobalNotification) bool {
				return n.GetMetadata().GetName() == auth.SAMLSigningKeyExpiryNotificationName
			}))
		})
	}

	t.Run("end-to-end cert notification flow", func(t *testing.T) {
		ctx := t.Context()

		initialTTL := auth.SAMLSigningKeyExpiryTimeframe - 7*24*time.Hour
		rotatedTTL := auth.SAMLSigningKeyExpiryTimeframe + 7*24*time.Hour

		connector, err := types.NewSAMLConnector("test-connector-rotation", types.SAMLConnectorSpecV2{
			AssertionConsumerService: "https://localhost:65535/acs", // Not called.
			EntityDescriptor:         createTestEntityDescriptor(t, []time.Duration{initialTTL}),
			SSO:                      "https://localhost.com/sso", // Not called.
			AttributesToRoles: []types.AttributeMapping{
				{Name: "group", Value: "devs", Roles: []string{"$1"}},
			},
		})
		require.NoError(t, err)

		createdConnector, err := srv.Auth().Services.CreateSAMLConnector(ctx, connector)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, srv.Auth().Services.DeleteSAMLConnector(context.Background(), connector.GetName()))
			srv.Auth().DeleteGlobalNotification(context.Background(), auth.SAMLSigningKeyExpiryNotificationName)
		})

		err = srv.Auth().CheckSAMLSigningKeyExpiry(ctx)
		require.NoError(t, err)

		notifications, _, err := srv.Auth().Services.ListGlobalNotifications(ctx, 10, "")
		require.NoError(t, err)

		notificationIndex := slices.IndexFunc(notifications, func(n *notificationsv1.GlobalNotification) bool {
			return n.GetMetadata().GetName() == auth.SAMLSigningKeyExpiryNotificationName
		})
		require.NotEqual(t, -1, notificationIndex)
		require.Equal(t, types.NotificationDefaultWarningSubKind, notifications[notificationIndex].GetSpec().GetNotification().GetSubKind())
		require.Equal(t, []*types.RoleConditions{
			{Rules: []types.Rule{{Resources: []string{types.KindSAML}, Verbs: services.RW()}}},
		}, notifications[notificationIndex].GetSpec().GetByPermissions().GetRoleConditions())

		createdConnector.SetEntityDescriptor(createTestEntityDescriptor(t, []time.Duration{rotatedTTL}))

		_, err = srv.Auth().Services.UpdateSAMLConnector(ctx, createdConnector)
		require.NoError(t, err)
		err = srv.Auth().CheckSAMLSigningKeyExpiry(ctx)
		require.NoError(t, err)

		notifications, _, err = srv.Auth().Services.ListGlobalNotifications(ctx, 10, "")
		require.NoError(t, err)

		require.False(t, slices.ContainsFunc(notifications, func(n *notificationsv1.GlobalNotification) bool {
			return n.GetMetadata().GetName() == auth.SAMLSigningKeyExpiryNotificationName
		}))
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
