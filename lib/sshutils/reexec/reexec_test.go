/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package reexec

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
)

func TestReadChildError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		readErr      error
		childErrIn   string
		wantChildErr string
		context      *ErrorContext
	}{
		{
			name:         "empty stderr",
			childErrIn:   "",
			wantChildErr: "",
		},
		{
			name:         "has stderr",
			childErrIn:   "Failed to launch: test error.\n",
			wantChildErr: "Failed to launch: test error.\n",
		},
		{
			name:         "stderr at max read limit",
			childErrIn:   strings.Repeat("a", maxRead),
			wantChildErr: strings.Repeat("a", maxRead),
		},
		{
			name:         "stderr over max read limit is truncated",
			childErrIn:   strings.Repeat("a", maxRead) + "b",
			wantChildErr: strings.Repeat("a", maxRead),
		},
		{
			name:    "read error",
			readErr: errors.New("read failure"),
		},
		{
			name:       "unknown user error with mixed host user creation decisions gets contextualized",
			childErrIn: "Failed to launch: user: unknown user teleport-test-user-does-not-exist-reexec.\n",
			context: &ErrorContext{
				Login: "teleport-test-user-does-not-exist-reexec",
				DecisionContext: &decisionpb.SSHAccessPermitContext{
					HostUserCreationAllowedBy: []*decisionpb.Determinant{
						{Kind: "role", Name: "allow-role"},
					},
					HostUserCreationDeniedBy: []*decisionpb.Determinant{
						{Kind: "role", Name: "deny-role"},
					},
				},
			},
			wantChildErr: "Failed to launch: user: unknown user teleport-test-user-does-not-exist-reexec: host user creation denied by the following resources: [role: \"deny-role\"]\n",
		},
		{
			name:       "pam context error for unknown user gets contextualized",
			childErrIn: "Failed to launch: failed to open PAM context: pam_start failed.\n",
			context: &ErrorContext{
				Login: "teleport-test-user-does-not-exist-pam",
				DecisionContext: &decisionpb.SSHAccessPermitContext{
					HostUserCreationAllowedBy: []*decisionpb.Determinant{
						{Kind: "role", Name: "allow-role"},
					},
					HostUserCreationDeniedBy: []*decisionpb.Determinant{
						{Kind: "role", Name: "deny-role"},
					},
				},
			},
			wantChildErr: "Failed to launch: failed to open PAM context: pam_start failed: host user creation denied by the following resources: [role: \"deny-role\"]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.readErr != nil {
				_, err := ReadChildErrorWithContext(iotest.ErrReader(tt.readErr), tt.context)
				require.ErrorIs(t, err, tt.readErr)
				return
			}

			got, err := ReadChildErrorWithContext(strings.NewReader(tt.childErrIn), tt.context)
			require.NoError(t, err)
			require.Equal(t, tt.wantChildErr, got)
		})
	}
}
