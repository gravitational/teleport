/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

// Package httplib implements common utility functions for writing
// classic HTTP handlers
package httplib

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/utils"
)

// CSPMap holds a map of Content Security Policy directives.
type CSPMap map[string][]string

var (
	defaultContentSecurityPolicy = CSPMap{
		"default-src": {"'self'"},
		"script-src":  {"'self'"},
		// specify CSP directives not covered by `default-src`
		"base-uri":        {"'self'"},
		"form-action":     {"'self'"},
		"frame-ancestors": {"'none'"},
		// additional default restrictions
		"object-src": {"'none'"},
		"img-src":    {"'self'", "data:", "blob:"},
		"style-src":  {"'self'", "'unsafe-inline'"},
	}
	defaultFontSrc     = CSPMap{"font-src": {"'self'", "data:"}}
	defaultConnectSrc  = CSPMap{"connect-src": {"'self'", "wss:"}}
	wasmSecurityPolicy = CSPMap{"script-src": {"'self'", "'wasm-unsafe-eval'"}}
)

// combineCSPMaps combines multiple CSP maps into a single map.
// When multiple of the input CSPMap have the same key, their
// respective lists are concatenated.
func combineCSPMaps(cspMaps ...CSPMap) CSPMap {
	combinedMap := make(CSPMap)

	for _, cspMap := range cspMaps {
		for key, value := range cspMap {
			combinedMap[key] = append(combinedMap[key], value...)
			combinedMap[key] = utils.Deduplicate(combinedMap[key])
		}
	}

	return combinedMap
}

// GetContentSecurityPolicyString combines multiple CSP maps into a single
// CSP string, alphabetically sorted by the directive key.
// When multiple of the input cspMaps have the same key, their
// respective lists are concatenated.
func GetContentSecurityPolicyString(cspMaps ...CSPMap) string {
	combined := combineCSPMaps(cspMaps...)

	keys := make([]string, 0, len(combined))
	for k := range combined {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var cspStringBuilder strings.Builder
	for _, k := range keys {
		cspStringBuilder.WriteString(k)
		for _, v := range combined[k] {
			cspStringBuilder.WriteString(" ")
			cspStringBuilder.WriteString(v)
		}
		cspStringBuilder.WriteString("; ")
	}

	return strings.TrimSpace(cspStringBuilder.String())
}

// SetNoCacheHeaders tells proxies and browsers do not cache the content
func SetNoCacheHeaders(h http.Header) {
	h.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	h.Set("Pragma", "no-cache")
	h.Set("Expires", "0")
}

// SetCacheHeaders tells proxies and browsers to cache the content
func SetCacheHeaders(h http.Header, maxAge time.Duration) {
	h.Set("Cache-Control", fmt.Sprintf("max-age=%.f, immutable", maxAge.Seconds()))
}

// SetEntityTagCacheHeaders tells proxies and browsers to cache the content
// and sets an ETag based on teleport version which can be used to check for modifications
func SetEntityTagCacheHeaders(h http.Header, etag string) {
	h.Set("Cache-Control", "no-cache")
	h.Set("ETag", etag)
}

// SetDefaultSecurityHeaders adds headers that should generally be considered safe defaults.  It is expected that all
// responses should be able to add these headers without negative impact.
func SetDefaultSecurityHeaders(h http.Header) {
	// Prevent web browsers from using content sniffing to discover a file’s MIME type
	h.Set("X-Content-Type-Options", "nosniff")

	// Only send the origin of the document as the referrer in all cases.  The use of `strict-origin` will also prevent
	// the sending of the origin if a request is downgraded from https to http.
	// The document https://example.com/page.html will send the referrer https://example.com/.
	h.Set("Referrer-Policy", "strict-origin")

	// X-Frame-Options indicates that the page can only be displayed in iframe on the same origin as the page itself
	h.Set("X-Frame-Options", "SAMEORIGIN")

	// X-XSS-Protection is a feature of Internet Explorer, Chrome and Safari that stops pages
	// from loading when they detect reflected cross-site scripting (XSS) attacks.
	h.Set("X-XSS-Protection", "1; mode=block")

	// Once a supported browser receives this header that browser will prevent any communications from
	// being sent over HTTP to the specified domain and will instead send all communications over HTTPS.
	// It also prevents HTTPS click through prompts on browsers
	h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
}

func getIndexContentSecurityPolicy(withWasm bool) CSPMap {
	cspMaps := []CSPMap{defaultContentSecurityPolicy, defaultFontSrc, defaultConnectSrc}

	if withWasm {
		cspMaps = append(cspMaps, wasmSecurityPolicy)
	}

	return combineCSPMaps(cspMaps...)
}

var (
	// desktopSessionRe is a regex that matches /web/cluster/:clusterId/desktops/:desktopName/:username
	// which is a route to a Windows desktop session that uses WASM.
	desktopSessionRe = regexp.MustCompile(`^/web/cluster/[^/]+/desktops/[^/]+/[^/]+$`)

	// linuxDesktopSessionRe is a regex that matches /web/cluster/:clusterId/linux_desktops/:desktopName/:username
	// which is a route to a Linux desktop session that uses WASM.
	linuxDesktopSessionRe = regexp.MustCompile(`^/web/cluster/[^/]+/linux_desktops/[^/]+/[^/]+$`)

	// recordingRe is a regex for the recordings endpoint /web/cluster/:clusterId/session/:sid
	// which is a route to a desktop recording that uses WASM.
	recordingRe = regexp.MustCompile(`^/web/cluster/[^/]+/session/[^/]+$`)

	// sshSessionRe is a regex for the ssh terminal endpoint /web/cluster/:clusterId/console/node/:sid/:login
	// which is a route to a SSH session that uses WASM.
	// (Note: the WASM dependency comes from xterm.js extensions)
	sshSessionRe = regexp.MustCompile(`^/web/cluster/[^/]+/console/node/[^/]+/[^/]+$`)
)

const (
	cspHeaderName         = "Content-Security-Policy"
	contentTypeHeaderName = "Content-Type"
)

var (
	indexCSPWithWASM    = GetContentSecurityPolicyString(getIndexContentSecurityPolicy(true))
	indexCSPWithoutWASM = GetContentSecurityPolicyString(getIndexContentSecurityPolicy(false))
)

func getIndexContentSecurityPolicyString(urlPath string) string {
	if wasm := desktopSessionRe.MatchString(urlPath) ||
		linuxDesktopSessionRe.MatchString(urlPath) ||
		recordingRe.MatchString(urlPath) ||
		sshSessionRe.MatchString(urlPath); wasm {
		return indexCSPWithWASM
	}
	return indexCSPWithoutWASM
}

// SetIndexContentSecurityPolicy sets the Content-Security-Policy header for main index.html page.
func SetIndexContentSecurityPolicy(h http.Header, urlPath string) {
	h.Set(cspHeaderName, getIndexContentSecurityPolicyString(urlPath))
}

// SetRedirectPageContentSecurityPolicy sets the Content-Security-Policy header
// for when Teleport serves a redirect.
func SetRedirectPageContentSecurityPolicy(h http.Header, scriptSrc string) {
	h.Set(
		cspHeaderName,
		GetContentSecurityPolicyString(
			defaultContentSecurityPolicy,
			CSPMap{"script-src": {"'" + scriptSrc + "'"}},
		))
}

// SetScriptHeaders sets headers for Teleport install scripts.
func SetScriptHeaders(h http.Header) {
	h.Set(contentTypeHeaderName, "text/x-shellscript")
}
