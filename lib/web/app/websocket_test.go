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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestParseCSWSHAction(t *testing.T) {
	for _, tc := range []struct {
		in            string
		report, block bool
	}{
		{"", false, false},
		{"off", false, false},
		{"none", false, false},
		{"garbage", false, false},
		{"report", true, false},
		{"REPORT", true, false},
		{" report ", true, false},
		{"block", false, true},
		{"Block", false, true},
		{"report-and-block", true, true},
		{"report_and_block", true, true},
		{"REPORT-AND-BLOCK", true, true},
		// Unknown combinations are not parsed loosely; only the exact tokens.
		{"block,report", false, false},
		{"report block", false, false},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got := parseCSWSHAction(tc.in)
			require.Equal(t, tc.report, got.report, "report")
			require.Equal(t, tc.block, got.block, "block")
		})
	}
}

func TestCSWSHActionEnabled(t *testing.T) {
	require.False(t, cswshAction{}.enabled())
	require.True(t, cswshAction{report: true}.enabled())
	require.True(t, cswshAction{block: true}.enabled())
	require.True(t, cswshAction{report: true, block: true}.enabled())
}

func TestHeaderListContainsToken(t *testing.T) {
	for _, tc := range []struct {
		header string
		token  string
		want   bool
	}{
		{"", "upgrade", false},
		{"upgrade", "upgrade", true},
		{"Upgrade", "upgrade", true},
		{"UPGRADE", "upgrade", true},
		{"keep-alive, Upgrade", "upgrade", true},
		{"keep-alive,Upgrade", "upgrade", true},
		{"  upgrade  ", "upgrade", true},
		{"keep-alive", "upgrade", false},
		// Token match, not substring.
		{"upgradeable", "upgrade", false},
		{"a, b, c", "b", true},
		{"a, b, c", "d", false},
	} {
		t.Run(tc.header+"|"+tc.token, func(t *testing.T) {
			require.Equal(t, tc.want, headerListContainsToken(tc.header, tc.token))
		})
	}
}

func TestIsWebSocketUpgrade(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{
			name:    "no relevant headers",
			headers: nil,
			want:    false,
		},
		{
			name:    "connection upgrade and upgrade websocket",
			headers: map[string]string{"Connection": "Upgrade", "Upgrade": "websocket"},
			want:    true,
		},
		{
			name:    "connection list with upgrade",
			headers: map[string]string{"Connection": "keep-alive, Upgrade", "Upgrade": "websocket"},
			want:    true,
		},
		{
			name:    "upgrade websocket mixed case",
			headers: map[string]string{"Connection": "Upgrade", "Upgrade": "WebSocket"},
			want:    true,
		},
		{
			name:    "connection upgrade without upgrade header",
			headers: map[string]string{"Connection": "Upgrade"},
			want:    false,
		},
		{
			name:    "upgrade header without connection",
			headers: map[string]string{"Upgrade": "websocket"},
			want:    false,
		},
		{
			name:    "upgrade to non-websocket protocol",
			headers: map[string]string{"Connection": "Upgrade", "Upgrade": "h2c"},
			want:    false,
		},
		{
			name:    "upgrade list with websocket first",
			headers: map[string]string{"Connection": "Upgrade", "Upgrade": "websocket, foo"},
			want:    true,
		},
		{
			name:    "upgrade list with websocket later",
			headers: map[string]string{"Connection": "Upgrade", "Upgrade": "foo, websocket"},
			want:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isWebSocketUpgrade(newWSTestRequest(tc.headers)))
		})
	}
}

