/*
Copyright 2023 Gravitational, Inc.

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

package types

import (
	"net/url"
	"strings"

	"github.com/gravitational/trace"
)

// Matcher is an interface for cloud resource matchers.
type Matcher interface {
	// GetTypes gets the types that the matcher can match.
	GetTypes() []string
	// CopyWithTypes copies the matcher with new types.
	CopyWithTypes(t []string) Matcher
}

// CheckAndSetDefaults checks and sets defaults for HTTPProxySettings.
func (settings *HTTPProxySettings) CheckAndSetDefaults() error {
	if settings == nil {
		return nil
	}

	if !isValidHTTPProxyURL(settings.HTTPProxy) {
		return trace.BadParameter("invalid http_proxy setting: %q", settings.HTTPProxy)
	}
	if !isValidHTTPProxyURL(settings.HTTPSProxy) {
		return trace.BadParameter("invalid https_proxy setting: %q", settings.HTTPSProxy)
	}

	// NO_PROXY can contain multiple comma-separated values.
	// Each value can have multiple formats: IP address, CIDR, domain name, etc.
	// Each tool might have its own rules for parsing and validating NO_PROXY values.
	// Due to this complexity and ambiguity, we skip strict validation here.

	return nil
}

// We expect these variables to be used by Go code, so this method must allow at least all possible variations that are allowed by the golang.org/x/net/http/httpproxy.
func isValidHTTPProxyURL(proxyURL string) bool {
	if proxyURL == "" {
		return true
	}

	if !strings.HasPrefix("https://", proxyURL) && !strings.HasPrefix("http://", proxyURL) {
		// See https://cs.opensource.google/go/x/net/+/refs/tags/v0.46.0:http/httpproxy/proxy.go;drc=cde1dda944dcf6350753df966bb5bda87a544842;l=154
		proxyURL = "http://" + proxyURL
	}
	if _, err := url.Parse(proxyURL); err != nil {
		return false
	}

	return true
}
