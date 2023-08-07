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

package common

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type mockedAlertGetter struct {
	alerts []types.ClusterAlert
}

func (mag *mockedAlertGetter) GetClusterAlerts(ctx context.Context, query types.GetClusterAlertsRequest) ([]types.ClusterAlert, error) {
	return mag.alerts, nil
}

func mockAlertGetter(alerts []types.ClusterAlert) ClusterAlertGetter {
	return &mockedAlertGetter{
		alerts: alerts,
	}
}

func TestShowClusterAlerts(t *testing.T) {
	tests := map[string]struct {
		alerts  []types.ClusterAlert
		wantOut string
	}{
		"Single message": {
			alerts: []types.ClusterAlert{
				{
					Spec: types.ClusterAlertSpec{
						Severity: types.AlertSeverity_MEDIUM,
						Message:  "someMessage",
					},
				},
			},
			wantOut: "\x1b[33msomeMessage\x1b[0m\n\n",
		},
		"Sorted messages": {
			alerts: []types.ClusterAlert{
				{
					ResourceHeader: types.ResourceHeader{
						Metadata: types.Metadata{
							Labels: map[string]string{
								"someLabel": "yes",
							},
						},
					},
					Spec: types.ClusterAlertSpec{
						Severity: types.AlertSeverity_MEDIUM,
						Message:  "someOtherMessage",
					},
				}, {
					Spec: types.ClusterAlertSpec{
						Severity: types.AlertSeverity_HIGH,
						Message:  "someMessage",
					},
				},
			},

			wantOut: "\x1b[31msomeMessage\x1b[0m\n\n\x1b[33msomeOtherMessage\x1b[0m\n\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			alertGetter := mockAlertGetter(test.alerts)
			var got bytes.Buffer
			err := ShowClusterAlerts(context.Background(), alertGetter, &got, nil, types.AlertSeverity_LOW)
			require.NoError(t, err)
			require.Equal(t, test.wantOut, got.String())
		})
	}
}
