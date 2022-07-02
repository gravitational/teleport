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

package reporting

import (
	"bytes"
	"sort"
	"testing"

	"github.com/gravitational/reporting/types"
	"github.com/stretchr/testify/require"
)

func TestWriteStatus(t *testing.T) {
	t.Parallel()
	notification := types.Notification{
		Type:     "licenseExpired",
		Severity: "error",
		Text:     "Your Teleport license has expired. If you are the System Administrator, please reach out to your Account Manager and obtain a new license to continue using Teleport.",
		HTML:     "Your Teleport license has expired. If you are the System Administrator, please reach out to your Account Manager and obtain a new license to continue using Teleport.",
	}
	b := bytes.Buffer{}

	in := types.NewHeartbeat(notification)
	require.NoError(t, writeLicenseStatus(&b, in))

	out, err := types.UnmarshalHeartbeat(b.Bytes())
	require.NoError(t, err)
	require.Len(t, out.Spec.Notifications, 1)
	require.Equal(t, notification, out.Spec.Notifications[0])
}

func TestGetWarnings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc          string
		notifications []types.Notification
		warnings      []string
	}{
		{
			desc:     "expected no warnings",
			warnings: []string{},
		},
		{
			desc: "expected licenseExpired warning",
			notifications: []types.Notification{
				{
					Type:     "licenseExpired",
					Severity: "error",
					Text:     "licenseExpired warning",
					HTML:     "licenseExpired warning",
				},
			},
			warnings: []string{
				"licenseExpired warning",
			},
		},
		{
			desc: "expected only licenseExpired warning",
			notifications: []types.Notification{
				{
					Type:     "licenseExpired",
					Severity: "error",
					Text:     "licenseExpired warning",
					HTML:     "licenseExpired warning",
				},
				{
					Type:     "test",
					Severity: "error",
					Text:     "test warning",
					HTML:     "test warning",
				},
			},
			warnings: []string{
				"licenseExpired warning",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			heartbeat := types.NewHeartbeat(tt.notifications...)
			b, err := types.MarshalHeartbeat(*heartbeat)
			require.NoError(t, err)

			warnings, err := getLicenseWarnings(bytes.NewReader(b))
			require.NoError(t, err)
			sort.Strings(warnings)
			require.Equal(t, tt.warnings, warnings)
		})
	}
}
