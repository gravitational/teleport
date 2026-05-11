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

package app

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/srv/app/policy"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestApplyPolicies(t *testing.T) {
	gitlab := mustCompilePolicies(t, []policy.Spec{
		{
			Name: "no-admin",
			Deny: []policy.RuleSpec{{
				Paths:      []string{"/admin/**"},
				ReasonCode: "admin_blocked",
				Reason:     "Admin endpoints are platform-team only.",
			}},
		},
		{
			Name: "read-only",
			Allow: []policy.RuleSpec{{
				Paths:   []string{"/api/**"},
				Methods: []string{"GET", "HEAD"},
			}},
		},
	})

	tests := []struct {
		name        string
		policiesFor string // app name in c.cfg.Policies
		app         string // app name on the request
		method      string
		path        string
		wantAnswer  bool   // applyPolicies return value
		wantStatus  int    // response status when wantAnswer
		wantReason  string // Teleport-Decision-Reason header
		wantAudit   bool   // expect an AppSessionRequest event emitted
	}{
		{
			name:        "no policies short-circuits",
			policiesFor: "",
			app:         "unrelated",
			method:      "GET",
			path:        "/anything",
			wantAnswer:  false,
		},
		{
			name:        "deny via path rule",
			policiesFor: "gitlab",
			app:         "gitlab",
			method:      "POST",
			path:        "/admin/users",
			wantAnswer:  true,
			wantStatus:  http.StatusForbidden,
			wantReason:  "admin_blocked",
			wantAudit:   true,
		},
		{
			name:        "allow via path rule",
			policiesFor: "gitlab",
			app:         "gitlab",
			method:      "GET",
			path:        "/api/v4/projects/1",
			wantAnswer:  false,
		},
		{
			name:        "no allow matched",
			policiesFor: "gitlab",
			app:         "gitlab",
			method:      "POST",
			path:        "/api/v4/projects/1",
			wantAnswer:  true,
			wantStatus:  http.StatusForbidden,
			wantReason:  policy.ReasonNoMatchingAllow,
			wantAudit:   true,
		},
		{
			name:        "leading double slash bypass blocked",
			policiesFor: "gitlab",
			app:         "gitlab",
			method:      "GET",
			path:        "//admin/users",
			wantAnswer:  true,
			wantStatus:  http.StatusForbidden,
			wantReason:  policy.ReasonPathDecodeFailed,
			wantAudit:   true,
		},
		{
			name:        "method is upper-cased",
			policiesFor: "gitlab",
			app:         "gitlab",
			method:      "get",
			path:        "/api/v4/projects/1",
			wantAnswer:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emitter := &eventstest.MockRecorderEmitter{}
			c := &ConnectionsHandler{
				cfg: &ConnectionsHandlerConfig{
					Emitter: emitter,
				},
				log: slog.Default(),
			}
			app := newTestApp(t, tc.app)
			if tc.policiesFor != "" {
				// policiesFor stores the test-case "key" we want to use;
				// the handler looks up by public_addr, so build the map
				// using the test app's public_addr to keep the table
				// driven by short app names while exercising the real
				// keying contract.
				c.cfg.Policies = map[string][]policy.Policy{app.GetPublicAddr(): gitlab}
			}
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			identity := &tlsca.Identity{Username: "alice"}

			got := c.applyPolicies(rec, req, identity, app)
			require.Equal(t, tc.wantAnswer, got)
			if tc.wantAnswer {
				require.Equal(t, tc.wantStatus, rec.Code)
				require.Equal(t, tc.wantReason, rec.Header().Get("Teleport-Decision-Reason"))
				require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
				var body struct {
					ReasonCode string `json:"reason_code"`
					Reason     string `json:"reason"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				require.Equal(t, tc.wantReason, body.ReasonCode)
				require.NotEmpty(t, body.Reason)
			}
			if tc.wantAudit {
				require.Len(t, emitter.Events(), 1)
				evt, ok := emitter.Events()[0].(*apievents.AppSessionRequest)
				require.True(t, ok, "expected AppSessionRequest, got %T", emitter.Events()[0])
				require.Equal(t, uint32(http.StatusForbidden), evt.StatusCode)
				require.Equal(t, tc.app, evt.AppName)
			} else {
				require.Empty(t, emitter.Events())
			}
		})
	}
}

func mustCompilePolicies(t *testing.T, specs []policy.Spec) []policy.Policy {
	t.Helper()
	p, err := policy.CompileAll(specs)
	require.NoError(t, err)
	return p
}

func newTestApp(t *testing.T, name string) types.Application {
	t.Helper()
	app, err := types.NewAppV3(
		types.Metadata{Name: name},
		types.AppSpecV3{URI: "http://localhost", PublicAddr: name + ".t.tp"},
	)
	require.NoError(t, err)
	return app
}

// TestApplyPolicies_KeyByPublicAddrPreventsNameCollision locks in the
// fix for the bot-flagged name-collision bug: two apps that share a
// name but differ in public_addr must not share policies.
func TestApplyPolicies_KeyByPublicAddrPreventsNameCollision(t *testing.T) {
	policies, err := policy.CompileAll([]policy.Spec{{
		Name: "block-admin",
		Deny: []policy.RuleSpec{{
			Paths:      []string{"/admin/**"},
			ReasonCode: "admin_blocked",
		}},
	}})
	require.NoError(t, err)

	gated, err := types.NewAppV3(
		types.Metadata{Name: "shared"},
		types.AppSpecV3{URI: "http://a", PublicAddr: "gated.t.tp"},
	)
	require.NoError(t, err)

	ungated, err := types.NewAppV3(
		types.Metadata{Name: "shared"},
		types.AppSpecV3{URI: "http://b", PublicAddr: "ungated.t.tp"},
	)
	require.NoError(t, err)

	c := &ConnectionsHandler{
		cfg: &ConnectionsHandlerConfig{
			Emitter: &eventstest.MockRecorderEmitter{},
			Policies: map[string][]policy.Policy{
				"gated.t.tp": policies,
			},
		},
		log: slog.Default(),
	}
	identity := &tlsca.Identity{Username: "alice"}

	// gated app must enforce the policy
	rec := httptest.NewRecorder()
	got := c.applyPolicies(rec, httptest.NewRequest("GET", "/admin/x", nil), identity, gated)
	require.True(t, got)
	require.Equal(t, http.StatusForbidden, rec.Code)

	// ungated app shares the name but the lookup keyed by public_addr
	// finds no policies for it, so the gate passes through.
	rec = httptest.NewRecorder()
	got = c.applyPolicies(rec, httptest.NewRequest("GET", "/admin/x", nil), identity, ungated)
	require.False(t, got)
}
