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
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

func TestConvertAuditEvent(t *testing.T) {
	anonymizer, err := utils.NewHMACAnonymizer("anon-key-or-cluster-id")
	require.NoError(t, err)

	cases := []struct {
		desc               string
		event              apievents.AuditEvent
		expected           Anonymizable
		expectedAnonymized *prehogv1a.SubmitEventRequest
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
			expectedAnonymized: &prehogv1a.SubmitEventRequest{
				Event: &prehogv1a.SubmitEventRequest_MfaAuthenticationEvent{
					MfaAuthenticationEvent: &prehogv1a.MFAAuthenticationEvent{
						UserName:          anonymizer.AnonymizeString("some-user"),
						DeviceId:          anonymizer.AnonymizeString("dev-id"),
						DeviceType:        "TOTP",
						MfaChallengeScope: "CHALLENGE_SCOPE_LOGIN",
					},
				},
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
			expectedAnonymized: &prehogv1a.SubmitEventRequest{
				Event: &prehogv1a.SubmitEventRequest_MfaAuthenticationEvent{
					MfaAuthenticationEvent: &prehogv1a.MFAAuthenticationEvent{
						UserName:          anonymizer.AnonymizeString("some-user"),
						DeviceId:          anonymizer.AnonymizeString(""),
						DeviceType:        "",
						MfaChallengeScope: "CHALLENGE_SCOPE_LOGIN",
					},
				},
			},
		},
		{
			desc: "DatabaseUserCreate",
			event: &apievents.DatabaseUserCreate{
				UserMetadata: apievents.UserMetadata{
					User: "alice",
				},
				DatabaseMetadata: apievents.DatabaseMetadata{
					DatabaseService:  "postgres-local",
					DatabaseProtocol: "postgres",
					DatabaseName:     "postgres",
					DatabaseUser:     "alice",
					DatabaseType:     "self-hosted",
					DatabaseOrigin:   "config-file",
					DatabaseRoles:    []string{"reader", "writer", "admin"},
				},
			},
			expected: &DatabaseUserCreatedEvent{
				Database: &prehogv1a.SessionStartDatabaseMetadata{
					DbType:     "self-hosted",
					DbProtocol: "postgres",
					DbOrigin:   "config-file",
				},
				UserName: "alice",
				NumRoles: 3,
			},
			expectedAnonymized: &prehogv1a.SubmitEventRequest{
				Event: &prehogv1a.SubmitEventRequest_DatabaseUserCreated{
					DatabaseUserCreated: &prehogv1a.DatabaseUserCreatedEvent{
						Database: &prehogv1a.SessionStartDatabaseMetadata{
							DbType:     "self-hosted",
							DbProtocol: "postgres",
							DbOrigin:   "config-file",
						},
						UserName: anonymizer.AnonymizeString("alice"),
						NumRoles: 3,
					},
				},
			},
		},
		{
			desc: "DatabasePermissionUpdate",
			event: &apievents.DatabasePermissionUpdate{
				UserMetadata: apievents.UserMetadata{
					User: "alice",
				},
				DatabaseMetadata: apievents.DatabaseMetadata{
					DatabaseService:  "postgres-local",
					DatabaseProtocol: "postgres",
					DatabaseName:     "postgres",
					DatabaseUser:     "alice",
					DatabaseType:     "self-hosted",
					DatabaseOrigin:   "config-file",
					DatabaseRoles:    []string{"reader", "writer", "admin"},
				},
				PermissionSummary: []apievents.DatabasePermissionEntry{
					{
						Permission: "SELECT",
						Counts:     map[string]int32{"table": 3},
					},
					{
						Permission: "UPDATE",
						Counts:     map[string]int32{"table": 6},
					},
				},
				AffectedObjectCounts: map[string]int32{"table": 7},
			},
			expected: &DatabaseUserPermissionsUpdateEvent{
				Database: &prehogv1a.SessionStartDatabaseMetadata{
					DbType:     "self-hosted",
					DbProtocol: "postgres",
					DbOrigin:   "config-file",
				},
				UserName:             "alice",
				NumTables:            7,
				NumTablesPermissions: 9,
			},
			expectedAnonymized: &prehogv1a.SubmitEventRequest{
				Event: &prehogv1a.SubmitEventRequest_DatabaseUserPermissionsUpdated{
					DatabaseUserPermissionsUpdated: &prehogv1a.DatabaseUserPermissionsUpdateEvent{
						Database: &prehogv1a.SessionStartDatabaseMetadata{
							DbType:     "self-hosted",
							DbProtocol: "postgres",
							DbOrigin:   "config-file",
						},
						UserName:             anonymizer.AnonymizeString("alice"),
						NumTables:            7,
						NumTablesPermissions: 9,
					},
				},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			actual := ConvertAuditEvent(tt.event)
			assert.Equal(t, tt.expected, actual)
			actualAnonymized := actual.Anonymize(anonymizer)
			assert.Equal(t, tt.expectedAnonymized, &actualAnonymized)
		})
	}
}
