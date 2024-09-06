/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestAlertSorting verifies the default cluster alert sorting.
func TestAlertSorting(t *testing.T) {
	start := time.Now()

	aa := []struct {
		t time.Time     // creation time
		s AlertSeverity // severity
		p int           // post-sort index
	}{
		{
			t: start.Add(time.Second * 2),
			s: AlertSeverity_HIGH,
			p: 1,
		},
		{
			t: start.Add(time.Second * 1),
			s: AlertSeverity_HIGH,
			p: 2,
		},
		{
			t: start.Add(time.Second * 2),
			s: AlertSeverity_LOW,
			p: 4,
		},
		{
			t: start.Add(time.Second * 3),
			s: AlertSeverity_HIGH,
			p: 0,
		},
		{
			t: start.Add(time.Hour),
			s: AlertSeverity_MEDIUM,
			p: 3,
		},
	}

	// build the alerts
	alerts := make([]ClusterAlert, 0, len(aa))
	for i, a := range aa {
		alert, err := NewClusterAlert(
			fmt.Sprintf("alert-%d", i),
			"uh-oh!",
			WithAlertCreated(a.t),
			WithAlertSeverity(a.s),
			WithAlertLabel("p", fmt.Sprintf("%d", a.p)),
		)
		require.NoError(t, err)
		alerts = append(alerts, alert)
	}

	// apply the default sorting
	SortClusterAlerts(alerts)

	// verify that post-sort labels now match order
	for i, a := range alerts {
		require.Equal(t, fmt.Sprintf("%d", i), a.Metadata.Labels["p"])
	}
}

// TestCheckAndSetDefaults verifies that only valid URLs are set on the link
// label and that only valid link text can be used.
func TestCheckAndSetDefaultsWithLink(t *testing.T) {
	tests := []struct {
		options []AlertOption
		name    string
		assert  require.ErrorAssertionFunc
	}{
		{
			name:    "valid link",
			options: []AlertOption{WithAlertLabel(AlertLink, "https://goteleport.com/docs")},
			assert:  require.NoError,
		},
		{
			name: "valid link with link text",
			options: []AlertOption{
				WithAlertLabel(AlertLink, "https://goteleport.com/support"),
				WithAlertLabel(AlertLinkText, "Contact Support"),
			},
			assert: require.NoError,
		},
		{
			name:    "invalid link",
			options: []AlertOption{WithAlertLabel(AlertLink, "h{t}tps://goteleport.com/docs")},
			assert:  require.Error,
		},
		{
			name:    "external link",
			options: []AlertOption{WithAlertLabel(AlertLink, "https://google.com")},
			assert:  require.Error,
		},
		{
			name: "valid link with invalid link text",
			options: []AlertOption{
				WithAlertLabel(AlertLink, "https://goteleport.com/support"),
				WithAlertLabel(AlertLinkText, "Contact!Support"),
			},
			assert: require.Error,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClusterAlert(
				fmt.Sprintf("name-%d", i),
				fmt.Sprintf("message-%d", i),
				tt.options...,
			)
			tt.assert(t, err)
		})
	}
}

// TestAlertAcknowledgement_Check verifies the validation of the arguments to ack an alert
func TestAlertAcknowledgement_Check(t *testing.T) {
	// some arbitrary expiry time
	expires := time.Now().Add(5 * time.Minute)

	testcases := []struct {
		desc    string
		ack     *AlertAcknowledgement
		wantErr bool
	}{
		{
			desc:    "empty",
			ack:     &AlertAcknowledgement{},
			wantErr: true,
		},
		{
			desc: "missing reason",
			ack: &AlertAcknowledgement{
				AlertID: "alert-id",
				Expires: expires,
			},
			wantErr: true,
		},
		{
			desc: "missing alert ID",
			ack: &AlertAcknowledgement{
				Expires: expires,
				Reason:  "some reason",
			},
			wantErr: true,
		},
		{
			desc: "missing expiry",
			ack: &AlertAcknowledgement{
				AlertID: "alert-id",
				Reason:  "some reason",
			},
			wantErr: true,
		},
		{
			desc: "success",
			ack: &AlertAcknowledgement{
				AlertID: "alert-id",
				Expires: expires,
				Reason:  "some reason",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.ack.Check()

			if !tc.wantErr {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.True(t,
				trace.IsBadParameter(err),
				"want BadParameter, got %v (%T)", err, trace.Unwrap(err))
		})
	}
}
