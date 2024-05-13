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
	"sync"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils"
)

// Mutex protected cache for memoizing functions which construct the CSP header string.
// This is necessary because the CSP header is constructed on every request to the web UI,
// so caching here has a significant performance impact.
type cspCache struct {
	sync.RWMutex
	entries map[string]string
}

func (c *cspCache) get(key string) (string, bool) {
	c.RLock()
	defer c.RUnlock()

	val, ok := c.entries[key]
	return val, ok
}

func (c *cspCache) set(key, value string) {
	c.Lock()
	defer c.Unlock()

	c.entries[key] = value
}

func newCSPCache() *cspCache {
	return &cspCache{
		entries: make(map[string]string),
	}
}

type cspMap map[string][]string

var defaultContentSecurityPolicy = cspMap{
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

var defaultFontSrc = cspMap{"font-src": {"'self'", "data:"}}
var defaultConnectSrc = cspMap{"connect-src": {"'self'", "wss:"}}

var stripeSecurityPolicy = cspMap{
	// auto-pay plans in Cloud use stripe.com to manage billing information
	"script-src": {"https://js.stripe.com"},
	"frame-src":  {"https://js.stripe.com"},
}

var wasmSecurityPolicy = cspMap{
	"script-src": {"'self'", "'wasm-unsafe-eval'"},
}

// combineCSPMaps combines multiple CSP maps into a single map.
// When multiple of the input cspMaps have the same key, their
// respective lists are concatenated.
func combineCSPMaps(cspMaps ...cspMap) cspMap {
	combinedMap := make(cspMap)

	for _, cspMap := range cspMaps {
		for key, value := range cspMap {
			combinedMap[key] = append(combinedMap[key], value...)
			combinedMap[key] = utils.Deduplicate(combinedMap[key])
		}
	}

	return combinedMap
}

// getContentSecurityPolicyString combines multiple CSP maps into a single
// CSP string, alphabetically sorted by the directive key.
// When multiple of the input cspMaps have the same key, their
// respective lists are concatenated.
func getContentSecurityPolicyString(cspMaps ...cspMap) string {
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

func getIndexContentSecurityPolicy(withStripe, withWasm bool) cspMap {
	cspMaps := []cspMap{defaultContentSecurityPolicy, defaultFontSrc, defaultConnectSrc}

	if withStripe {
		cspMaps = append(cspMaps, stripeSecurityPolicy)
	}

	if withWasm {
		cspMaps = append(cspMaps, wasmSecurityPolicy)
	}

	return combineCSPMaps(cspMaps...)
}

// desktopSessionRe is a regex that matches /web/cluster/:clusterId/desktops/:desktopName/:username
// which is a route to a desktop session that uses WASM.
var desktopSessionRe = regexp.MustCompile(`^/web/cluster/[^/]+/desktops/[^/]+/[^/]+$`)

// regex for the recordings endpoint /web/cluster/:clusterId/session/:sid
// which is a route to a desktop recording that uses WASM.
var recordingRe = regexp.MustCompile(`^/web/cluster/[^/]+/session/[^/]+$`)

var indexCSPStringCache *cspCache = newCSPCache()

func getIndexContentSecurityPolicyString(cfg proto.Features, urlPath string) string {
	// Check for result with this cfg and urlPath in cache
	withStripe := cfg.GetIsStripeManaged()
	key := fmt.Sprintf("%v-%v", withStripe, urlPath)
	if cspString, ok := indexCSPStringCache.get(key); ok {
		return cspString
	}

	// Nothing found in cache, calculate regex and result
	withWasm := desktopSessionRe.MatchString(urlPath) || recordingRe.MatchString(urlPath)
	cspString := getContentSecurityPolicyString(
		getIndexContentSecurityPolicy(withStripe, withWasm),
	)
	// Add result to cache
	indexCSPStringCache.set(key, cspString)

	return cspString
}

// SetIndexContentSecurityPolicy sets the Content-Security-Policy header for main index.html page
func SetIndexContentSecurityPolicy(h http.Header, cfg proto.Features, urlPath string) {
	cspString := getIndexContentSecurityPolicyString(cfg, urlPath)
	h.Set("Content-Security-Policy", cspString)
}

var redirectCSPStringCache *cspCache = newCSPCache()

func getRedirectPageContentSecurityPolicyString(scriptSrc string) string {
	if cspString, ok := redirectCSPStringCache.get(scriptSrc); ok {
		return cspString
	}

	cspString := getContentSecurityPolicyString(
		defaultContentSecurityPolicy,
		cspMap{
			"script-src": {"'" + scriptSrc + "'"},
		},
	)
	redirectCSPStringCache.set(scriptSrc, cspString)

	return cspString
}

func SetRedirectPageContentSecurityPolicy(h http.Header, scriptSrc string) {
	cspString := getRedirectPageContentSecurityPolicyString(scriptSrc)
	h.Set("Content-Security-Policy", cspString)
}

// SetWebConfigHeaders sets headers for webConfig.js
func SetWebConfigHeaders(h http.Header) {
	h.Set("Content-Type", "application/javascript")
}

// SetScriptHeaders sets headers for the teleport install script
func SetScriptHeaders(h http.Header) {
	h.Set("Content-Type", "text/x-shellscript")
}