func TestIsCrossOriginWebSocketUpgrade(t *testing.T) {
	originWS := func(origin string) map[string]string {
		if origin == "" {
			return nil
		}
		return map[string]string{"Origin": origin}
	}

	for _, tc := range []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{
			name:    "no origin (non-browser)",
			headers: originWS(""),
			want:    false,
		},
		{
			name:    "origin matches host",
			headers: originWS("https://app.example.com"),
			want:    false,
		},
		{
			name:    "origin host differs",
			headers: originWS("https://attacker.example"),
			want:    true,
		},
		{
			name:    "origin host differs by port",
			headers: originWS("https://app.example.com:8443"),
			want:    true,
		},
		{
			// A plain-HTTP page on the same host is a foreign origin: its
			// wss:// handshake still carries the Secure session cookies, so the
			// scheme mismatch must be treated as cross-origin.
			name:    "origin scheme differs (http) but host matches",
			headers: originWS("http://app.example.com"),
			want:    true,
		},
		{
			name:    "origin scheme differs (ws) but host matches",
			headers: originWS("ws://app.example.com"),
			want:    true,
		},
		{
			name:    "origin scheme uppercase https matches",
			headers: originWS("HTTPS://app.example.com"),
			want:    false,
		},
		{
			name:    "unparseable origin",
			headers: originWS("://://"),
			want:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isCrossOriginWebSocketUpgrade(newWSTestRequest(tc.headers)))
		})
	}
}

// TestGuardCrossSiteWebSocket exercises the end-to-end guard decision (detect a
// WebSocket upgrade, classify its origin, act per the configured action) using
// realistic handshake headers. It tests the guard directly rather than through
// the HTTP pipeline because Go's http.Client manages the hop-by-hop Connection
// header itself and won't deliver a verbatim "Connection: Upgrade".
func TestGuardCrossSiteWebSocket(t *testing.T) {
	// A realistic browser WebSocket handshake (Connection/Upgrade, no Sec-Fetch),
	// carrying the given Origin.
	ws := func(origin string) map[string]string {
		h := map[string]string{"Connection": "Upgrade", "Upgrade": "websocket"}
		if origin != "" {
			h["Origin"] = origin
		}
		return h
	}

	for _, tc := range []struct {
		name        string
		action      cswshAction
		headers     map[string]string
		wantBlocked bool
	}{
		{
			name:    "default no-op leaves cross-origin WebSocket alone",
			action:  cswshAction{},
			headers: ws("https://attacker.example"),
		},
		{
			name:    "report allows cross-origin WebSocket",
			action:  cswshAction{report: true},
			headers: ws("https://attacker.example"),
		},
		{
			name:        "block rejects cross-origin WebSocket",
			action:      cswshAction{block: true},
			headers:     ws("https://attacker.example"),
			wantBlocked: true,
		},
		{
			name:        "report-and-block rejects cross-origin WebSocket",
			action:      cswshAction{report: true, block: true},
			headers:     ws("https://attacker.example"),
			wantBlocked: true,
		},
		{
			name:    "block allows same-origin WebSocket",
			action:  cswshAction{block: true},
			headers: ws("https://app.example.com"),
		},
		{
			// Plain-HTTP page on the same host (e.g. injected before HSTS is
			// effective) is a foreign origin and must be blocked.
			name:        "block rejects same-host cross-scheme (http) WebSocket",
			action:      cswshAction{block: true},
			headers:     ws("http://app.example.com"),
			wantBlocked: true,
		},
		{
			name:    "block allows non-browser WebSocket with no Origin",
			action:  cswshAction{block: true},
			headers: ws(""),
		},
		{
			name:    "block leaves cross-origin non-WebSocket request alone",
			action:  cswshAction{block: true},
			headers: map[string]string{"Origin": "https://attacker.example"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{logger: slog.Default(), cswshAction: tc.action}
			err := h.guardCrossSiteWebSocket(newWSTestRequest(tc.headers))
			if tc.wantBlocked {
				require.True(t, trace.IsAccessDenied(err), "want AccessDenied, got %v", err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// newWSTestRequest builds a request to https://app.example.com with the given
// headers (so r.Host is "app.example.com" for Origin/Host comparisons).
func newWSTestRequest(headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "https://app.example.com/socket", nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}
