/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package httplib implements common utility functions for writing
// classic HTTP handlers
package httplib

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport/api/client/proto"
)

var defaultContentSecurityPolicy = map[string]string{
	"default-src": "'self'",
	// specify CSP directives not covered by `default-src`
	"base-uri":        "'self'",
	"form-action":     "'self'",
	"frame-ancestors": "'none'",
	// additional default restrictions
	"object-src": "'none'",
	"img-src":    "'self' data: blob:",
	"style-src":  "'self' 'unsafe-inline'",
}

var defaultFontSrc = map[string]string{"font-src": "'self' data:"}

var stripeSecurityPolicy = map[string]string{
	// auto-pay plans in Cloud use stripe.com to manage billing information
	"script-src": "'self' https://js.stripe.com",
	"frame-src":  "https://js.stripe.com",
}

// combineCSPMaps combines multiple CSP maps into a single map.
// When multiple of the input cspMaps have the same key, the
// latter map's value takes precedence.
func combineCSPMaps(cspMaps ...map[string]string) map[string]string {
	combined := make(map[string]string)
	for _, cspMap := range cspMaps {
		maps.Copy(combined, cspMap)
	}
	return combined
}

// getContentSecurityPolicyString combines multiple CSP maps into a single
// CSP string, alphabetically sorted by the directive key.
// When multiple of the input cspMaps have the same key, the
// latter map's value takes precedence.
func getContentSecurityPolicyString(cspMaps ...map[string]string) string {
	combined := combineCSPMaps(cspMaps...)

	keys := make([]string, 0, len(combined))
	for k := range combined {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var cspStringBuilder strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&cspStringBuilder, "%s %s; ", k, combined[k])
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

// SetIndexContentSecurityPolicy sets the Content-Security-Policy header for main index.html page
func SetIndexContentSecurityPolicy(h http.Header, cfg proto.Features) {
	cspMaps := []map[string]string{
		defaultContentSecurityPolicy,
		defaultFontSrc,
		{"connect-src": "'self' wss:"},
	}

	if cfg.GetCloud() && cfg.GetIsUsageBased() {
		cspMaps = append(cspMaps, stripeSecurityPolicy)
	}

	cspString := getContentSecurityPolicyString(cspMaps...)
	h.Set("Content-Security-Policy", cspString)
}

// SetAppLaunchContentSecurityPolicy sets the Content-Security-Policy header for /web/launch
func SetAppLaunchContentSecurityPolicy(h http.Header, applicationURL string) {
	cspString := getContentSecurityPolicyString(
		defaultContentSecurityPolicy,
		defaultFontSrc,
		map[string]string{
			"connect-src": "'self' " + applicationURL,
		},
	)

	h.Set("Content-Security-Policy", cspString)
}

func SetRedirectPageContentSecurityPolicy(h http.Header, scriptSrc string) {
	cspString := getContentSecurityPolicyString(
		defaultContentSecurityPolicy,
		map[string]string{
			"script-src": "'" + scriptSrc + "'",
		},
	)

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
