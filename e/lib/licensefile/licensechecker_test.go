package licensefile

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/constants"
)

type mockStatusInternal struct {
	alerts []types.ClusterAlert
}

func (msi *mockStatusInternal) GetClusterAlerts(ctx context.Context, query types.GetClusterAlertsRequest) ([]types.ClusterAlert, error) {
	return msi.alerts, nil
}

func (msi *mockStatusInternal) UpsertClusterAlert(ctx context.Context, alert types.ClusterAlert) error {
	msi.alerts = append(msi.alerts, alert)
	return nil
}

func (msi *mockStatusInternal) DeleteClusterAlert(ctx context.Context, alertID string) error {
	return nil
}

func (msi *mockStatusInternal) CreateAlertAck(ctx context.Context, ack types.AlertAcknowledgement) error {
	return nil
}

func (msi *mockStatusInternal) GetAlertAcks(ctx context.Context) ([]types.AlertAcknowledgement, error) {
	return nil, nil
}

func (msi *mockStatusInternal) ClearAlertAcks(ctx context.Context, req proto.ClearAlertAcksRequest) error {
	return nil
}

type mockLicenseStatusGetter struct {
	disabled   bool
	expired    bool
	expiresIn  time.Duration
	disabledIn time.Duration
}

func (m *mockLicenseStatusGetter) IsDisabled() bool          { return m.disabled }
func (m *mockLicenseStatusGetter) IsExpired() bool           { return m.expired }
func (m *mockLicenseStatusGetter) ExpiresIn() time.Duration  { return m.expiresIn }
func (m *mockLicenseStatusGetter) DisabledIn() time.Duration { return m.disabledIn }

func TestCheckLicense(t *testing.T) {
	tests := map[string]struct {
		license      LicenseStatusGetter
		wantSeverity types.AlertSeverity
		wantMessage  string
	}{
		"Disabled": {
			license:      &mockLicenseStatusGetter{disabled: true},
			wantSeverity: types.AlertSeverity_HIGH,
			wantMessage:  constants.LicenseDisabledMessageFormat,
		},
		"Expired": {
			license:      &mockLicenseStatusGetter{expired: true, disabledIn: constants.LicenseGraceInterval - 1},
			wantSeverity: types.AlertSeverity_HIGH,
			wantMessage: fmt.Sprintf(
				constants.LicenseExpiredMessageFormat,
				durationMessage(constants.LicenseGraceInterval-1),
			),
		},
		"Almost expired": {
			license:      &mockLicenseStatusGetter{expiresIn: constants.LicenseWarningInterval/2 - 1},
			wantSeverity: types.AlertSeverity_MEDIUM,
			wantMessage: fmt.Sprintf(
				constants.LicenseWarningMessageFormat,
				durationMessage(constants.LicenseWarningInterval/2-1),
			),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			msi := &mockStatusInternal{}
			err := checkLicense(context.Background(), msi, test.license)
			require.NoError(t, err)
			query := types.GetClusterAlertsRequest{
				Labels: map[string]string{
					types.AlertOnLogin:   "yes",
					types.AlertPermitAll: "yes",
				},
			}
			alerts, err := msi.GetClusterAlerts(context.Background(), query)
			require.NoError(t, err)
			require.Len(t, alerts, 1)
			require.Equal(t, test.wantSeverity, alerts[0].Spec.Severity)
			require.Equal(t, test.wantMessage, alerts[0].Spec.Message)
		})
	}
}
