package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// samlSigningKeyCheckCycle is the frequency for the SAML signing key expiry check to run.
	samlSigningKeyCheckCycle = 24 * time.Hour
	// samlSigningKeyExpiryTimeframe is the duration before expiry at which a SAML signing key
	// is considered to be 'expiring'.
	samlSigningKeyExpiryTimeframe = 90 * 24 * time.Hour
	// samlSigningKeyExpiryNotificationName is the ID used for the global notification.
	samlSigningKeyExpiryNotificationName = "saml-signing-key-expiry-warning"
	// samlSigningKeyExpiryNotificationExpires is the expiration time for the notification.
	// It's set to 2x the check cycle so any stale notifications will clear automatically
	// without affecting valid notifications.
	samlSigningKeyExpiryNotificationExpires = samlSigningKeyCheckCycle * 2
)

// MonitorSAMLSigningKeyExpiry periodically runs the SAML signing key expiry check.
func (a *Server) MonitorSAMLSigningKeyExpiry(ctx context.Context) error {
	checkInterval := interval.New(interval.Config{
		FirstDuration: 10 * time.Second,
		Duration:      samlSigningKeyCheckCycle,
		Clock:         a.GetClock(),
	})
	defer checkInterval.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-checkInterval.Next():
			if err := a.checkSAMLSigningKeyExpiry(ctx); err != nil {
				slog.ErrorContext(ctx, "Failed to check SAML signing key expiry", "error", err)
			}
		}
	}
}

// checkSAMLSigningKeyExpiry checks all SAML connectors for any that are expiring or expired
// and creates or updates a global notification. If none are expiring, then any existing
// notification is deleted.
func (a *Server) checkSAMLSigningKeyExpiry(ctx context.Context) error {
	connectors, err := a.GetSAMLConnectors(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}

	var expiringConnectors []string
	for _, connector := range connectors {
		if expiring, err := services.CheckSAMLSigningKeyExpiry(connector, samlSigningKeyExpiryTimeframe); err != nil {
			a.logger.WarnContext(ctx, "Failed to check SAML connector signing key expiry", "connector", connector.GetName(), "error", err)
		} else if expiring {
			expiringConnectors = append(expiringConnectors, connector.GetName())
		}
	}

	if len(expiringConnectors) > 0 {
		title := "SAML signing keys expiring or expired"
		message := buildSAMLSigningKeyExpiryNotificationMessage(expiringConnectors)

		if err := upsertSAMLSigningKeyExpiryNotification(ctx, a.Services, title, message); err != nil {
			slog.ErrorContext(ctx, "Failed to upsert SAML signing key expiry notification", "error", err)
		}

		return nil
	}

	if err := a.Services.DeleteGlobalNotification(ctx, samlSigningKeyExpiryNotificationName); err != nil && !trace.IsNotFound(err) {
		slog.ErrorContext(ctx, "Failed to delete SAML signing key expiry notification", "error", err)
	}

	return nil
}

func upsertSAMLSigningKeyExpiryNotification(ctx context.Context, notification services.Notifications, title, text string) error {
	now := time.Now()

	_, err := notification.UpsertGlobalNotification(ctx, &notificationsv1.GlobalNotification{
		Kind:     types.KindGlobalNotification,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: samlSigningKeyExpiryNotificationName},
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
				ByPermissions: &notificationsv1.ByPermissions{
					RoleConditions: []*types.RoleConditions{
						{Rules: []types.Rule{{Resources: []string{types.KindSAML}, Verbs: services.RW()}}},
					},
				},
			},
			Notification: &notificationsv1.Notification{
				SubKind: types.NotificationDefaultWarningSubKind,
				Spec: &notificationsv1.NotificationSpec{
					Created: timestamppb.New(now),
				},
				Metadata: &headerv1.Metadata{
					Name:    samlSigningKeyExpiryNotificationName,
					Expires: timestamppb.New(now.Add(samlSigningKeyExpiryNotificationExpires)),
					Labels: map[string]string{
						types.NotificationTitleLabel:       title,
						types.NotificationTextContentLabel: text,
					},
				},
			},
		},
	})

	return trace.Wrap(err)
}

func buildSAMLSigningKeyExpiryNotificationMessage(connectorNames []string) string {
	return fmt.Sprintf(
		"The following connectors have one or more signing keys that have expired or will expire in the next %d days.\n%s",
		int(samlSigningKeyExpiryTimeframe.Hours()/24),
		strings.Join(connectorNames, "\n"),
	)
}
