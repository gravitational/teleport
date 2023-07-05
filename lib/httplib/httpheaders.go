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
	"strings"
)

// SetNoCacheHeaders tells proxies and browsers do not cache the content
func SetNoCacheHeaders(h http.Header) {
	h.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	h.Set("Pragma", "no-cache")
	h.Set("Expires", "0")
}

// SetDefaultSecurityHeaders adds headers that should generally be considered safe defaults.  It is expected that all
// responses should be able to add these headers without negative impact.
func SetDefaultSecurityHeaders(h http.Header) {
	// Prevent web browsers from using content sniffing to discover a file’s MIME type
	h.Set("X-Content-Type-Options", "nosniff")

	// Only send the origin of the document as the referrer in all cases.
	// The document https://example.com/page.html will send the referrer https://example.com/.
	h.Set("Referrer-Policy", "origin")

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
func SetIndexContentSecurityPolicy(h http.Header) {
	cspValue := strings.Join([]string{
		GetDefaultContentSecurityPolicy(),
		// 'unsafe-inline' is required by CSS-in-JS to work
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"font-src 'self' data:",
		"connect-src 'self' wss:",
	}, ";")

	h.Set("Content-Security-Policy", cspValue)
}

// SetAppLaunchContentSecurityPolicy sets the Content-Security-Policy header for /web/launch
func SetAppLaunchContentSecurityPolicy(h http.Header, applicationURL string) {
	cspValue := strings.Join([]string{
		GetDefaultContentSecurityPolicy(),
		// 'unsafe-inline' is required by CSS-in-JS to work
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"font-src 'self' data:",
		fmt.Sprintf("connect-src 'self' %s", applicationURL),
	}, ";")

	h.Set("Content-Security-Policy", cspValue)
}

// GetDefaultContentSecurityPolicy provides a starting Content Security Policy with safe defaults.
func GetDefaultContentSecurityPolicy() string {
	return strings.Join([]string{
		"default-src 'self'",
		// specify CSP directives not covered by `default-src`
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
		// additional default restrictions
		"object-src 'none'",
	}, ";")
}

// SetDefaultContentSecurityPolicy provides a starting Content Security Policy with safe defaults.
func SetDefaultContentSecurityPolicy(h http.Header) {
	h.Set("Content-Security-Policy", GetDefaultContentSecurityPolicy())
}

// SetWebConfigHeaders sets headers for webConfig.js
func SetWebConfigHeaders(h http.Header) {
	h.Set("Content-Type", "application/javascript")
}

// SetScriptHeaders sets headers for the teleport install script
func SetScriptHeaders(h http.Header) {
	h.Set("Content-Type", "text/x-shellscript")
}
