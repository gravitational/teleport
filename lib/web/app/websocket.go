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
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
)

// cswshActionEnv is the environment variable that controls how the app proxy
// responds to a detected cross-origin WebSocket upgrade (cross-site WebSocket
// hijacking, CSWSH). Recognized values: "report", "block", "report-and-block".
// Any other value, including unset, does nothing.
const cswshActionEnv = "TELEPORT_UNSTABLE_APP_CSWSH_ACTION"

// cswshAction is the parsed TELEPORT_UNSTABLE_APP_CSWSH_ACTION behavior. report
// and block are independent: unset of both is the default no-op.
type cswshAction struct {
	report bool
	block  bool
}

// enabled reports whether the action does anything (report and/or block). The
// zero value is disabled, which is the safe default.
func (a cswshAction) enabled() bool { return a.report || a.block }

// parseCSWSHAction parses the TELEPORT_UNSTABLE_APP_CSWSH_ACTION value. Unknown
// or empty values disable the guard (do nothing) so the default is always safe.
func parseCSWSHAction(v string) cswshAction {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "report":
		return cswshAction{report: true}
	case "block":
		return cswshAction{block: true}
	case "report-and-block", "report_and_block":
		return cswshAction{report: true, block: true}
	default:
		return cswshAction{}
	}
}

// guardCrossSiteWebSocket detects cross-origin WebSocket upgrades on the app
// forward path and reacts per the configured action (report, block, both, or —
// by default — nothing).
//
// Scope: only a browser attaches the victim's ambient cookies, so CSWSH is
// inherently a browser attack, and we police it using signals a browser is forced
// to send and a script cannot forge (the Origin header — see
// isCrossOriginWebSocketUpgrade). Non-browser clients carry no such cookies and no
// Origin, so they pass through. Same-origin upgrades (the app's own page) always
// pass. Plain HTTP CSRF is out of scope.
func (h *Handler) guardCrossSiteWebSocket(r *http.Request) error {
	if !h.cswshAction.enabled() {
		return nil
	}
	if !isWebSocketUpgrade(r) {
		return nil
	}
	if !isCrossOriginWebSocketUpgrade(r) {
		return nil
	}

	if h.cswshAction.report {
		h.logger.WarnContext(r.Context(), "Detected cross-origin WebSocket upgrade to application",
			"blocked", h.cswshAction.block,
			"sec_fetch_site", r.Header.Get("Sec-Fetch-Site"),
			"origin", r.Header.Get("Origin"),
			"host", r.Host,
			"path", r.URL.Path,
		)
	}

	if h.cswshAction.block {
		return trace.AccessDenied("cross-origin WebSocket upgrade rejected")
	}
	return nil
}

// isCrossOriginWebSocketUpgrade reports whether a WebSocket upgrade was initiated
// from an origin other than the app's own. r must already be a WebSocket upgrade
// (callers check isWebSocketUpgrade first).
//
// Origin is the signal: RFC 6455 §4.1 requires browser clients to send it on the
// handshake and §10.2 sanctions rejecting on it; OWASP's WebSocket Security Cheat
// Sheet likewise makes Origin validation the primary CSWSH defense. A script
// cannot forge it, and non-browser clients (which carry no ambient cookies to
// abuse) may omit it. A present Origin whose host differs from the app's is
// cross-origin.
//
// We do not consult Sec-Fetch-Site: browsers don't attach it to WebSocket
// handshakes, and where it is present it agrees with the Origin comparison. We
// also do not distinguish same-site from cross-site — that would need the Public
// Suffix List and is deferred; today any foreign origin is cross-origin.
func isCrossOriginWebSocketUpgrade(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return true // unparseable Origin: treat as foreign
	}
	// An origin is (scheme, host, port), so compare scheme too (port is part of
	// Host). App access is always TLS, so a plain-HTTP page on the same host is a
	// foreign origin whose wss:// handshake still carries the Secure cookies.
	return !strings.EqualFold(u.Scheme, "https") || !strings.EqualFold(u.Host, r.Host)
}

// isWebSocketUpgrade reports whether r is a WebSocket upgrade handshake. A
// handshake is an HTTP GET carrying Connection: Upgrade and Upgrade: websocket
// (RFC 6455). Both are forbidden header names, so a script cannot forge them on a
// non-WebSocket request. Both are RFC 7230 comma-separated token lists, so match
// each as a token rather than by exact value.
//
// This detects HTTP/1.1 upgrades only, which is all the app forward path relays
// (httputil.ReverseProxy switches protocols on a 101 response). HTTP/2 WebSockets
// (RFC 8441 Extended CONNECT) carry no Connection/Upgrade headers; if WebSocket
// forwarding over HTTP/2 is ever added, extend this to also match :method=CONNECT
// with :protocol=websocket.
func isWebSocketUpgrade(r *http.Request) bool {
	return headerListContainsToken(r.Header.Get("Connection"), "upgrade") &&
		headerListContainsToken(r.Header.Get("Upgrade"), "websocket")
}

// headerListContainsToken reports whether a comma-separated header value
// contains the given token, case-insensitively.
func headerListContainsToken(headerValue, token string) bool {
	for part := range strings.SplitSeq(headerValue, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}
