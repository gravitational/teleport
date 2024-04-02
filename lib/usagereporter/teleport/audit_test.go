// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package usagereporter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	apievents "github.com/gravitational/teleport/api/types/events"
)

func TestConvertAuditEvent(t *testing.T) {
	cases := []struct {
		desc     string
		event    apievents.AuditEvent
		expected Anonymizable
	}{
		{
			desc: "ValidateMFAAuthResponse",
			event: &apievents.ValidateMFAAuthResponse{
				UserMetadata: apievents.UserMetadata{
					User: "some-user",
				},
				MFADevice: &apievents.MFADeviceMetadata{
					DeviceID:   "dev-id",
					DeviceType: "TOTP",
				},
				ChallengeScope: "CHALLENGE_SCOPE_LOGIN",
			},
			expected: &MFAAuthenticationEvent{
				UserName:          "some-user",
				DeviceId:          "dev-id",
				DeviceType:        "TOTP",
				MfaChallengeScope: "CHALLENGE_SCOPE_LOGIN",
			},
		},
		{
			desc: "ValidateMFAAuthResponse without MFADevice",
			event: &apievents.ValidateMFAAuthResponse{
				UserMetadata: apievents.UserMetadata{
					User: "some-user",
				},
				ChallengeScope: "CHALLENGE_SCOPE_LOGIN",
			},
			expected: &MFAAuthenticationEvent{
				UserName:          "some-user",
				DeviceId:          "",
				DeviceType:        "",
				MfaChallengeScope: "CHALLENGE_SCOPE_LOGIN",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			actual := ConvertAuditEvent(tt.event)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
