package usagereporter

import (
	"testing"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/stretchr/testify/assert"
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
