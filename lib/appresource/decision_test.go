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

package appresource

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDecisionJSON pins the flat wire form of a decision, the payload
// the audit event and the tctl evaluate output carry.
func TestDecisionJSON(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
		want     string
	}{
		{
			name: "allow with vars and code",
			decision: Decision{
				Allowed: true,
				Allow: &AllowDetails{
					Vars:   map[string]string{"project": "42"},
					Code:   "repo_read",
					Reason: "Read access to the repository API",
				},
				EvaluatedRoles: []string{"developer"},
			},
			want: `{
				"allowed": true,
				"evaluated_roles": ["developer"],
				"vars": {"project": "42"},
				"allow_code": "repo_read",
				"allow_reason": "Read access to the repository API"
			}`,
		},
		{
			name: "bare allow omits unset fields",
			decision: Decision{
				Allowed:        true,
				Allow:          &AllowDetails{},
				EvaluatedRoles: []string{"developer"},
			},
			want: `{"allowed": true, "evaluated_roles": ["developer"]}`,
		},
		{
			name: "deny with hints",
			decision: Decision{
				Deny: &DenyDetails{
					Kind: DenyNotAllowed,
					Hints: []Hint{
						{Code: "project_not_allowed", Reason: "Project is not in the caller's allowlist"},
						{Code: "needs_dev"},
					},
				},
				EvaluatedRoles: []string{"developer", "reader"},
			},
			want: `{
				"allowed": false,
				"evaluated_roles": ["developer", "reader"],
				"deny_kind": "teleport_request_not_allowed",
				"hints": [
					{"code": "project_not_allowed", "reason": "Project is not in the caller's allowlist"},
					{"code": "needs_dev"}
				]
			}`,
		},
		{
			name: "invalid request deny",
			decision: Decision{
				Deny:           &DenyDetails{Kind: DenyInvalidRequest},
				EvaluatedRoles: []string{"developer"},
			},
			want: `{
				"allowed": false,
				"evaluated_roles": ["developer"],
				"deny_kind": "teleport_invalid_request"
			}`,
		},
		{
			name: "misconfigured default-deny omits evaluated_roles",
			decision: Decision{
				Deny: &DenyDetails{Kind: DenyNotAllowed},
			},
			want: `{"allowed": false, "deny_kind": "teleport_request_not_allowed"}`,
		},
		{
			// A decision whose detail contradicts the verdict projects the
			// verdict side only, so a malformed value cannot emit deny keys
			// on an allow.
			name: "mismatched detail is dropped",
			decision: Decision{
				Allowed: true,
				Deny:    &DenyDetails{Kind: DenyNotAllowed},
			},
			want: `{"allowed": true}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.decision)
			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(got))
		})
	}
}
