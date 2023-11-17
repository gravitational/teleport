package licensefile

import (
	"context"
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	liblicense "github.com/gravitational/license"
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

func TestCheckLicense(t *testing.T) {
	tests := map[string]struct {
		license      *LicenseFile
		wantSeverity types.AlertSeverity
		wantMessage  string
	}{
		"Disabled": {
			license: &LicenseFile{
				KeyPair: &liblicense.License{
					Cert: &x509.Certificate{
						NotAfter: time.Now().Add(-constants.LicenseGraceInterval),
					},
				},
			},
			wantSeverity: types.AlertSeverity_HIGH,
			wantMessage:  constants.LicenseDisabledMessageFormat,
		},
		"Expired": {
			license: &LicenseFile{
				KeyPair: &liblicense.License{
					Cert: &x509.Certificate{
						NotAfter: time.Now().Add(-time.Hour),
					},
				},
			},
			wantSeverity: types.AlertSeverity_HIGH,
			wantMessage: fmt.Sprintf(
				constants.LicenseExpiredMessageFormat,
				durationMessage(constants.LicenseGraceInterval-1),
			),
		},
		"Almost expired": {
			license: &LicenseFile{
				KeyPair: &liblicense.License{
					Cert: &x509.Certificate{
						NotAfter: time.Now().Add(constants.LicenseWarningInterval / 2),
					},
				},
			},
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
