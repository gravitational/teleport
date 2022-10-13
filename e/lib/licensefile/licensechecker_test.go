// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package licensefile

import (
	"context"
	"crypto/x509"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/stretchr/testify/require"

	liblicense "github.com/gravitational/license"
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

func TestCheckLicense(t *testing.T) {
	tests := map[string]struct {
		license      *LicenseFile
		wantSeverity types.AlertSeverity
	}{
		"Expired": {
			license: &LicenseFile{
				KeyPair: &liblicense.License{
					Cert: &x509.Certificate{
						NotAfter: time.Now().Add(time.Hour * -1),
					},
				},
			},
			wantSeverity: types.AlertSeverity_HIGH,
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
			require.Equal(t, 1, len(alerts))
			if len(alerts) == 1 {
				require.Equal(t, test.wantSeverity, alerts[0].Spec.Severity)
			}
		})
	}
}
